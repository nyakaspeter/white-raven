package main

import (
	"flag"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"

	"github.com/nyakaspeter/white-raven/server/internal/httpserver"
	"github.com/nyakaspeter/white-raven/server/internal/settings"
	"github.com/nyakaspeter/white-raven/server/internal/torrentclient"
)

var quitSignal = make(chan os.Signal, 1)

func main() {
	settings.Init()
	log.SetFlags(0)
	signal.Notify(quitSignal, os.Interrupt)

	if *settings.StorageType != "memory" && *settings.StorageType != "file" {
		log.Printf("missing or invalid -storagetype value: \"%s\" (must be set to \"memory\" or \"file\")\nUsage of %s:\n", *settings.StorageType, os.Args[0])
		flag.PrintDefaults()
		os.Exit(2)
	}

	if *settings.StorageType == "memory" && *settings.MemorySize < 64 {
		log.Printf("the memory size is too small: \"%dMB\" (must be set to minimum 64MB)\nUsage of %s:\n", *settings.MemorySize, os.Args[0])
		flag.PrintDefaults()
		os.Exit(2)
	}

	if *settings.StorageType == "file" && *settings.DownloadDir == "" {
		log.Printf("empty -dir value (must be set if selected -storagetype is \"file\")\nUsage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(2)
	}

	if !*settings.EnableLog {
		log.SetOutput(io.Discard)
		defer log.SetOutput(os.Stderr)
	}

	if *settings.Background {
		args := os.Args[1:]
		for i := 0; i < len(args); i++ {
			if args[i] == "-background=true" || args[i] == "-background" || args[i] == "--background=true" || args[i] == "--background" {
				args[i] = "-background=false"
				break
			}
		}
		for i := 0; i < len(args); i++ {
			if args[i] == "-log=true" || args[i] == "-log" || args[i] == "--log=true" || args[i] == "--log" {
				args[i] = "-log=false"
				break
			}
		}
		cmd := exec.Command(os.Args[0], args...)
		if err := cmd.Start(); err != nil {
			log.Fatal(err)
		}
		log.Println("Running in the background with the following PID number:", cmd.Process.Pid)
		os.Exit(0)
	}

	if _, err := torrentclient.StartTorrentClient(); err != nil {
		quit()
	}

	httpserver.StartHttpServer(quitSignal)

	<-quitSignal
	quit()
}

func quit() {
	log.Println("Quitting.")
	httpserver.StopHttpServer()
	torrentclient.StopTorrentClient()
}
