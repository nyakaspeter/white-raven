//go:build android

package main

import (
	"encoding/json"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var platformBackground = struct {
	sync.Mutex
	reasons map[string]string
}{reasons: make(map[string]string)}

func startPlatformBackground(reason string, message string) {
	platformBackground.Lock()
	defer platformBackground.Unlock()
	platformBackground.reasons[reason] = message
	startPlatformForegroundService(message)
}

func stopPlatformBackground(reason string) {
	platformBackground.Lock()
	defer platformBackground.Unlock()
	delete(platformBackground.reasons, reason)
	if message, ok := platformBackground.reasons["app-sync"]; ok {
		startPlatformForegroundService(message)
		return
	}
	if message, ok := platformBackground.reasons["server"]; ok {
		startPlatformForegroundService(message)
		return
	}
	if message, ok := platformBackground.reasons["harbrr"]; ok {
		startPlatformForegroundService(message)
		return
	}
	application.Android.StopForegroundService()
}

func startPlatformForegroundService(message string) {
	payload, _ := json.Marshal(map[string]string{
		"title": "White Raven Companion",
		"text":  message,
	})
	application.Android.StartForegroundService(string(payload))
}
