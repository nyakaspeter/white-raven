#!/usr/bin/env bash

set -euo pipefail

version="${1:-}"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	echo "Usage: $0 MAJOR.MINOR.PATCH" >&2
	exit 1
fi

IFS=. read -r major minor patch <<<"$version"
version_code=$((10#$major * 1000000 + 10#$minor * 1000 + 10#$patch))

export COMPANION_VERSION="$version"
export COMPANION_VERSION_CODE="$version_code"

perl -0pi -e 's/^  version: "[^"]+"/  version: "$ENV{COMPANION_VERSION}"/m' companion/build/config.yml
perl -0pi -e 's{(<key>CFBundle(?:ShortVersionString|Version)</key>\s*<string>)[^<]+}{$1$ENV{COMPANION_VERSION}}g' \
	companion/build/darwin/Info.plist \
	companion/build/ios/Info.plist
perl -0pi -e 's/("(?:file_version|ProductVersion)"\s*:\s*")[^"]+/${1}$ENV{COMPANION_VERSION}/g' \
	companion/build/windows/info.json
perl -0pi -e 's/(assemblyIdentity type="win32" name="com\.whiteraven\.server" version=")[^"]+/${1}$ENV{COMPANION_VERSION}.0/' \
	companion/build/windows/wails.exe.manifest
perl -0pi -e 's/(!define INFO_PRODUCTVERSION ")[^"]+/${1}$ENV{COMPANION_VERSION}/' \
	companion/build/windows/nsis/wails_tools.nsh
perl -0pi -e 's/^(version:\s*)"[^"]+"/${1}"$ENV{COMPANION_VERSION}"/m' \
	companion/build/linux/nfpm/nfpm.yaml
perl -0pi -e 's/(versionCode\s+)\d+/${1}$ENV{COMPANION_VERSION_CODE}/; s/(versionName\s+")[^"]+/${1}$ENV{COMPANION_VERSION}/' \
	companion/build/android/app/build.gradle

echo "Set companion package metadata to $version (Android version code $version_code)."
