package v0

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/nyakaspeter/white-raven/server/pkg/torrents"
	torrentsTypes "github.com/nyakaspeter/white-raven/server/pkg/torrents/types"
)

type ShowMagnetLinksResponse struct {
	Success bool                        `json:"success"`
	Results []torrentsTypes.ShowTorrent `json:"results"`
}

func GetShowTorrentsByImdb() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		log.Println("Searching torrents:", vars)

		output := torrents.GetShowTorrents(getShowParams(vars["imdb"], "", vars["season"], vars["episode"]), getSourceParams(vars["providers"]))
		if len(output) > 0 {
			io.WriteString(w, showTorrentsList(output))
		} else {
			http.Error(w, noShowTorrentsFound(), http.StatusNotFound)
		}
	}
}

func GetShowTorrentsByQuery() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		log.Println("Searching torrents:", vars)

		output := torrents.GetShowTorrents(getShowParams("", vars["query"], vars["season"], vars["episode"]), getSourceParams(vars["providers"]))
		if len(output) > 0 {
			io.WriteString(w, showTorrentsList(output))
		} else {
			http.Error(w, noShowTorrentsFound(), http.StatusNotFound)
		}
	}
}

func GetShowTorrentsByImdbAndQuery() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		log.Println("Searching torrents:", vars)

		output := torrents.GetShowTorrents(getShowParams(vars["imdb"], vars["query"], vars["season"], vars["episode"]), getSourceParams(vars["providers"]))
		if len(output) > 0 {
			io.WriteString(w, showTorrentsList(output))
		} else {
			http.Error(w, noShowTorrentsFound(), http.StatusNotFound)
		}
	}
}

func getShowParams(imdb string, query string, season string, episode string) torrentsTypes.ShowParams {
	showParams := torrentsTypes.ShowParams{}

	showParams.ImdbId = imdb
	showParams.SearchText = ""
	showParams.Season = season
	showParams.Episode = episode

	params, err := url.ParseQuery(query)
	if err == nil {
		if params["title"] != nil {
			showParams.SearchText += params["title"][0]
		}
	}

	return showParams
}

func showTorrentsList(results []torrentsTypes.ShowTorrent) string {
	message := ShowMagnetLinksResponse{
		Success: true,
		Results: results,
	}

	messageString, _ := json.Marshal(message)

	log.Println("Found", len(results), "torrents.")

	return string(messageString)
}

func noShowTorrentsFound() string {
	message := MessageResponse{
		Success: false,
		Message: "No torrents found.",
	}

	messageString, _ := json.Marshal(message)

	log.Println("No torrents found.")

	return string(messageString)
}
