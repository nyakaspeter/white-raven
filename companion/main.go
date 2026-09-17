package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	service, err := NewServerService()
	if err != nil {
		log.Fatal(err)
	}

	app := application.New(application.Options{
		Name:        "White Raven Companion",
		Description: "Run and configure White Raven Server",
		Services:    []application.Service{application.NewService(service)},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})
	app.OnShutdown(service.Shutdown)
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "White Raven Companion", Width: 760, Height: 720,
		MinWidth: 360, MinHeight: 560,
		BackgroundColour: application.NewRGB(16, 20, 27), URL: "/",
	})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
