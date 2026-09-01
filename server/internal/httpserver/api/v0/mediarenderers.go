package v0

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/nyakaspeter/white-raven/server/pkg/dlnacast"
	dlnacasttypes "github.com/nyakaspeter/white-raven/server/pkg/dlnacast/types"
)

type MediaRenderersResponse struct {
	Success bool                        `json:"success"`
	Results []dlnacasttypes.MediaDevice `json:"results"`
}

func GetMediaRenderers() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("Looking for media renderers")

		devices := dlnacast.GetMediaDevices()

		if len(devices) == 0 {
			http.Error(w, noMediaRenderersFound(), http.StatusNotFound)
			return
		}

		io.WriteString(w, mediaRenderersList(devices))
	}
}

func mediaRenderersList(renderers []dlnacasttypes.MediaDevice) string {
	message := MediaRenderersResponse{
		Success: true,
		Results: renderers,
	}

	messageString, _ := json.Marshal(message)

	log.Println("Found", len(renderers), "media renderers.")

	return string(messageString)
}

func noMediaRenderersFound() string {
	message := MessageResponse{
		Success: false,
		Message: "No media renderers found.",
	}

	messageString, _ := json.Marshal(message)

	log.Println("No media renderers found.")

	return string(messageString)
}
