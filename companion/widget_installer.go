package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/nyakaspeter/white-raven/server/runtime"
	"golang.org/x/crypto/ssh"
)

const (
	releaseAPIURL  = "https://api.github.com/repos/nyakaspeter/white-raven/releases/latest"
	widgetRoot     = "/mtd_rwcommon/widgets/user/WhiteRaven"
	appSyncAddress = ":80"
	appSyncWidget  = "WhiteRaven.zip"
)

type RootedInstallRequest struct {
	Host          string         `json:"host"`
	Port          int            `json:"port"`
	Username      string         `json:"username"`
	Password      string         `json:"password"`
	InstallServer bool           `json:"installServer"`
	Reboot        bool           `json:"reboot"`
	Config        runtime.Config `json:"config"`
}

type WidgetStatus struct {
	Busy        bool   `json:"busy"`
	SyncRunning bool   `json:"syncRunning"`
	Mode        string `json:"mode"`
	Message     string `json:"message"`
	Version     string `json:"version"`
	Address     string `json:"address"`
}

type releaseAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
	Size        int64  `json:"size"`
}

type githubRelease struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

type widgetInstaller struct {
	mu         sync.Mutex
	status     WidgetStatus
	syncServer *http.Server
	tempDir    string
	tempRoot   string
	httpClient *http.Client
}

func newWidgetInstaller(tempRoot string) *widgetInstaller {
	return &widgetInstaller{
		tempRoot:   filepath.Join(tempRoot, "White Raven", "app-sync"),
		httpClient: &http.Client{Timeout: 10 * time.Minute},
	}
}

func (installer *widgetInstaller) Status() WidgetStatus {
	installer.mu.Lock()
	defer installer.mu.Unlock()
	return installer.status
}

func (installer *widgetInstaller) setStatus(update func(*WidgetStatus)) {
	installer.mu.Lock()
	defer installer.mu.Unlock()
	update(&installer.status)
}

func (installer *widgetInstaller) InstallRooted(request RootedInstallRequest) (err error) {
	installer.mu.Lock()
	if installer.status.Busy {
		installer.mu.Unlock()
		return errors.New("another widget operation is already in progress")
	}
	installer.status.Busy = true
	installer.status.Mode = "rooted"
	variant := "rootless"
	if request.InstallServer {
		variant = "rooted"
	}
	installer.status.Message = "Finding the latest " + variant + " release…"
	installer.mu.Unlock()
	defer func() {
		installer.setStatus(func(status *WidgetStatus) {
			status.Busy = false
			if err != nil {
				status.Message = "Install failed: " + err.Error()
			}
		})
	}()

	request.Host = strings.TrimSpace(request.Host)
	request.Username = strings.TrimSpace(request.Username)
	if request.Host == "" {
		return errors.New("TV address is required")
	}
	if request.Port == 0 {
		request.Port = 22
	}
	if request.Port < 1 || request.Port > 65535 {
		return errors.New("SSH port must be between 1 and 65535")
	}
	if request.Username == "" {
		request.Username = "root"
	}

	release, asset, err := installer.latestAsset(!request.InstallServer)
	if err != nil {
		return err
	}
	installer.setStatus(func(status *WidgetStatus) {
		status.Version = release.TagName
		status.Message = "Downloading " + asset.Name + "…"
	})
	archive, err := installer.download(asset)
	if err != nil {
		return err
	}

	installer.setStatus(func(status *WidgetStatus) { status.Message = "Connecting to the TV…" })
	sshClient, err := dialTV(request)
	if err != nil {
		return err
	}
	defer sshClient.Close()
	installer.setStatus(func(status *WidgetStatus) { status.Message = "Installing files on the TV… (0%)" })
	if err := uploadArchive(sshClient, archive, widgetRoot, func(written int64, total int64) {
		percent := 0
		if total > 0 {
			percent = int(written * 100 / total)
		}
		installer.setStatus(func(status *WidgetStatus) {
			status.Message = fmt.Sprintf("Installing files on the TV… (%d%%)", percent)
		})
	}); err != nil {
		return err
	}
	installer.setStatus(func(status *WidgetStatus) { status.Message = "Finalizing installation…" })
	finalizeCommand := "chown -R app:app " + shellQuote(widgetRoot)
	if request.InstallServer {
		serverInit, renderErr := renderServerInit(archive, request.Config)
		if renderErr != nil {
			return renderErr
		}
		if err := uploadBytes(sshClient, path.Join(widgetRoot, "server/server.init"), []byte(serverInit), 0755, nil); err != nil {
			return fmt.Errorf("update server.init: %w", err)
		}
		finalizeCommand += " && chmod 755 " + shellQuote(path.Join(widgetRoot, "server/server.init")) +
			" " + shellQuote(path.Join(widgetRoot, "server/wrserver"))
	} else {
		finalizeCommand = "rm -rf " + shellQuote(path.Join(widgetRoot, "server")) + " && " + finalizeCommand
	}
	if err := runSSHCommand(sshClient, finalizeCommand); err != nil {
		return fmt.Errorf("set widget ownership and permissions: %w", err)
	}

	message := "Installed " + release.TagName + ". Restart the TV to finish."
	if request.Reboot {
		installer.setStatus(func(status *WidgetStatus) { status.Message = "Rebooting the TV…" })
		session, sessionErr := sshClient.NewSession()
		if sessionErr != nil {
			return fmt.Errorf("start reboot session: %w", sessionErr)
		}
		rebootErr := session.Run("sync; reboot")
		_ = session.Close()
		// Many TVs close SSH immediately once reboot starts. Treat that as success.
		if rebootErr != nil && !isDisconnectError(rebootErr) {
			return fmt.Errorf("reboot TV: %w", rebootErr)
		}
		message = "Installed " + release.TagName + " and sent the reboot command."
	}
	installer.setStatus(func(status *WidgetStatus) { status.Message = message })
	return nil
}

func (installer *widgetInstaller) StartAppSync() (err error) {
	installer.mu.Lock()
	if installer.status.Busy {
		installer.mu.Unlock()
		return errors.New("another widget operation is already in progress")
	}
	if installer.syncServer != nil {
		installer.mu.Unlock()
		return nil
	}
	installer.status.Busy = true
	installer.status.Mode = "unrooted"
	installer.status.Message = "Finding the latest unrooted release…"
	installer.mu.Unlock()
	defer func() {
		installer.setStatus(func(status *WidgetStatus) {
			status.Busy = false
			if err != nil {
				status.Message = "App Sync failed: " + err.Error()
			}
		})
	}()

	release, asset, err := installer.latestAsset(true)
	if err != nil {
		return err
	}
	installer.setStatus(func(status *WidgetStatus) { status.Message = "Downloading " + asset.Name + "…" })
	archive, err := installer.download(asset)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(installer.tempRoot, 0700); err != nil {
		return fmt.Errorf("create private App Sync storage: %w", err)
	}
	tempDir, err := os.MkdirTemp(installer.tempRoot, "session-")
	if err != nil {
		return fmt.Errorf("create App Sync directory: %w", err)
	}
	zipPath := filepath.Join(tempDir, asset.Name)
	if err := os.WriteFile(zipPath, archive, 0600); err != nil {
		_ = os.RemoveAll(tempDir)
		return fmt.Errorf("store widget release: %w", err)
	}

	listener, err := net.Listen("tcp", appSyncAddress)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return fmt.Errorf("listen on port 80 (required by Samsung App Sync): %w", err)
	}
	ip := localNetworkIP()
	address := "http://" + ip
	mux := http.NewServeMux()
	mux.HandleFunc("/widgetlist.xml", func(writer http.ResponseWriter, request *http.Request) {
		installer.setStatus(func(status *WidgetStatus) {
			status.Message = "TV connected. Waiting for the White Raven download…"
		})
		writeWidgetList(writer, int64(len(archive)), address)
	})
	mux.HandleFunc("/"+appSyncWidget, func(writer http.ResponseWriter, request *http.Request) {
		installer.serveWidgetArchive(writer, request, archive)
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	installer.mu.Lock()
	installer.syncServer = server
	installer.tempDir = tempDir
	installer.status.SyncRunning = true
	installer.status.Version = release.TagName
	installer.status.Address = address
	installer.status.Message = "App Sync is ready. Waiting for the TV…"
	installer.mu.Unlock()
	startPlatformBackground("app-sync", "App Sync is waiting for the TV")
	go func() {
		serveErr := server.Serve(listener)
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			installer.mu.Lock()
			if installer.syncServer == server {
				installer.syncServer = nil
				installer.tempDir = ""
				installer.status.Address = ""
				installer.status.Message = "App Sync server stopped: " + serveErr.Error()
				installer.status.SyncRunning = false
			}
			installer.mu.Unlock()
			stopPlatformBackground("app-sync")
			_ = os.RemoveAll(tempDir)
		}
	}()
	return nil
}

func (installer *widgetInstaller) StopAppSync() {
	installer.mu.Lock()
	server := installer.syncServer
	tempDir := installer.tempDir
	installer.syncServer = nil
	installer.tempDir = ""
	installer.status.SyncRunning = false
	installer.status.Address = ""
	installer.status.Mode = "unrooted"
	installer.status.Message = "App Sync server stopped."
	installer.mu.Unlock()
	if server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = server.Shutdown(ctx)
		cancel()
	}
	stopPlatformBackground("app-sync")
	if tempDir != "" {
		_ = os.RemoveAll(tempDir)
	}
}

func (installer *widgetInstaller) latestAsset(rootless bool) (githubRelease, releaseAsset, error) {
	request, err := http.NewRequest(http.MethodGet, releaseAPIURL, nil)
	if err != nil {
		return githubRelease{}, releaseAsset{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "White-Raven-Companion")
	response, err := installer.httpClient.Do(request)
	if err != nil {
		return githubRelease{}, releaseAsset{}, fmt.Errorf("fetch latest GitHub release: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return githubRelease{}, releaseAsset{}, fmt.Errorf("fetch latest GitHub release: %s", response.Status)
	}
	var release githubRelease
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&release); err != nil {
		return githubRelease{}, releaseAsset{}, fmt.Errorf("read latest GitHub release: %w", err)
	}
	asset, err := selectReleaseAsset(release.Assets, rootless)
	return release, asset, err
}

func selectReleaseAsset(assets []releaseAsset, rootless bool) (releaseAsset, error) {
	for _, asset := range assets {
		name := strings.ToLower(asset.Name)
		if !strings.HasPrefix(name, "whiteraven-") || !strings.HasSuffix(name, ".zip") {
			continue
		}
		isRootless := strings.Contains(name, "-rootless-")
		if isRootless == rootless {
			return asset, nil
		}
	}
	variant := "rooted"
	if rootless {
		variant = "unrooted"
	}
	return releaseAsset{}, fmt.Errorf("latest GitHub release has no %s widget ZIP", variant)
}

func (installer *widgetInstaller) download(asset releaseAsset) ([]byte, error) {
	response, err := installer.httpClient.Get(asset.DownloadURL)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", asset.Name, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", asset.Name, response.Status)
	}
	const maxWidgetSize = 100 << 20
	data, err := io.ReadAll(io.LimitReader(response.Body, maxWidgetSize+1))
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", asset.Name, err)
	}
	if len(data) > maxWidgetSize {
		return nil, errors.New("widget release is larger than 100 MB")
	}
	return data, nil
}

func dialTV(request RootedInstallRequest) (*ssh.Client, error) {
	address := net.JoinHostPort(request.Host, strconv.Itoa(request.Port))
	log.Printf("Connecting to TV over SSH at %s", address)
	config := &ssh.ClientConfig{
		User:            request.Username,
		Auth:            []ssh.AuthMethod{ssh.Password(request.Password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Legacy TVs do not expose stable host-key management.
		Timeout:         20 * time.Second,
		HostKeyAlgorithms: []string{
			ssh.KeyAlgoRSA, ssh.KeyAlgoECDSA256, ssh.KeyAlgoECDSA384, ssh.KeyAlgoECDSA521, ssh.KeyAlgoED25519,
		},
		Config: ssh.Config{
			KeyExchanges: []string{"diffie-hellman-group14-sha256", "diffie-hellman-group14-sha1", "diffie-hellman-group1-sha1"},
			Ciphers:      []string{"aes128-ctr", "aes192-ctr", "aes256-ctr", "aes128-cbc", "3des-cbc"},
		},
	}
	client, err := ssh.Dial("tcp", address, config)
	if err != nil {
		log.Printf("SSH connection to %s failed: %v", address, err)
		if errors.Is(err, syscall.EHOSTUNREACH) {
			return nil, fmt.Errorf(
				"connect to TV over SSH: macOS blocked local-network access to %s (no route to host); allow White Raven Companion in System Settings > Privacy & Security > Local Network, then fully quit and reopen the app",
				address,
			)
		}
		return nil, fmt.Errorf("connect to TV over SSH: %w", err)
	}
	log.Printf("SSH connection to %s established", address)
	return client, nil
}

func uploadArchive(client *ssh.Client, data []byte, destination string, progress func(int64, int64)) error {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("open rooted widget ZIP: %w", err)
	}
	for _, file := range reader.File {
		cleanName := path.Clean(strings.ReplaceAll(file.Name, "\\", "/"))
		if cleanName == "." || strings.HasPrefix(cleanName, "../") || path.IsAbs(cleanName) {
			return fmt.Errorf("unsafe path in widget ZIP: %q", file.Name)
		}
	}

	archivePath := destination + ".install.zip"
	if err := uploadBytes(client, archivePath, data, 0600, progress); err != nil {
		return fmt.Errorf("upload rooted widget ZIP: %w", err)
	}
	command := fmt.Sprintf(
		"mkdir -p %s && unzip -oq %s -d %s; status=$?; rm -f %s; exit $status",
		shellQuote(destination), shellQuote(archivePath), shellQuote(destination), shellQuote(archivePath),
	)
	if err := runSSHCommand(client, command); err != nil {
		return fmt.Errorf("extract rooted widget ZIP: %w", err)
	}
	return nil
}

func uploadBytes(client *ssh.Client, remotePath string, data []byte, mode os.FileMode, progress func(int64, int64)) error {
	name := path.Base(remotePath)
	if name == "." || name == "/" || strings.ContainsAny(name, "\r\n\x00") {
		return fmt.Errorf("invalid remote filename %q", name)
	}
	directory := path.Dir(remotePath)
	if err := runSSHCommand(client, "mkdir -p "+shellQuote(directory)); err != nil {
		return fmt.Errorf("create remote directory: %w", err)
	}

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("start SCP session: %w", err)
	}
	defer session.Close()
	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("open SCP input: %w", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("open SCP output: %w", err)
	}
	var stderr bytes.Buffer
	session.Stderr = &stderr
	if err := session.Start("scp -t " + shellQuote(directory)); err != nil {
		return fmt.Errorf("start remote SCP receiver: %w", err)
	}
	ack := bufio.NewReader(stdout)
	if err := readSCPAck(ack); err != nil {
		return fmt.Errorf("initialize SCP transfer: %w", err)
	}
	if _, err := fmt.Fprintf(stdin, "C%04o %d %s\n", mode.Perm(), len(data), name); err != nil {
		return fmt.Errorf("send SCP file header: %w", err)
	}
	if err := readSCPAck(ack); err != nil {
		return fmt.Errorf("accept SCP file header: %w", err)
	}
	if progress != nil {
		progress(0, int64(len(data)))
	}
	const chunkSize = 64 * 1024
	written := 0
	for written < len(data) {
		end := written + chunkSize
		if end > len(data) {
			end = len(data)
		}
		n, err := stdin.Write(data[written:end])
		if n > 0 {
			written += n
			if progress != nil {
				progress(int64(written), int64(len(data)))
			}
		}
		if err != nil {
			return fmt.Errorf("send SCP file: %w", err)
		}
		if n == 0 {
			return fmt.Errorf("send SCP file: %w", io.ErrShortWrite)
		}
	}
	if _, err := stdin.Write([]byte{0}); err != nil {
		return fmt.Errorf("finish SCP file: %w", err)
	}
	if err := readSCPAck(ack); err != nil {
		return fmt.Errorf("complete SCP transfer: %w", err)
	}
	if err := stdin.Close(); err != nil {
		return fmt.Errorf("close SCP input: %w", err)
	}
	if err := session.Wait(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message != "" {
			return fmt.Errorf("remote SCP receiver: %s: %w", message, err)
		}
		return fmt.Errorf("remote SCP receiver: %w", err)
	}
	return nil
}

func readSCPAck(reader *bufio.Reader) error {
	code, err := reader.ReadByte()
	if err != nil {
		return err
	}
	if code == 0 {
		return nil
	}
	message, readErr := reader.ReadString('\n')
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return readErr
	}
	message = strings.TrimSpace(message)
	if message == "" {
		message = "unknown remote error"
	}
	if code == 1 || code == 2 {
		return errors.New(message)
	}
	return fmt.Errorf("unexpected SCP response %d: %s", code, message)
}

func runSSHCommand(client *ssh.Client, command string) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	output, err := session.CombinedOutput(command)
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message != "" {
			return fmt.Errorf("%s: %w", message, err)
		}
		return err
	}
	return nil
}

func writeWidgetList(writer http.ResponseWriter, size int64, address string) {
	// Keep this deliberately simple and byte-compatible with White Raven's
	// original sync server. Some legacy Samsung parsers reject otherwise-valid
	// XML variants such as a paired, empty compression element.
	body := fmt.Sprintf(
		"<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"+
			"<rsp stat=\"ok\"><list><widget id=\"WhiteRaven\">"+
			"<title>White Raven</title>"+
			"<compression size=\"%d\" type=\"zip\"/>"+
			"<description>Watch Movies And TV Shows From Torrents Instantly!</description>"+
			"<download>%s/%s</download>"+
			"</widget></list></rsp>",
		size, address, appSyncWidget,
	)
	writer.Header().Set("Content-Type", "text/xml; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Content-Length", strconv.Itoa(len(body)))
	_, _ = io.WriteString(writer, body)
}

func (installer *widgetInstaller) serveWidgetArchive(writer http.ResponseWriter, request *http.Request, archive []byte) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		writer.Header().Set("Allow", "GET, HEAD")
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writer.Header().Set("Content-Type", "application/zip")
	writer.Header().Set("Content-Length", strconv.Itoa(len(archive)))
	writer.Header().Set("Cache-Control", "no-store")
	if request.Method == http.MethodHead {
		return
	}

	installer.setStatus(func(status *WidgetStatus) {
		status.Message = "TV is downloading White Raven… (0%)"
	})
	const chunkSize = 32 * 1024
	written := 0
	lastPercent := -1
	for written < len(archive) {
		end := written + chunkSize
		if end > len(archive) {
			end = len(archive)
		}
		n, err := writer.Write(archive[written:end])
		written += n
		percent := written * 100 / len(archive)
		if percent != lastPercent {
			lastPercent = percent
			installer.setStatus(func(status *WidgetStatus) {
				status.Message = fmt.Sprintf("TV is downloading White Raven… (%d%%)", percent)
			})
		}
		if err != nil {
			installer.setStatus(func(status *WidgetStatus) {
				status.Message = "TV download failed: " + err.Error()
			})
			return
		}
		if n == 0 {
			installer.setStatus(func(status *WidgetStatus) {
				status.Message = "TV download failed: no data was transferred"
			})
			return
		}
	}
	installer.setStatus(func(status *WidgetStatus) {
		status.Message = "White Raven downloaded (100%). The TV is installing the widget…"
	})
}

func renderServerInit(archive []byte, config runtime.Config) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return "", fmt.Errorf("open rooted widget ZIP for server.init: %w", err)
	}

	var script string
	for _, file := range reader.File {
		name := strings.TrimPrefix(path.Clean(strings.ReplaceAll(file.Name, "\\", "/")), "./")
		if name != "server/server.init" && !strings.HasSuffix(name, "/server/server.init") {
			continue
		}
		if file.UncompressedSize64 > 1<<20 {
			return "", errors.New("server.init in rooted widget ZIP is larger than 1 MB")
		}
		content, openErr := file.Open()
		if openErr != nil {
			return "", fmt.Errorf("open server.init in rooted widget ZIP: %w", openErr)
		}
		data, readErr := io.ReadAll(io.LimitReader(content, (1<<20)+1))
		closeErr := content.Close()
		if readErr != nil {
			return "", fmt.Errorf("read server.init in rooted widget ZIP: %w", readErr)
		}
		if closeErr != nil {
			return "", fmt.Errorf("close server.init in rooted widget ZIP: %w", closeErr)
		}
		if len(data) > 1<<20 {
			return "", errors.New("server.init in rooted widget ZIP is larger than 1 MB")
		}
		script = string(data)
		break
	}
	if script == "" {
		return "", errors.New("rooted widget ZIP does not contain server/server.init")
	}

	settings := []struct {
		name  string
		value string
	}{
		{"TMDBKEY", shellQuote(config.TMDBKey)},
		{"OPENSUBTITLESKEY", shellQuote(config.OpenSubtitlesKey)},
		{"OPENSUBTITLESUSER", shellQuote(config.OpenSubtitlesUser)},
		{"OPENSUBTITLESPASSWORD", shellQuote(config.OpenSubtitlesPassword)},
		{"JACKETTADDRESS", shellQuote(config.JackettAddress)},
		{"JACKETTKEY", shellQuote(config.JackettKey)},
		{"NCOREUSER", shellQuote(config.NcoreUser)},
		{"NCOREPASSWORD", shellQuote(config.NcorePassword)},
		{"INSANEUSER", shellQuote(config.InsaneUser)},
		{"INSANEPASSWORD", shellQuote(config.InsanePassword)},
		{"MEMORYSIZE", shellQuote(strconv.FormatInt(config.MemorySize, 10))},
		{"DOWNSPEED", fmt.Sprintf(`"${2:-%d}"`, config.DownloadRate)},
		{"UPSPEED", fmt.Sprintf(`"${3:-%d}"`, config.UploadRate)},
		{"MAXCONNECTIONS", shellQuote(strconv.Itoa(config.MaxConnections))},
		{"NODHT", shellQuote(strconv.FormatBool(config.NoDHT))},
		{"NOIPV6", shellQuote(strconv.FormatBool(config.DisableIPv6))},
		{"NOUTP", shellQuote(strconv.FormatBool(config.DisableUTP))},
	}
	for _, setting := range settings {
		script, err = replaceShellAssignment(script, setting.name, setting.value)
		if err != nil {
			return "", err
		}
	}
	return script, nil
}

func replaceShellAssignment(script string, name string, value string) (string, error) {
	prefix := name + "="
	start := shellAssignmentStart(script, name)
	if start < 0 {
		return "", fmt.Errorf("server.init is missing %s setting", name)
	}
	end := strings.IndexByte(script[start:], '\n')
	if end < 0 {
		end = len(script)
	} else {
		end += start
	}
	return script[:start] + prefix + value + script[end:], nil
}

func shellAssignmentStart(script string, name string) int {
	prefix := name + "="
	if strings.HasPrefix(script, prefix) {
		return 0
	}
	position := strings.Index(script, "\n"+prefix)
	if position < 0 {
		return -1
	}
	return position + 1
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }

func localNetworkIP() string {
	connection, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer connection.Close()
		if address, ok := connection.LocalAddr().(*net.UDPAddr); ok {
			return address.IP.String()
		}
	}
	interfaces, _ := net.InterfaceAddrs()
	for _, address := range interfaces {
		if network, ok := address.(*net.IPNet); ok && network.IP.To4() != nil && !network.IP.IsLoopback() {
			return network.IP.String()
		}
	}
	return "127.0.0.1"
}

func isDisconnectError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "connection reset") || strings.Contains(message, "broken pipe") || strings.Contains(message, "eof")
}
