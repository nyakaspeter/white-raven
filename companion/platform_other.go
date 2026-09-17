//go:build !android

package main

func startPlatformBackground(reason string, message string) {}
func stopPlatformBackground(reason string)                  {}
