//go:build android

package main

import "github.com/wailsapp/wails/v3/pkg/application"

func startPlatformBackground() {
	application.Android.StartForegroundService(`{"title":"White Raven Companion","text":"Server is running"}`)
}

func stopPlatformBackground() { application.Android.StopForegroundService() }
