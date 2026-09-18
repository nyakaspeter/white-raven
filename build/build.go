package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/js"
	"github.com/tdewolff/minify/v2/json"
)

const (
	serverName         = "wrserver"
	legacyRoot         = "widget/"
	serverRoot         = legacyRoot + "server/"
	serverOutputPath   = serverRoot + serverName
	serverPathInZip    = "server/" + serverName
	defaultZipName     = "build/whiteraven.zip"
	versionPlaceholder = "__WHITE_RAVEN_VERSION__"
	versionVariable    = "github.com/nyakaspeter/white-raven/build/version.Value"

	mainName            string = "Main.js"
	detectRootCode      string = "var detectedValues = DetectRoot();"
	blockRootDetectCode string = "var detectedValues = { isRooted: false, isSupported: false };"
)

var zipName string = defaultZipName
var widgetVersion = ""

var validVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)

func buildServer(version string) error {
	ldflags := fmt.Sprintf("-s -w -X %s=%s", versionVariable, version)
	command := exec.Command("go", "build", "-trimpath", "-ldflags", ldflags, "-o", serverOutputPath, "./server")
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Env = buildEnvironment(map[string]string{
		"CGO_ENABLED": "0",
		"GOARCH":      "arm",
		"GOARM":       "7",
		"GOOS":        "linux",
	})
	return command.Run()
}

func buildEnvironment(overrides map[string]string) []string {
	environment := make([]string, 0, len(os.Environ())+len(overrides))
	for _, entry := range os.Environ() {
		name, _, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		if _, overridden := overrides[name]; !overridden {
			environment = append(environment, entry)
		}
	}
	for name, value := range overrides {
		environment = append(environment, name+"="+value)
	}
	return environment
}

func removeServerFile() {
	err := os.Remove(serverOutputPath)
	if err != nil && !os.IsNotExist(err) {
		fmt.Printf("Unable to delete temporary %s file.\n -> %v\n", serverName, err)
	}
}

func buildReleaseFromDirectory(root string, buildRootless bool) error {
	if buildRootless {
		zipName = strings.TrimSuffix(defaultZipName, ".zip") + "-rootless-" + widgetVersion + ".zip"
	} else {
		zipName = strings.TrimSuffix(defaultZipName, ".zip") + "-" + widgetVersion + ".zip"
	}

	// Create output file
	outFile, err := os.Create(zipName)
	if err != nil {
		return err
	}
	defer outFile.Close()

	zipWriter := zip.NewWriter(outFile)
	defer zipWriter.Close()

	fmt.Println("Minify css, js and json files.")

	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Remove server directory from rootless widget
		if buildRootless == true {
			tmpPath := strings.Replace(path, "\\", "/", -1)
			tmpServerRoot := strings.TrimSuffix(serverRoot, "/")

			if tmpPath == tmpServerRoot || strings.HasPrefix(tmpPath, tmpServerRoot+"/") {
				return nil
			}
		}

		miniData := []byte{}
		if info.IsDir() == false {
			miniData, err = minifyFile(path, buildRootless)
			if err != nil {
				return err
			}
		}

		path = strings.Replace(path, "\\", "/", -1)
		relPath := strings.TrimPrefix(path, root)

		err = addFileToZip(zipWriter, miniData, relPath, info)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func minifyFile(path string, buildRootless bool) ([]byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return []byte{}, err
	}
	content = bytes.ReplaceAll(content, []byte(versionPlaceholder), []byte(widgetVersion))

	r := bytes.NewBuffer(content)
	var buff bytes.Buffer

	m := minify.New()

	switch filepath.Ext(path) {
	case ".js":
		// Block root detection in Main.js with replacing some part of the source code.
		if buildRootless == true && filepath.Base(path) == mainName {
			content = bytes.Replace(content, []byte(detectRootCode), []byte(blockRootDetectCode), 1)
			r = bytes.NewBuffer(content)
		}

		err = js.Minify(m, &buff, r, nil)
		if err != nil {
			return []byte{}, err
		}

		return buff.Bytes(), nil

	case ".css":
		err = css.Minify(m, &buff, r, nil)
		if err != nil {
			return []byte{}, err
		}

		return buff.Bytes(), nil

	case ".json":
		err = json.Minify(m, &buff, r, nil)
		if err != nil {
			return []byte{}, err
		}

		return buff.Bytes(), nil
	}

	return content, nil
}

func addFileToZip(zipWriter *zip.Writer, data []byte, path string, info os.FileInfo) error {
	if path == "" {
		return nil
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}

	header.Name = path

	if info.IsDir() == true {
		header.Name += "/"
		header.SetMode(0755)
	} else {
		header.Method = zip.Deflate

		if path == serverPathInZip {
			header.SetMode(0755)
		} else {
			header.SetMode(0644)
		}

		header.UncompressedSize = uint32(len(data))

		switch filepath.Ext(path) {
		case ".js":
			header.SetModTime(time.Now())
		case ".css":
			header.SetModTime(time.Now())
		case ".json":
			header.SetModTime(time.Now())
		}
	}

	zipFile, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	if info.IsDir() == true {
		return nil
	}

	_, err = zipFile.Write(data)
	if err != nil {
		return err
	}

	return nil
}

func usageGuide() {
	fmt.Printf("Usage of %s:\n", os.Args[0])

	rootedText := "rooted"
	rootedDesc := "Build widget for rooted Samsung Smart TVs."

	rootlessText := "rootless"
	rootlessDesc := "Build widget for Samsung Smart TVs without root."

	fmt.Printf("%*s\n", 2+len(rootedText), rootedText)
	fmt.Printf("%*s\n", 8+len(rootedDesc), rootedDesc)
	fmt.Printf("%*s\n", 2+len(rootlessText), rootlessText)
	fmt.Printf("%*s\n", 8+len(rootlessDesc), rootlessDesc)

	os.Exit(2)
}

func main() {
	fmt.Println("White Raven Widget Builder For Legacy Samsung Smart TVs")

	rootless := flag.NewFlagSet("rootless", flag.ExitOnError)
	rootlessVersion := rootless.String("version", "", "Version to embed, for example 0.8.0.")

	rooted := flag.NewFlagSet("rooted", flag.ExitOnError)
	rootedVersion := rooted.String("version", "", "Version to embed, for example 0.8.0.")

	if len(os.Args) < 2 {
		usageGuide()
	}

	buildRootless := false
	serverBuilt := false

	switch os.Args[1] {
	case "rooted":
		rooted.Parse(os.Args[2:])
		widgetVersion = strings.TrimSpace(*rootedVersion)

		if len(rooted.Args()) != 0 {
			fmt.Printf("Unknown parameter(s): %v\n", rooted.Args())
			fmt.Printf("Usage of %s:\n", os.Args[1])
			rooted.PrintDefaults()
			os.Exit(2)
		} else if !validVersion.MatchString(widgetVersion) {
			fmt.Println("-version must use semantic version form, for example 0.8.0")
			fmt.Printf("Usage of %s:\n", os.Args[1])
			rooted.PrintDefaults()
			os.Exit(2)
		}

		fmt.Printf("\nBuild %s for Samsung Smart TV Linux/ARMv7.\n", serverName)

		err := buildServer(widgetVersion)
		if err != nil {
			removeServerFile()
			fmt.Printf("Unable to build %s.\n -> %v\n", serverName, err)
			os.Exit(2)
		}

		serverBuilt = true

	case "rootless":
		rootless.Parse(os.Args[2:])
		widgetVersion = strings.TrimSpace(*rootlessVersion)

		if len(rootless.Args()) != 0 {
			fmt.Printf("Unknown parameter(s): %v\n", rootless.Args())
			os.Exit(2)
		} else if !validVersion.MatchString(widgetVersion) {
			fmt.Println("-version must use semantic version form, for example 0.8.0")
			fmt.Printf("Usage of %s:\n", os.Args[1])
			rootless.PrintDefaults()
			os.Exit(2)
		}

		buildRootless = true

	default:
		usageGuide()
	}

	err := buildReleaseFromDirectory(legacyRoot, buildRootless)
	if err != nil {
		if serverBuilt {
			removeServerFile()
		}

		fmt.Printf("Unable to create %s widget.\n -> %v\n", zipName, err)
		os.Exit(2)
	}

	if serverBuilt {
		removeServerFile()
	}

	fmt.Printf(
		"\nBuild completed successfully!\n%s is ready to install.\n",
		zipName,
	)
}
