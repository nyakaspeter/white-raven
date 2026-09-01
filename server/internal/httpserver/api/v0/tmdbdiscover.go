package v0

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/nyakaspeter/white-raven/server/pkg/mediainfo"
	mediainfotypes "github.com/nyakaspeter/white-raven/server/pkg/mediainfo/types"
)

type TmdbMovieResultsResponse struct {
	Success bool                        `json:"success"`
	Results mediainfotypes.MovieResults `json:"results"`
}

type TmdbShowResultsResponse struct {
	Success bool                       `json:"success"`
	Results mediainfotypes.ShowResults `json:"results"`
}

func DiscoverMovies() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		log.Println("Fetching movie list:", vars)

		page, err := strconv.Atoi(vars["page"])
		if err != nil {
			http.Error(w, noTmdbDataFound(), http.StatusNotFound)
			return
		}

		genreIds := []int{}
		if vars["genretype"] != "all" {
			genreIdStrings := strings.Split(vars["genretype"], ",")
			genreIds, err = sliceAtoi(genreIdStrings)
			if err != nil {
				http.Error(w, noTmdbDataFound(), http.StatusNotFound)
				return
			}
		}

		params := mediainfotypes.MovieDiscoverParams{}
		params.SortBy = vars["sort"]
		params.MaxReleaseDate = vars["date"]
		params.GenreIds = genreIds

		results := mediainfo.DiscoverMovies(params, vars["lang"], page)
		if results.TotalResults == 0 {
			http.Error(w, noTmdbDataFound(), http.StatusNotFound)
			return
		}

		io.WriteString(w, movieResultsList(results))
	}
}

func DiscoverShows() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		log.Println("Fetching show list:", vars)

		page, err := strconv.Atoi(vars["page"])
		if err != nil {
			http.Error(w, noTmdbDataFound(), http.StatusNotFound)
			return
		}

		genreIds := []int{}
		if vars["genretype"] != "all" {
			genreIdStrings := strings.Split(vars["genretype"], ",")
			genreIds, err = sliceAtoi(genreIdStrings)
			if err != nil {
				http.Error(w, noTmdbDataFound(), http.StatusNotFound)
				return
			}
		}

		params := mediainfotypes.ShowDiscoverParams{}
		params.SortBy = vars["sort"]
		params.MaxAirDate = vars["date"]
		params.GenreIds = genreIds

		results := mediainfo.DiscoverShows(params, vars["lang"], page)
		if results.TotalResults == 0 {
			http.Error(w, noTmdbDataFound(), http.StatusNotFound)
			return
		}

		io.WriteString(w, showResultsList(results))
	}
}

func sliceAtoi(sa []string) ([]int, error) {
	si := make([]int, 0, len(sa))
	for _, a := range sa {
		i, err := strconv.Atoi(a)
		if err != nil {
			return si, err
		}
		si = append(si, i)
	}
	return si, nil
}

func movieResultsList(results mediainfotypes.MovieResults) string {
	response := TmdbMovieResultsResponse{
		Success: true,
		Results: results,
	}

	log.Println("Found", len(results.Results), "movies.")

	json, _ := json.Marshal(response)
	return string(json)
}

func showResultsList(results mediainfotypes.ShowResults) string {
	response := TmdbShowResultsResponse{
		Success: true,
		Results: results,
	}

	log.Println("Found", len(results.Results), "shows.")

	json, _ := json.Marshal(response)
	return string(json)
}

func noTmdbDataFound() string {
	message := MessageResponse{
		Success: false,
		Message: "No TMDB data found.",
	}

	messageString, _ := json.Marshal(message)

	log.Println("No TMDB data found.")

	return string(messageString)
}
