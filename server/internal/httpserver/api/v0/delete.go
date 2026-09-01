package v0

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/nyakaspeter/white-raven/server/internal/torrentclient"
)

func DeleteTorrent() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		log.Println("Deleting torrent:", vars)

		err := torrentclient.RemoveTorrent(vars["hash"])
		if err != nil {
			http.Error(w, torrentNotFound(), http.StatusNotFound)
			return
		}

		io.WriteString(w, deletedTorrent())
	}
}

func deletedTorrent() string {
	message := MessageResponse{
		Success: true,
		Message: "Torrent deleted.",
	}

	messageString, _ := json.Marshal(message)

	log.Println("Deleted torrent.")

	return string(messageString)
}
