package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/nyakaspeter/white-raven/server/runtime"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type ServerService struct {
	controller runtime.Controller
	logs       *runtime.LogBuffer
	configPath string
	operation  sync.Mutex
	installer  *widgetInstaller
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
	log.SetFlags(0)
	log.SetOutput(io.MultiWriter(os.Stderr, logs))
	return &ServerService{
		logs:       logs,
		configPath: filepath.Join(configRoot, "White Raven", "server.json"),
		installer:  newWidgetInstaller(configRoot),
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

func (service *ServerService) Status() runtime.Status     { return service.controller.Status() }
func (service *ServerService) Logs() string               { return service.logs.String() }
func (service *ServerService) ClearLogs()                 { service.logs.Clear() }
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
	service.StopServer()
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
