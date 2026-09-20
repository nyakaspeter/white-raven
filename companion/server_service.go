package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/autobrr/harbrr/pkg/embedded"
	"github.com/nyakaspeter/white-raven/server/runtime"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type ServerService struct {
	controller runtime.Controller
	logs       *runtime.LogBuffer
	configPath string
	operation  sync.Mutex
	installer  *widgetInstaller
	harbrr     *embedded.Server
	harbrrMu   sync.Mutex
	harbrrLogs *runtime.LogBuffer
	harbrrData string
	harbrrPort int
}

func NewServerService() (*ServerService, error) {
	configRoot := application.Mobile.StoragePath()
	if configRoot == "" {
		var err error
		configRoot, err = os.UserConfigDir()
		if err != nil {
			return nil, err
		}
	}
	logs := runtime.NewLogBuffer(1000)
	harbrrLogs := runtime.NewLogBuffer(1000)
	log.SetFlags(0)
	log.SetOutput(io.MultiWriter(os.Stderr, logs))
	return &ServerService{
		logs:       logs,
		configPath: filepath.Join(configRoot, "White Raven", "server.json"),
		installer:  newWidgetInstaller(configRoot),
		harbrrLogs: harbrrLogs,
		harbrrData: filepath.Join(configRoot, "White Raven", "harbrr"),
		harbrrPort: 7478,
	}, nil
}

func (service *ServerService) DefaultConfig() runtime.Config {
	return normalized(runtime.DefaultConfig())
}

func (service *ServerService) LoadConfig() runtime.Config {
	config := service.DefaultConfig()
	data, err := os.ReadFile(service.configPath)
	if os.IsNotExist(err) {
		return normalized(config)
	}
	if err != nil {
		log.Println("Unable to read settings:", err)
		return normalized(config)
	}
	if err := json.Unmarshal(data, &config); err != nil {
		log.Println("Unable to parse settings:", err)
	}
	return normalized(config)
}

func (service *ServerService) SaveConfig(config runtime.Config) error {
	config = normalized(config)
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(service.configPath), 0700); err != nil {
		return err
	}
	return os.WriteFile(service.configPath, data, 0600)
}

func (service *ServerService) StartServer(config runtime.Config) error {
	service.operation.Lock()
	defer service.operation.Unlock()
	config = normalized(config)
	if err := service.SaveConfig(config); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	service.logs.Clear()
	log.Println("Starting White Raven Server.")
	if err := service.controller.Start(config); err != nil {
		log.Println("Start failed:", err)
		return err
	}
	startPlatformBackground("server", "Server is running")
	go func() {
		service.controller.Wait()
		stopPlatformBackground("server")
	}()
	return nil
}

func (service *ServerService) StopServer() {
	service.operation.Lock()
	defer service.operation.Unlock()
	service.controller.Stop()
	stopPlatformBackground("server")
}

func (service *ServerService) Status() runtime.Status { return service.controller.Status() }
func (service *ServerService) Logs() string           { return service.logs.String() }
func (service *ServerService) ClearLogs()             { service.logs.Clear() }
func (service *ServerService) StartHarbrr() error {
	service.operation.Lock()
	defer service.operation.Unlock()

	service.harbrrMu.Lock()
	if service.harbrr != nil {
		service.harbrrMu.Unlock()
		return fmt.Errorf("Harbrr is already running")
	}
	service.harbrrMu.Unlock()

	service.harbrrLogs.Clear()
	instance, err := embedded.Start(context.Background(), embedded.Options{
		Host: "", Port: service.harbrrPort, DataDir: service.harbrrData,
		Log: io.MultiWriter(os.Stderr, service.harbrrLogs),
	})
	if err != nil {
		return err
	}
	service.harbrrMu.Lock()
	service.harbrr = instance
	service.harbrrMu.Unlock()
	startPlatformBackground("harbrr", "Harbrr is running")
	go service.waitForHarbrr(instance)
	return nil
}

func (service *ServerService) waitForHarbrr(instance *embedded.Server) {
	err := instance.Wait()
	service.harbrrMu.Lock()
	if service.harbrr == instance {
		service.harbrr = nil
	}
	service.harbrrMu.Unlock()
	if !embedded.IsStopped(err) {
		_, _ = fmt.Fprintln(service.harbrrLogs, "Harbrr stopped:", err)
	}
	stopPlatformBackground("harbrr")
}

func (service *ServerService) StopHarbrr() {
	service.operation.Lock()
	defer service.operation.Unlock()
	service.harbrrMu.Lock()
	instance := service.harbrr
	service.harbrrMu.Unlock()
	if instance != nil {
		_ = instance.Stop()
		service.harbrrMu.Lock()
		if service.harbrr == instance {
			service.harbrr = nil
		}
		service.harbrrMu.Unlock()
	}
	stopPlatformBackground("harbrr")
}

func (service *ServerService) HarbrrStatus() runtime.Status {
	service.harbrrMu.Lock()
	running := service.harbrr != nil
	service.harbrrMu.Unlock()
	if !running {
		return runtime.Status{}
	}
	return runtime.Status{Running: true, Address: localAddress(service.harbrrPort)}
}

func (service *ServerService) HarbrrLogs() string         { return service.harbrrLogs.String() }
func (service *ServerService) ClearHarbrrLogs()           { service.harbrrLogs.Clear() }
func (service *ServerService) WidgetStatus() WidgetStatus { return service.installer.Status() }
func (service *ServerService) InstallRooted(request RootedInstallRequest) error {
	request.Config = normalized(request.Config)
	if err := service.SaveConfig(request.Config); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	return service.installer.InstallRooted(request)
}
func (service *ServerService) StartAppSync() error { return service.installer.StartAppSync() }
func (service *ServerService) StopAppSync()        { service.installer.StopAppSync() }
func (service *ServerService) Shutdown() {
	service.installer.StopAppSync()
	service.StopHarbrr()
	service.StopServer()
}

func localAddress(port int) string {
	host := "127.0.0.1"
	connection, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		host = connection.LocalAddr().(*net.UDPAddr).IP.String()
		_ = connection.Close()
	}
	return fmt.Sprintf("http://%s:%d", host, port)
}

func normalized(config runtime.Config) runtime.Config {
	config.Host = ""
	config.Port = 9000
	config.DlnaPort = 3500
	config.StorageType = "memory"
	config.DownloadDir = "data"
	config.EnableLog = true
	config.EnableReceiver = true
	config.Background = false
	config.CORS = true
	return config
}
