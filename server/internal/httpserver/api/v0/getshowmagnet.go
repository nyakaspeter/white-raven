package v0

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/nyakaspeter/white-raven/server/pkg/mediainfo"
	"github.com/nyakaspeter/white-raven/server/pkg/torrents"
	torrentsTypes "github.com/nyakaspeter/white-raven/server/pkg/torrents/types"
)

var streamImdbPattern = regexp.MustCompile(`^tt\d{7,10}$`)

type ShowMagnetLinksResponse struct {
	Success bool                        `json:"success"`
	Results []torrentsTypes.ShowTorrent `json:"results"`
}

func GetShowTorrentsByImdb() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		log.Println("Searching torrents:", vars)

		output := torrents.GetShowTorrents(getShowParams(r, vars["imdb"], "", vars["season"], vars["episode"]), getSourceParams(vars["providers"]))
		writeShowTorrents(w, output)
	}
}

func GetShowTorrentsByQuery() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		log.Println("Searching torrents:", vars)

		output := torrents.GetShowTorrents(getShowParams(r, "", vars["query"], vars["season"], vars["episode"]), getSourceParams(vars["providers"]))
		writeShowTorrents(w, output)
	}
}

func GetShowTorrentsByImdbAndQuery() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		log.Println("Searching torrents:", vars)

		output := torrents.GetShowTorrents(getShowParams(r, vars["imdb"], vars["query"], vars["season"], vars["episode"]), getSourceParams(vars["providers"]))
		writeShowTorrents(w, output)
	}
}

func GetShowTorrentsByTmdb() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		log.Println("Searching torrents:", vars)

		tmdbId, err := strconv.Atoi(vars["tmdb"])
		if err != nil {
			writeShowTorrents(w, nil)
			return
		}

		params := resolveTmdbShowParams(tmdbId, vars["season"], vars["episode"])
		output := torrents.GetShowTorrents(params, getSourceParams(vars["providers"]))
		writeShowTorrents(w, output)
	}
}

func resolveTmdbShowParams(tmdbId int, season string, episode string) torrentsTypes.ShowParams {
	info := mediainfo.GetShowInfo(tmdbId, "en")
	if info.Id == 0 {
		return torrentsTypes.ShowParams{}
	}

	if info.ExternalIds.ImdbId != "" {
		return torrentsTypes.ShowParams{
			ImdbId:  info.ExternalIds.ImdbId,
			Season:  season,
			Episode: episode,
		}
	}

	seasonNumber, err := strconv.Atoi(season)
	if err == nil && seasonNumber > 0 {
		seasonInfo := mediainfo.GetShowSeason(tmdbId, seasonNumber, "en")
		if len(seasonInfo.Episodes) > 0 {
			name := info.OriginalTitle
			if name == "" {
				name = info.Title
			}
			reference := mediainfo.ResolveStreamReferences(name, []mediainfo.SeasonEpisodes{
				{Number: seasonNumber, Episodes: seasonInfo.Episodes},
			})
			if reference != nil {
				if mapped, ok := reference.Seasons[season]; ok && mapped > 0 {
					return torrentsTypes.ShowParams{
						ImdbId:  reference.ImdbId,
						Season:  strconv.Itoa(mapped),
						Episode: episode,
					}
				}
			}
		}
	}

	text := info.OriginalTitle
	if text == "" {
		text = info.Title
	}
	return torrentsTypes.ShowParams{
		SearchText: text,
		Season:     season,
		Episode:    episode,
	}
}

func getShowParams(r *http.Request, imdb string, query string, season string, episode string) torrentsTypes.ShowParams {
	showParams := torrentsTypes.ShowParams{}

	showParams.ImdbId = imdb
	showParams.SearchText = ""
	showParams.Season = season
	showParams.Episode = episode

	params := url.Values{}
	if parsed, err := url.ParseQuery(query); err == nil {
		params = parsed
	}
	if parsed, err := url.ParseQuery(r.URL.RawQuery); err == nil {
		for key, values := range parsed {
			if _, ok := params[key]; !ok {
				params[key] = values
			}
		}
	}

	if params["title"] != nil {
		showParams.SearchText += params["title"][0]
	}
	if streamImdb := params["streamimdb"]; len(streamImdb) > 0 && streamImdbPattern.MatchString(streamImdb[0]) {
		showParams.ImdbId = streamImdb[0]
		showParams.SearchText = ""
	}
	if streamSeason := params["streamseason"]; len(streamSeason) > 0 && isNonNegativeNumber(streamSeason[0]) {
		showParams.Season = streamSeason[0]
	}

	return showParams
}

func isNonNegativeNumber(value string) bool {
	if value == "" {
		return false
	}
	number, err := strconv.Atoi(value)
	return err == nil && number >= 0
}

func writeShowTorrents(w http.ResponseWriter, results []torrentsTypes.ShowTorrent) {
	if len(results) > 0 {
		io.WriteString(w, showTorrentsList(results))
	} else {
		http.Error(w, noShowTorrentsFound(), http.StatusNotFound)
	}
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
