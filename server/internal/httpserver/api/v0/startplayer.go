package v0

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/nyakaspeter/white-raven/server/pkg/mediaplayer"
	mediaplayertypes "github.com/nyakaspeter/white-raven/server/pkg/mediaplayer/types"
)

func StartMediaPlayer() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		log.Println("Starting media player:", vars)

		path, err := decodeURLBase64(vars["base64path"])
		if err != nil {
			http.Error(w, failedToOpenMediaPlayer(), http.StatusNotFound)
			return
		}

		args, err := decodeURLBase64(vars["base64args"])
		if err != nil {
			http.Error(w, failedToOpenMediaPlayer(), http.StatusNotFound)
			return
		}

		params := mediaplayertypes.MediaPlayerParams{}
		params.ExecutablePath = string(path)
		params.ExecutableArgs = string(args)

		err = mediaplayer.StartMediaPlayer(params)
		if err != nil {
			http.Error(w, failedToOpenMediaPlayer(), http.StatusNotFound)
			return
		}

		io.WriteString(w, successMessage())
	}
}

func failedToOpenMediaPlayer() string {
	message := MessageResponse{
		Success: false,
		Message: "Failed to open media player.",
	}

	messageString, _ := json.Marshal(message)

	log.Println("Failed to open media player.")

	return string(messageString)
}
