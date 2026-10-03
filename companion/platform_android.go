//go:build android

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
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

func platformOpenURL(url string) error {
	application.Android.OpenURL(url)
	return nil
}

// startUpdateDownload downloads the release APK into app storage and hands
// it to the system package installer, streaming progress events to the
// frontend's update dialog.
func startUpdateDownload(service *ServerService, release *updater.Release) (string, error) {
	storage := application.Mobile.StoragePath()
	if storage == "" {
		return "", errors.New("app storage is not available for the update download")
	}
	url := releaseDownloadURL(release)
	dir := filepath.Join(storage, "updates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// The directory only ever holds update APKs, so drop any older
	// downloads: the user keeps just the newest installer.
	if entries, err := os.ReadDir(dir); err == nil {
		for _, entry := range entries {
			if entry.Type().IsRegular() {
				_ = os.Remove(filepath.Join(dir, entry.Name()))
			}
		}
	}
	target := filepath.Join(dir, release.Artifact.Filename)
	part := target + ".part"

	response, err := (&http.Client{Timeout: 30 * time.Minute}).Get(url)
	if err != nil {
		return "", fmt.Errorf("start APK download: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("APK download failed with status %s", response.Status)
	}

	out, err := os.Create(part)
	if err != nil {
		return "", err
	}
	reporter := newAPKProgressReporter(service, response.ContentLength)
	_, err = io.Copy(io.MultiWriter(out, reporter), response.Body)
	_ = out.Close()
	if err != nil {
		_ = os.Remove(part)
		return "", fmt.Errorf("download APK: %w", err)
	}
	if err := os.Rename(part, target); err != nil {
		return "", err
	}
	reporter.final()

	// Hand the APK to the system package installer through the FileProvider.
	// Plain file:// URIs into the app's private storage are blocked for other
	// apps on Android 7+, so the installer never launches; the content:// URI
	// below (see file_paths.xml) shares the file with a read-permission grant.
	rel, err := filepath.Rel(storage, target)
	if err != nil {
		return "", err
	}
	application.Android.OpenURL("content://com.whiteraven.server.fileprovider/" + rel)
	return "Downloaded the APK and opened the system installer.", nil
}

// apkProgressReporter emits wails:updater:download-progress events at most
// once per second while an APK is being downloaded.
type apkProgressReporter struct {
	service *ServerService
	total   int64
	written int64
	lastAt  time.Time
	lastN   int64
}

func newAPKProgressReporter(service *ServerService, total int64) *apkProgressReporter {
	r := &apkProgressReporter{service: service, total: total, lastAt: time.Now()}
	r.emit(0)
	return r
}

func (r *apkProgressReporter) Write(p []byte) (int, error) {
	r.written += int64(len(p))
	now := time.Now()
	if now.Sub(r.lastAt) < time.Second {
		return len(p), nil
	}
	rate := float64(r.written-r.lastN) / now.Sub(r.lastAt).Seconds()
	r.lastAt, r.lastN = now, r.written
	r.emit(rate)
	return len(p), nil
}

func (r *apkProgressReporter) final() { r.emit(0) }

func (r *apkProgressReporter) emit(rate float64) {
	if r.service == nil || r.service.app == nil {
		return
	}
	r.service.app.Event.Emit(updater.EventDownloadProgress, updater.Progress{
		Written:  r.written,
		Total:    r.total,
		Rate:     rate,
		Provider: "github",
	})
}
