//go:build !android

package main

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/updater"
)

func startPlatformBackground(reason string, message string) {}
func stopPlatformBackground(reason string)                  {}

// startUpdateDownload opens the release asset's download link in the system
// browser so the user installs the new version through the normal channel.
func startUpdateDownload(service *ServerService, release *updater.Release) (string, error) {
	url := releaseDownloadURL(release)
	if err := platformOpenURL(url); err != nil {
		return "", err
	}
	return "Opened the download link in your browser.", nil
}

func platformOpenURL(url string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", url)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "linux":
		command = exec.Command("xdg-open", url)
	default:
		return fmt.Errorf("opening URLs is not supported on %s", runtime.GOOS)
	}
	if err := command.Start(); err != nil {
		return err
	}
	go command.Wait() //nolint:errcheck
	return nil
}
