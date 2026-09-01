package v0

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/nyakaspeter/white-raven/server/pkg/mediainfo"
	mediainfotypes "github.com/nyakaspeter/white-raven/server/pkg/mediainfo/types"
)

type ShowEpisodesResponse struct {
	Success bool                           `json:"success"`
	Results []mediainfotypes.TvMazeEpisode `json:"results"`
}

func GetShowEpisodesByImdb() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		log.Println("Fetching show episodes:", vars)

		showIds := mediainfotypes.ShowIds{}
		showIds.ImdbId = vars["imdb"]

		episodes := mediainfo.GetShowEpisodes(showIds)
		if len(episodes) == 0 {
			http.Error(w, noTvMazeDataFound(), http.StatusNotFound)
			return
		}

		io.WriteString(w, showEpisodeList(episodes))
	}
}

func GetShowEpisodesByTvdb() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		log.Println("Fetching show episodes:", vars)

		showIds := mediainfotypes.ShowIds{}
		showIds.TvdbId = vars["tvdb"]

		episodes := mediainfo.GetShowEpisodes(showIds)
		if len(episodes) == 0 {
			http.Error(w, noTvMazeDataFound(), http.StatusNotFound)
			return
		}

		io.WriteString(w, showEpisodeList(episodes))
	}
}

func GetShowEpisodesByImdbAndTvdb() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		log.Println("Fetching show episodes:", vars)

		showIds := mediainfotypes.ShowIds{}
		showIds.ImdbId = vars["imdb"]
		showIds.TvdbId = vars["tvdb"]

		episodes := mediainfo.GetShowEpisodes(showIds)
		if len(episodes) == 0 {
			http.Error(w, noTvMazeDataFound(), http.StatusNotFound)
			return
		}

		io.WriteString(w, showEpisodeList(episodes))
	}
}

func showEpisodeList(episodes []mediainfotypes.TvMazeEpisode) string {
	response := ShowEpisodesResponse{
		Success: true,
		Results: episodes,
	}

	log.Println("Found", len(episodes), "episodes.")

	json, _ := json.Marshal(response)
	return string(json)
}

func noTvMazeDataFound() string {
	message := MessageResponse{
		Success: false,
		Message: "No TVMaze data found.",
	}

	messageString, _ := json.Marshal(message)

	log.Println("No TVMaze data found.")

	return string(messageString)
}
