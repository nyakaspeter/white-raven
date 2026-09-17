package httpserver

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/nyakaspeter/white-raven/server/internal/settings"
	"github.com/nyakaspeter/white-raven/server/pkg/utils"
)

var quitSignal chan os.Signal

var serverHost string
var serverPort int
var httpServer *http.Server

func StartHttpServer(appQuitSignal chan os.Signal) (*http.Server, error) {
	quitSignal = appQuitSignal

	httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", *settings.Host, *settings.Port),
		ReadTimeout:  38 * time.Second,
		WriteTimeout: 38 * time.Second,
		Handler:      routesHandler(),
	}

	localIP := *settings.Host
	if localIP == "" {
		localIP = utils.GetLocalIP()
	}

	serverHost = localIP
	serverPort = *settings.Port

	address := fmt.Sprintf("http://%s:%d", serverHost, serverPort)

	listener, err := net.Listen("tcp", httpServer.Addr)
	if err != nil {
		return nil, err
	}

	go func() {
		if err := httpServer.Serve(listener); err != nil {
			// cannot panic, because this probably is an intentional close
			if err != http.ErrServerClosed {
				log.Printf("HTTP Server Error: %s\n", err)
			}
			time.Sleep(1 * time.Nanosecond)
			quitSignal <- os.Kill
		}
	}()

	// Must appear
	log.Printf("White Raven Server started on address: %s\n", address)

	return httpServer, nil
}

func StopHttpServer() {
	if httpServer == nil {
		return
	}

	httpServer.Close()
}
