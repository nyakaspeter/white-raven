package v0

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/nyakaspeter/white-raven/server/internal/settings"
	"github.com/nyakaspeter/white-raven/server/internal/torrentclient"
)

func RestartTorrentClient() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		log.Println("Restarting torrent client:", vars)

		downrate, err := strconv.Atoi(vars["downrate"])
		if err != nil {
			http.Error(w, failedToSetLimits(), http.StatusBadRequest)
			return
		}

		uprate, err := strconv.Atoi(vars["uprate"])
		if err != nil {
			http.Error(w, failedToSetLimits(), http.StatusBadRequest)
			return
		}

		*settings.DownloadRate = downrate
		*settings.UploadRate = uprate

		if err := torrentclient.RestartTorrentClient(); err != nil {
			log.Println("Failed to restart torrent client:", err)
			http.Error(w, torrentClientRestartFailed(), http.StatusInternalServerError)
			return
		}

		io.WriteString(w, torrentClientRestarted())
	}
}

func torrentClientRestartFailed() string {
	message := MessageResponse{
		Success: false,
		Message: "Failed to restart torrent client.",
	}

	messageString, _ := json.Marshal(message)
	return string(messageString)
}

func torrentClientRestarted() string {
	message := MessageResponse{
		Success: true,
		Message: "Restarted torrent client.",
	}

	messageString, _ := json.Marshal(message)

	log.Println("Restarted torrent client.")

	return string(messageString)
}

func failedToSetLimits() string {
	message := MessageResponse{
		Success: false,
		Message: "Failed to set speed limits.",
	}

	messageString, _ := json.Marshal(message)

	log.Println("Failed to set speed limits.")

	return string(messageString)
}
