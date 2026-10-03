package v0

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/nyakaspeter/white-raven/server/pkg/mediainfo"
	mediainfotypes "github.com/nyakaspeter/white-raven/server/pkg/mediainfo/types"
)

type TmdbMovieInfoResponse struct {
	Success bool                     `json:"success"`
	Result  mediainfotypes.MovieInfo `json:"result"`
}

type TmdbShowInfoResponse struct {
	Success bool                    `json:"success"`
	Result  mediainfotypes.ShowInfo `json:"result"`
}

func GetMovieInfo() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		log.Println("Fetching movie info:", vars)

		tmdbid, err := strconv.Atoi(vars["tmdbid"])
		if err != nil {
			http.Error(w, noTmdbDataFound(), http.StatusNotFound)
			return
		}

		result := mediainfo.GetMovieInfo(tmdbid, vars["lang"])
		if result.Id == 0 {
			http.Error(w, noTmdbDataFound(), http.StatusNotFound)
			return
		}

		io.WriteString(w, movieInfo(result))
	}
}

func GetShowInfo() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		log.Println("Fetching show info:", vars)

		tmdbid, err := strconv.Atoi(vars["tmdbid"])
		if err != nil {
			http.Error(w, noTmdbDataFound(), http.StatusNotFound)
			return
		}

		result := mediainfo.GetShowInfo(tmdbid, vars["lang"])
		if result.Id == 0 {
			http.Error(w, noTmdbDataFound(), http.StatusNotFound)
			return
		}

		io.WriteString(w, showInfo(result))
	}
}

func movieInfo(result mediainfotypes.MovieInfo) string {
	response := TmdbMovieInfoResponse{
		Success: true,
		Result:  result,
	}

	log.Println("Returning movie info.")

	json, _ := json.Marshal(response)
	return string(json)
}

func showInfo(result mediainfotypes.ShowInfo) string {
	response := TmdbShowInfoResponse{
		Success: true,
		Result:  result,
	}

	log.Println("Returning show info.")

	json, _ := json.Marshal(response)
	return string(json)
}

type ShowEpisodesResponse struct {
	Success         bool                            `json:"success"`
	Results         []mediainfotypes.ShowEpisode    `json:"results"`
	StreamReference *mediainfotypes.StreamReference `json:"stream_reference,omitempty"`
}

func GetShowEpisodes() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		log.Println("Fetching show episodes:", vars)

		tmdbId, err := strconv.Atoi(vars["tmdb"])
		if err != nil {
			http.Error(w, noShowEpisodesFound(), http.StatusNotFound)
			return
		}

		data, err := mediainfo.GetShowEpisodesByTmdb(tmdbId)
		if err != nil || len(data.Episodes) == 0 {
			http.Error(w, noShowEpisodesFound(), http.StatusNotFound)
			return
		}

		io.WriteString(w, showEpisodeList(data))
	}
}

func showEpisodeList(data mediainfo.ShowEpisodeData) string {
	message := ShowEpisodesResponse{
		Success:         true,
		Results:         data.Episodes,
		StreamReference: data.StreamReference,
	}

	log.Println("Found", len(data.Episodes), "episodes.")

	output, _ := json.Marshal(message)
	return string(output)
}

func noShowEpisodesFound() string {
	message := MessageResponse{
		Success: false,
		Message: "No show episodes found.",
	}

	messageString, _ := json.Marshal(message)

	log.Println("No show episodes found.")

	return string(messageString)
}
