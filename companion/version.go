package main

// appVersion is the version of the running companion. Release builds stamp
// the real version with -ldflags "-X main.appVersion=<version>" (see the
// per-platform Taskfiles, which read it from build/config.yml). Development
// builds keep the sentinel below, which disables the auto updater so local
// builds are never prompted to install a published release.
var appVersion = "0.0.0-dev"
