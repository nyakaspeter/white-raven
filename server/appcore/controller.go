package appcore

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/nyakaspeter/white-raven/server/internal/httpserver"
	"github.com/nyakaspeter/white-raven/server/internal/settings"
	"github.com/nyakaspeter/white-raven/server/internal/torrentclient"
)

type Config = settings.Config

func DefaultConfig() Config { return settings.DefaultConfig() }

type Controller struct {
	mu      sync.Mutex
	running bool
	quit    chan os.Signal
	done    chan struct{}
}

type Status struct {
	Running bool   `json:"running"`
	Address string `json:"address"`
}

func (controller *Controller) Status() Status {
	controller.mu.Lock()
	defer controller.mu.Unlock()
	if !controller.running {
		return Status{}
	}
	return Status{Running: true, Address: fmt.Sprintf("http://%s:%d", localIP(), *settings.Port)}
}

func localIP() string {
	connection, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer connection.Close()
	return connection.LocalAddr().(*net.UDPAddr).IP.String()
}

func (controller *Controller) Start(config Config) error {
	if err := validate(config); err != nil {
		return err
	}

	controller.mu.Lock()
	defer controller.mu.Unlock()
	if controller.running {
		return errors.New("server is already running")
	}

	settings.Apply(config)
	quit := make(chan os.Signal, 1)
	if _, err := torrentclient.StartTorrentClient(); err != nil {
		return fmt.Errorf("start torrent client: %w", err)
	}
	if _, err := httpserver.StartHttpServer(quit); err != nil {
		torrentclient.StopTorrentClient()
		return fmt.Errorf("start HTTP server: %w", err)
	}

	controller.running = true
	controller.quit = quit
	controller.done = make(chan struct{})
	go controller.wait(quit, controller.done)
	return nil
}

func (controller *Controller) wait(quit chan os.Signal, done chan struct{}) {
	<-quit
	controller.mu.Lock()
	if controller.quit == quit {
		log.Println("Stopping White Raven Server.")
		httpserver.StopHttpServer()
		torrentclient.StopTorrentClient()
		controller.running = false
		controller.quit = nil
		controller.done = nil
	}
	controller.mu.Unlock()
	close(done)
}

func (controller *Controller) Stop() {
	controller.mu.Lock()
	quit := controller.quit
	done := controller.done
	controller.mu.Unlock()
	if quit == nil {
		return
	}
	select {
	case quit <- os.Interrupt:
	default:
	}
	<-done
}

func (controller *Controller) Wait() {
	controller.mu.Lock()
	done := controller.done
	controller.mu.Unlock()
	if done != nil {
		<-done
	}
}

func (controller *Controller) Running() bool {
	controller.mu.Lock()
	defer controller.mu.Unlock()
	return controller.running
}

func validate(config Config) error {
	if config.Port < 1 || config.Port > 65535 {
		return errors.New("server port must be between 1 and 65535")
	}
	if config.DlnaPort < 1 || config.DlnaPort > 65535 {
		return errors.New("DLNA port must be between 1 and 65535")
	}
	if config.StorageType != "memory" && config.StorageType != "file" {
		return errors.New("storage type must be memory or file")
	}
	if config.StorageType == "memory" && config.MemorySize < 64 {
		return errors.New("memory size must be at least 64 MB")
	}
	if config.StorageType == "file" && strings.TrimSpace(config.DownloadDir) == "" {
		return errors.New("download directory is required for file storage")
	}
	if config.DownloadRate < 0 || config.UploadRate < 0 || config.MaxConnections < 1 {
		return errors.New("rate limits cannot be negative and maximum connections must be positive")
	}
	return nil
}
