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

type MovieMagnetLinksResponse struct {
	Success bool                         `json:"success"`
	Results []torrentsTypes.MovieTorrent `json:"results"`
}

func GetMovieTorrentsByImdb() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		log.Println("Searching torrents:", vars)

		output := torrents.GetMovieTorrents(getMovieParams(vars["imdb"], ""), getSourceParams(vars["providers"]))
		if len(output) > 0 {
			io.WriteString(w, movieTorrentsList(output))

		} else {
			http.Error(w, noMovieTorrentsFound(), http.StatusNotFound)
		}
	}
}

func GetMovieTorrentsByQuery() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		log.Println("Searching torrents:", vars)

		output := torrents.GetMovieTorrents(getMovieParams("", vars["query"]), getSourceParams(vars["providers"]))
		if len(output) > 0 {
			io.WriteString(w, movieTorrentsList(output))
		} else {
			http.Error(w, noMovieTorrentsFound(), http.StatusNotFound)
		}
	}
}

func GetMovieTorrentsByImdbAndQuery() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		log.Println("Searching torrents:", vars)

		output := torrents.GetMovieTorrents(getMovieParams(vars["imdb"], vars["query"]), getSourceParams(vars["providers"]))
		if len(output) > 0 {
			io.WriteString(w, movieTorrentsList(output))
		} else {
			http.Error(w, noMovieTorrentsFound(), http.StatusNotFound)
		}
	}
}

func getMovieParams(imdb string, query string) torrentsTypes.MovieParams {
	movieParams := torrentsTypes.MovieParams{}

	movieParams.ImdbId = imdb
	movieParams.SearchText = ""

	params, err := url.ParseQuery(query)
	if err == nil {
		if params["title"] != nil {
			movieParams.SearchText += params["title"][0]
		}
		if params["releaseyear"] != nil {
			movieParams.SearchText += " " + params["releaseyear"][0]
		}
	}

	return movieParams
}

func movieTorrentsList(results []torrentsTypes.MovieTorrent) string {
	message := MovieMagnetLinksResponse{
		Success: true,
		Results: results,
	}

	messageString, _ := json.Marshal(message)

	log.Println("Found", len(results), "torrents.")

	return string(messageString)
}

func noMovieTorrentsFound() string {
	message := MessageResponse{
		Success: false,
		Message: "No torrents found.",
	}

	messageString, _ := json.Marshal(message)

	log.Println("No torrents found.")

	return string(messageString)
}
