//go:build !android

package main

import (
	"fmt"
	"os/exec"
	"runtime"
)

func startPlatformBackground(reason string, message string) {}
func stopPlatformBackground(reason string)                  {}

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
