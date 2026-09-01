package v0

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
)

func About(version string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, serverInfo(version))
	}
}

func serverInfo(version string) string {
	message := MessageResponse{
		Success: true,
		Message: "White Raven Server v" + version,
	}

	messageString, _ := json.Marshal(message)

	log.Println("Returning server info.")

	return string(messageString)
}
