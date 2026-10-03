package main

import (
	"regexp"
	"runtime"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

const (
	releaseRepository = "nyakaspeter/white-raven"
	assetNamePrefix   = "whiteraven-companion-"
	devVersion        = "0.0.0-dev"
)

var releaseVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

// updaterSupported reports whether the update flow can run on this build. It
// is disabled for development builds (no published release can match the dev
// sentinel) and for iOS, which publishes no installable asset (App Store
// distribution). Desktop opens a browser download; Android downloads the APK
// and hands it to the system installer.
func updaterSupported() bool {
	if !releaseVersionPattern.MatchString(appVersion) {
		return false
	}
	switch runtime.GOOS {
	case "darwin", "windows", "linux", "android":
		return true
	default:
		return false
	}
}

// companionAssetMatcher selects the release asset for the running platform
// from the names published by the release workflow:
//
//	whiteraven-companion-<version>-windows-x64.exe
//	whiteraven-companion-<version>-macos-universal.zip  (the .app bundle)
//	whiteraven-companion-<version>-linux-x64.AppImage
//	whiteraven-companion-<version>-android.apk
func companionAssetMatcher(req updater.CheckRequest, assets []github.ReleaseAsset) int {
	for i, asset := range assets {
		name := strings.ToLower(asset.Name)
		if !strings.HasPrefix(name, assetNamePrefix) {
			continue
		}
		switch req.Platform {
		case "windows":
			if strings.Contains(name, "-windows-") && strings.HasSuffix(name, ".exe") {
				return i
			}
		case "darwin":
			if strings.Contains(name, "-macos-") && strings.HasSuffix(name, ".zip") {
				return i
			}
		case "linux":
			if strings.Contains(name, "-linux-") && strings.HasSuffix(name, ".appimage") {
				return i
			}
		case "android":
			if strings.Contains(name, "-android") && strings.HasSuffix(name, ".apk") {
				return i
			}
		}
	}
	return -1
}

// setupAutoUpdater configures the framework updater against the GitHub
// releases of this repository. The updater only checks for and announces
// releases (headless, WindowNone); the frontend renders its own popup and
// hands the user to the platform's normal install path — a browser download
// on desktop, the system APK installer on Android.
func setupAutoUpdater(app *application.App, service *ServerService) {
	if !updaterSupported() {
		return
	}
	provider, err := github.New(github.Config{
		Repository:   releaseRepository,
		AssetMatcher: companionAssetMatcher,
	})
	if err != nil {
		return
	}
	if err := app.Updater.Init(updater.Config{
		CurrentVersion: appVersion,
		Providers:      []updater.Provider{provider},
		Window:         updater.WindowNone,
	}); err != nil {
		return
	}
	service.setUpdater(app)
	// Remember the pending release so DismissUpdate can record a skip for
	// exactly the version the user was offered.
	app.Event.On(updater.EventUpdateAvailable, func(event *application.CustomEvent) {
		if release, ok := event.Data.(*updater.Release); ok {
			service.notePendingRelease(release)
		}
	})
	// The check itself runs once on startup: the frontend calls
	// ServerService.CheckForUpdates after it has registered its event
	// listeners, so the "update available" popup can react to the result.
}
