// Package version exposes the product version embedded by the build command.
package version

// Value is replaced with -ldflags at build time. Development builds use 0.0.0.
var Value = "0.0.0"
