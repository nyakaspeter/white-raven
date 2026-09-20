package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const harbrrRepository = "https://github.com/nyakaspeter/harbrr.git"

var commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

func main() {
	revisionBytes, err := os.ReadFile("harbrr.version")
	check(err)
	revision := strings.TrimSpace(string(revisionBytes))
	if !commitPattern.MatchString(revision) {
		check(fmt.Errorf("harbrr.version must contain one full Git commit hash"))
	}

	temporaryRoot, err := os.MkdirTemp("", "white-raven-harbrr-")
	check(err)
	defer os.RemoveAll(temporaryRoot)

	source := filepath.Join(temporaryRoot, "source")
	check(run("", "git", "clone", "--quiet", "--no-checkout", harbrrRepository, source))
	check(run("", "git", "-C", source, "checkout", "--quiet", revision))

	webDir := filepath.Join(source, "web")
	check(run(webDir, "npx", "--yes", "bun@1.4.0", "install", "--frozen-lockfile"))
	check(run(webDir, "npx", "--yes", "bun@1.4.0", "run", "build"))

	target, err := filepath.Abs("harbrr-build")
	check(err)
	if filepath.Base(target) != "harbrr-build" {
		check(fmt.Errorf("refusing to replace unexpected output path %q", target))
	}
	check(os.RemoveAll(target))
	check(copyTree(source, target))
}

func run(directory, name string, arguments ...string) error {
	command := exec.Command(name, arguments...)
	if directory != "" {
		command.Dir = directory
	}
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Stdin = os.Stdin
	if err := command.Run(); err != nil {
		return fmt.Errorf("run %s: %w", name, err)
	}
	return nil
}

func copyTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if entry.IsDir() && relative != "." &&
			(entry.Name() == ".git" || entry.Name() == "node_modules") {
			return filepath.SkipDir
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			_ = input.Close()
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			_ = input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		inputCloseErr := input.Close()
		outputCloseErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if inputCloseErr != nil {
			return inputCloseErr
		}
		return outputCloseErr
	})
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
