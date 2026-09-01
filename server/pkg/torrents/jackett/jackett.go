package jackett

import (
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/nyakaspeter/white-raven/server/internal/settings"
	"github.com/nyakaspeter/white-raven/server/pkg/torrents/types"
	"github.com/nyakaspeter/white-raven/server/pkg/torrents/utils"
)

type apiResponse struct {
	TorrentResults []struct {
		Title        string `json:"Title"`
		Tracker      string `json:"Tracker"`
		CategoryDesc string `json:"CategoryDesc"`
		PublishDate  string `json:"PublishDate"`
		Description  string `json:"Description"`
		Imdb         int64  `json:"Imdb"`
		Size         int64  `json:"Size"`
		Seeders      int64  `json:"Seeders"`
		Peers        int64  `json:"Peers"`
		Details      string `json:"Details"`
		Guid         string `json:"Guid"`
		Link         string `json:"Link"`
		MagnetUri    string `json:"MagnetUri"`
		InfoHash     string `json:"InfoHash"`
	} `json:"Results"`
}

var movieCategories = [...]string{"2000", "2030", "2040"}
var showCategories = [...]string{"5000", "5030", "5040"}

func getJackettResponse(apiAddress string, apiKey string, category string, query string) (*apiResponse, error) {
	req, err := http.NewRequest(
		"GET",
		apiAddress+
			"/api/v2.0/indexers/all/results?apikey="+
			url.QueryEscape(apiKey)+
			"&category="+
			url.QueryEscape(category)+
			"&query="+
			url.QueryEscape(query),
		nil,
	)
	if err != nil {
		return nil, err
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	if err != nil {
		return nil, err
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &jackettHTTPError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
		}
	}

	response := &apiResponse{}

	err = json.Unmarshal(body, response)
	if err != nil {
		return nil, err
	}

	return response, nil
}

type jackettHTTPError struct {
	StatusCode int
	Status     string
}

func (e *jackettHTTPError) Error() string {
	return e.Status
}

func getTorrentHash(infoHash string, magnetUri string) string {
	if infoHash != "" {
		return infoHash
	}

	if magnetUri != "" {
		return utils.GetInfoHashFromMagnetLink(magnetUri)
	}

	return ""
}

func GetMovieTorrentsByImdbId(
	imdb string,
	jackettApiAddress string,
	jackettApiKey string,
	ch chan<- []types.MovieTorrent,
) {
	apiAddress := *settings.JackettAddress
	if jackettApiAddress != "" {
		apiAddress = jackettApiAddress
	}

	apiKey := *settings.JackettKey
	if jackettApiKey != "" {
		apiKey = jackettApiKey
	}

	outputMovieData := []types.MovieTorrent{}

	for _, category := range movieCategories {
		response, err := getJackettResponse(
			apiAddress,
			apiKey,
			category,
			imdb,
		)
		if err != nil {
			continue
		}

		if len(response.TorrentResults) == 0 {
			continue
		}

		for _, thistorrent := range response.TorrentResults {
			temp := types.MovieTorrent{
				Hash: getTorrentHash(
					thistorrent.InfoHash,
					thistorrent.MagnetUri,
				),
				Quality:  utils.GuessQualityFromString(thistorrent.Title),
				Size:     strconv.FormatInt(thistorrent.Size, 10),
				Provider: thistorrent.Tracker,
				Lang:     utils.GuessLanguageFromString(thistorrent.Title),
				Title:    thistorrent.Title,
				Seeds:    strconv.FormatInt(thistorrent.Seeders, 10),
				Peers:    strconv.FormatInt(thistorrent.Peers, 10),
				Magnet:   thistorrent.MagnetUri,
				Torrent:  thistorrent.Link,
			}

			outputMovieData = append(outputMovieData, temp)
		}
	}

	ch <- outputMovieData
}

func GetMovieTorrentsByText(
	searchText string,
	jackettApiAddress string,
	jackettApiKey string,
	ch chan<- []types.MovieTorrent,
) {
	apiAddress := *settings.JackettAddress
	if jackettApiAddress != "" {
		apiAddress = jackettApiAddress
	}

	apiKey := *settings.JackettKey
	if jackettApiKey != "" {
		apiKey = jackettApiKey
	}

	outputMovieData := []types.MovieTorrent{}

	for _, category := range movieCategories {
		response, err := getJackettResponse(
			apiAddress,
			apiKey,
			category,
			searchText,
		)
		if err != nil {
			continue
		}

		if len(response.TorrentResults) == 0 {
			continue
		}

		for _, thistorrent := range response.TorrentResults {
			temp := types.MovieTorrent{
				Hash: getTorrentHash(
					thistorrent.InfoHash,
					thistorrent.MagnetUri,
				),
				Quality:  utils.GuessQualityFromString(thistorrent.Title),
				Size:     strconv.FormatInt(thistorrent.Size, 10),
				Provider: thistorrent.Tracker,
				Lang:     utils.GuessLanguageFromString(thistorrent.Title),
				Title:    thistorrent.Title,
				Seeds:    strconv.FormatInt(thistorrent.Seeders, 10),
				Peers:    strconv.FormatInt(thistorrent.Peers, 10),
				Magnet:   thistorrent.MagnetUri,
				Torrent:  thistorrent.Link,
			}

			outputMovieData = append(outputMovieData, temp)
		}
	}

	ch <- outputMovieData
}

func GetShowTorrentsByImdbId(
	imdb string,
	season string,
	episode string,
	jackettApiAddress string,
	jackettApiKey string,
	ch chan<- []types.ShowTorrent,
) {
	apiAddress := *settings.JackettAddress
	if jackettApiAddress != "" {
		apiAddress = jackettApiAddress
	}

	apiKey := *settings.JackettKey
	if jackettApiKey != "" {
		apiKey = jackettApiKey
	}

	outputShowData := []types.ShowTorrent{}

	for _, category := range showCategories {
		response, err := getJackettResponse(
			apiAddress,
			apiKey,
			category,
			imdb,
		)
		if err != nil {
			continue
		}

		if len(response.TorrentResults) == 0 {
			continue
		}

		for _, thistorrent := range response.TorrentResults {
			titleSeason, titleEpisode :=
				utils.GuessSeasonEpisodeNumberFromString(thistorrent.Title)

			if titleSeason != season || titleEpisode != episode {
				continue
			}

			temp := types.ShowTorrent{
				Hash: getTorrentHash(
					thistorrent.InfoHash,
					thistorrent.MagnetUri,
				),
				Quality:  utils.GuessQualityFromString(thistorrent.Title),
				Season:   season,
				Episode:  episode,
				Size:     strconv.FormatInt(thistorrent.Size, 10),
				Provider: thistorrent.Tracker,
				Lang:     utils.GuessLanguageFromString(thistorrent.Title),
				Title:    thistorrent.Title,
				Seeds:    strconv.FormatInt(thistorrent.Seeders, 10),
				Peers:    strconv.FormatInt(thistorrent.Peers, 10),
				Magnet:   thistorrent.MagnetUri,
				Torrent:  thistorrent.Link,
			}

			outputShowData = append(outputShowData, temp)
		}
	}

	ch <- outputShowData
}

func GetShowTorrentsByText(
	searchText string,
	season string,
	episode string,
	jackettApiAddress string,
	jackettApiKey string,
	ch chan<- []types.ShowTorrent,
) {
	apiAddress := *settings.JackettAddress
	if jackettApiAddress != "" {
		apiAddress = jackettApiAddress
	}

	apiKey := *settings.JackettKey
	if jackettApiKey != "" {
		apiKey = jackettApiKey
	}

	query := searchText + " "

	if season != "0" {
		if len(season) == 1 {
			query += "s0" + season
		} else {
			query += "s" + season
		}
	}

	if episode != "0" {
		if len(episode) == 1 {
			query += "e0" + episode
		} else {
			query += "e" + episode
		}
	}

	outputShowData := []types.ShowTorrent{}

	for _, category := range showCategories {
		response, err := getJackettResponse(
			apiAddress,
			apiKey,
			category,
			query,
		)
		if err != nil {
			continue
		}

		if len(response.TorrentResults) == 0 {
			continue
		}

		for _, thistorrent := range response.TorrentResults {
			titleSeason, titleEpisode :=
				utils.GuessSeasonEpisodeNumberFromString(thistorrent.Title)

			temp := types.ShowTorrent{
				Hash: getTorrentHash(
					thistorrent.InfoHash,
					thistorrent.MagnetUri,
				),
				Quality:  utils.GuessQualityFromString(thistorrent.Title),
				Season:   titleSeason,
				Episode:  titleEpisode,
				Size:     strconv.FormatInt(thistorrent.Size, 10),
				Provider: thistorrent.Tracker,
				Lang:     utils.GuessLanguageFromString(thistorrent.Title),
				Title:    thistorrent.Title,
				Seeds:    strconv.FormatInt(thistorrent.Seeders, 10),
				Peers:    strconv.FormatInt(thistorrent.Peers, 10),
				Magnet:   thistorrent.MagnetUri,
				Torrent:  thistorrent.Link,
			}

			outputShowData = append(outputShowData, temp)
		}
	}

	ch <- outputShowData
}
