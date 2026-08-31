package main

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/js"
	"github.com/tdewolff/minify/v2/json"
)

const (
	builderVersion string = "0.1.0"

	serverName        string = "wrserver"
	legacyRoot        string = "widget/"
	serverRoot        string = legacyRoot + "server/"
	serverOutputPath  string = serverRoot + serverName
	serverPathInZip string = "server/" + serverName
	defaultZipName    string = "build/whiteraven.zip"
	configFilePath    string = legacyRoot + "config.xml" // Needed for version detection

	mainName            string = "Main.js"
	detectRootCode      string = "var detectedValues = DetectRoot();"
	blockRootDetectCode string = "var detectedValues = { isRooted: false, isSupported: false };"
)

// Version detection from config.xml file
type ConfigXML struct {
	Version string `xml:"ver"`
}

// Some global variables
var zipName string = defaultZipName
var widgetVersion = ""

func readVersionFromConfig(path string) string {
	xmlFile, err := os.Open(path)
	if err != nil {
		return "" // Do not generate error
	}
	defer xmlFile.Close()

	byteValue, err := ioutil.ReadAll(xmlFile)
	if err != nil {
		return "" // Do not generate error
	}

	var config ConfigXML
	xml.Unmarshal(byteValue, &config)

	return config.Version
}

func validateServerFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	var ident [4]byte
	if _, err := io.ReadFull(file, ident[:]); err != nil {
		return err
	}

	if ident[0] != '\x7f' || ident[1] != 'E' || ident[2] != 'L' || ident[3] != 'F' {
		return errors.New("the specified file is not a Linux executable")
	}

	return nil
}

func copyServerFile(sourcePath string) error {
	if err := validateServerFile(sourcePath); err != nil {
		return err
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(serverOutputPath)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(destination, source)
	closeErr := destination.Close()

	if copyErr != nil {
		os.Remove(serverOutputPath)
		return copyErr
	}

	if closeErr != nil {
		os.Remove(serverOutputPath)
		return closeErr
	}

	return nil
}

func removeServerFile() {
	err := os.Remove(serverOutputPath)
	if err != nil && !os.IsNotExist(err) {
		fmt.Printf("Unable to delete temporary %s file.\n -> %v\n", serverName, err)
	}
}

func buildReleaseFromDirectory(root string, buildRootless bool) error {
	// Create filename
	if buildRootless == true {
		zipName = strings.Replace(defaultZipName, ".", "-rootless-"+widgetVersion+".", -1)
	} else {
		zipName = strings.Replace(defaultZipName, ".", "-"+widgetVersion+".", -1)
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
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return []byte{}, err
	}

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
	fmt.Printf("White Raven Widget Builder For Legacy Samsung Smart TVs v%s\n", builderVersion)

	rootless := flag.NewFlagSet("rootless", flag.ExitOnError)

	rooted := flag.NewFlagSet("rooted", flag.ExitOnError)
	serverFile := rooted.String(
		"serverfile",
		"",
		"Specify the path of the \""+serverName+"\" executable compiled for Samsung Smart TVs.",
	)

	if len(os.Args) < 2 {
		usageGuide()
	}

	buildRootless := false
	serverCopied := false

	// Check if config.xml file and version number exist.
	widgetVersion = readVersionFromConfig(configFilePath)

	switch os.Args[1] {
	case "rooted":
		rooted.Parse(os.Args[2:])

		if len(rooted.Args()) != 0 {
			fmt.Printf("Unknown parameter(s): %v\n", rooted.Args())
			fmt.Printf("Usage of %s:\n", os.Args[1])
			rooted.PrintDefaults()
			os.Exit(2)
		} else if *serverFile == "" {
			fmt.Printf("Usage of %s:\n", os.Args[1])
			rooted.PrintDefaults()
			os.Exit(2)
		} else if widgetVersion == "" {
			fmt.Printf(
				"The \"%s\" file cannot be found or cannot contain version information.\n",
				configFilePath,
			)
			os.Exit(2)
		}

		fmt.Printf("\nAdd %s directly to the rooted widget.\n", serverName)

		err := copyServerFile(*serverFile)
		if err != nil {
			fmt.Printf("Unable to add %s to the widget.\n -> %v\n", serverName, err)
			os.Exit(2)
		}

		serverCopied = true

	case "rootless":
		rootless.Parse(os.Args[2:])

		if len(rootless.Args()) != 0 {
			fmt.Printf("Unknown parameter(s): %v\n", rootless.Args())
			os.Exit(2)
		} else if widgetVersion == "" {
			fmt.Printf(
				"The \"%s\" file is not found or does not contain widget version information.\n",
				configFilePath,
			)
			os.Exit(2)
		}

		buildRootless = true

	default:
		usageGuide()
	}

	err := buildReleaseFromDirectory(legacyRoot, buildRootless)
	if err != nil {
		if serverCopied {
			removeServerFile()
		}

		fmt.Printf("Unable to create %s widget.\n -> %v\n", zipName, err)
		os.Exit(2)
	}

	if serverCopied {
		removeServerFile()
	}

	fmt.Printf(
		"\nBuild completed successfully!\n%s is ready to install.\n",
		zipName,
	)
}