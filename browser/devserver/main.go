package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8080", "address for the browser harness")
	server := flag.String("server", "http://127.0.0.1:9000", "White Raven server URL")
	flag.Parse()
	serverURL, err := url.Parse(*server)
	if err != nil || (serverURL.Scheme != "http" && serverURL.Scheme != "https") || serverURL.Host == "" {
		log.Fatal("-server must be an absolute HTTP or HTTPS URL")
	}
	*server = strings.TrimRight(*server, "/")

	root, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "widget", "app.json")); err != nil {
		log.Fatal("run this command from the white-raven repository root")
	}

	handler := http.NewServeMux()
	handler.Handle("/images/", noStore(http.StripPrefix("/images/", http.FileServer(http.Dir(filepath.Join(root, "widget", "images"))))))
	handler.HandleFunc("/", func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Cache-Control", "no-store")
		if request.URL.Path == "/" {
			http.ServeFile(response, request, filepath.Join(root, "browser", "index.html"))
			return
		}
		http.FileServer(http.Dir(root)).ServeHTTP(response, request)
	})
	handler.HandleFunc("/browser-config.js", func(response http.ResponseWriter, _ *http.Request) {
		value, _ := json.Marshal(*server)
		response.Header().Set("Cache-Control", "no-store")
		response.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		fmt.Fprintf(response, "window.WHITE_RAVEN_SERVER_URL = %s;\n", value)
	})

	log.Printf("White Raven browser harness: http://%s/", *listen)
	log.Printf("White Raven server: %s", *server)
	log.Fatal(http.ListenAndServe(*listen, handler))
}

func noStore(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Cache-Control", "no-store")
		handler.ServeHTTP(response, request)
	})
}
