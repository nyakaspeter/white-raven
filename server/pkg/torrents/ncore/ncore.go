package ncore

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/nyakaspeter/white-raven/server/internal/settings"
	"github.com/nyakaspeter/white-raven/server/pkg/torrents/session"
	"github.com/nyakaspeter/white-raven/server/pkg/torrents/types"
	"github.com/nyakaspeter/white-raven/server/pkg/torrents/utils"
)

var authenticatedSession = session.NewAuthenticatedSession(session.Options{
	Name:     "nCore",
	Origin:   "https://ncore.pro",
	LoginURL: "https://ncore.pro/login.php",
	LoginValues: func(credentials session.Credentials) url.Values {
		return url.Values{
			"nev":       {credentials.Username},
			"pass":      {credentials.Password},
			"set_lang":  {"hu"},
			"submitted": {"1"},
		}
	},
	IsLoginSuccessful: func(_ *http.Response, redirectURL *url.URL) bool {
		return redirectURL != nil && redirectURL.Path != "/login.php"
	},
	IsLoginRedirect: func(redirectURL *url.URL) bool {
		return redirectURL != nil && redirectURL.Path == "/login.php"
	},
})

func GetMovieTorrentsByImdbId(imdb string, user string, password string, ch chan<- []types.MovieTorrent) {
	credentials := getCredentials(user, password)

	params := url.Values{
		"oldal":              {"1"},
		"tipus":              {"kivalasztottak_kozott"},
		"kivalasztott_tipus": {"xvid_hun,xvid,hd_hun,hd"},
		"mire":               {imdb},
		"miben":              {"imdb"},
		"miszerint":          {"ctime"},
		"hogyan":             {"DESC"},
	}

	link := fmt.Sprintf("https://ncore.pro/torrents.php?%s", params.Encode())

	resp, err := authenticatedSession.Fetch(link, credentials)
	if err != nil {
		ch <- []types.MovieTorrent{}
		return
	}

	defer resp.Body.Close()

	torrents, err := parseMovieTorrents(resp)
	if err != nil {
		ch <- []types.MovieTorrent{}
		return
	}

	ch <- torrents
}

func GetMovieTorrentsByText(searchText string, user string, password string, ch chan<- []types.MovieTorrent) {
	credentials := getCredentials(user, password)

	params := url.Values{
		"oldal":              {"1"},
		"tipus":              {"kivalasztottak_kozott"},
		"kivalasztott_tipus": {"xvid_hun,xvid,hd_hun,hd"},
		"mire":               {searchText},
		"miben":              {"name"},
		"miszerint":          {"ctime"},
		"hogyan":             {"DESC"},
	}

	link := fmt.Sprintf("https://ncore.pro/torrents.php?%s", params.Encode())

	resp, err := authenticatedSession.Fetch(link, credentials)
	if err != nil {
		ch <- []types.MovieTorrent{}
		return
	}

	defer resp.Body.Close()

	torrents, err := parseMovieTorrents(resp)
	if err != nil {
		ch <- []types.MovieTorrent{}
		return
	}

	ch <- torrents
}

func GetShowTorrentsByImdbId(imdb string, season string, episode string, user string, password string, ch chan<- []types.ShowTorrent) {
	credentials := getCredentials(user, password)

	params := url.Values{
		"oldal":              {"1"},
		"tipus":              {"kivalasztottak_kozott"},
		"kivalasztott_tipus": {"xvidser_hun,xvidser,hdser_hun,hdser"},
		"mire":               {imdb},
		"miben":              {"imdb"},
		"miszerint":          {"ctime"},
		"hogyan":             {"DESC"},
	}

	link := fmt.Sprintf("https://ncore.pro/torrents.php?%s", params.Encode())

	resp, err := authenticatedSession.Fetch(link, credentials)
	if err != nil {
		ch <- []types.ShowTorrent{}
		return
	}

	defer resp.Body.Close()

	torrents, err := parseShowTorrents(resp)
	if err != nil {
		ch <- []types.ShowTorrent{}
		return
	}

	matchingTorrents := []types.ShowTorrent{}
	for _, torrent := range torrents {
		if (torrent.Season == season || season == "0") && (torrent.Episode == episode || episode == "0") {
			matchingTorrents = append(matchingTorrents, torrent)
		}
	}

	ch <- matchingTorrents
}

func GetShowTorrentsByText(searchText string, season string, episode string, user string, password string, ch chan<- []types.ShowTorrent) {
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

	credentials := getCredentials(user, password)

	params := url.Values{
		"oldal":              {"1"},
		"tipus":              {"kivalasztottak_kozott"},
		"kivalasztott_tipus": {"xvidser_hun,xvidser,hdser_hun,hdser"},
		"mire":               {query},
		"miben":              {"name"},
		"miszerint":          {"ctime"},
		"hogyan":             {"DESC"},
	}

	link := fmt.Sprintf("https://ncore.pro/torrents.php?%s", params.Encode())

	resp, err := authenticatedSession.Fetch(link, credentials)
	if err != nil {
		ch <- []types.ShowTorrent{}
		return
	}

	defer resp.Body.Close()

	torrents, err := parseShowTorrents(resp)
	if err != nil {
		ch <- []types.ShowTorrent{}
		return
	}

	matchingTorrents := []types.ShowTorrent{}
	for _, torrent := range torrents {
		if (torrent.Season == season || season == "0") && (torrent.Episode == episode || episode == "0") {
			matchingTorrents = append(matchingTorrents, torrent)
		}
	}

	ch <- matchingTorrents
}

func getCredentials(user string, password string) session.Credentials {
	ncoreUser := *settings.NcoreUser
	if user != "" {
		ncoreUser = user
	}

	ncorePassword := *settings.NcorePassword
	if password != "" {
		ncorePassword = password
	}

	return session.Credentials{
		Username: ncoreUser,
		Password: ncorePassword,
	}
}

func parseMovieTorrents(resp *http.Response) ([]types.MovieTorrent, error) {
	torrents := []types.MovieTorrent{}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	downloadKey := getDownloadKey(doc)
	if downloadKey == "" {
		return nil, fmt.Errorf("download key not found")
	}

	doc.Find("div.box_torrent").Each(func(i int, s *goquery.Selection) {
		name, _ := s.Find("div.torrent_txt > a").Attr("title")
		size := parseSize(s.Find("div.box_meret2").Text())
		seeds, _ := strconv.Atoi(s.Find("div.box_s2").Text())
		peers, _ := strconv.Atoi(s.Find("div.box_l2").Text())
		torrentId, _ := s.Next().Next().Attr("id")
		torrent := fmt.Sprintf("https://ncore.pro/torrents.php?action=download&id=%s&key=%s", torrentId, downloadKey)

		if name == "" || torrentId == "" {
			return
		}

		torrents = append(torrents, types.MovieTorrent{
			Title:    name,
			Provider: "NCORE",
			Lang:     utils.GuessLanguageFromString(name),
			Quality:  utils.GuessQualityFromString(name),
			Size:     strconv.FormatInt(size, 10),
			Seeds:    strconv.Itoa(seeds),
			Peers:    strconv.Itoa(peers),
			Torrent:  torrent,
		})
	})

	return torrents, nil
}

func parseShowTorrents(resp *http.Response) ([]types.ShowTorrent, error) {
	torrents := []types.ShowTorrent{}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	downloadKey := getDownloadKey(doc)
	if downloadKey == "" {
		return nil, fmt.Errorf("download key not found")
	}

	doc.Find("div.box_torrent").Each(func(i int, s *goquery.Selection) {
		name, _ := s.Find("div.torrent_txt > a").Attr("title")
		size := parseSize(s.Find("div.box_meret2").Text())
		seeds, _ := strconv.Atoi(s.Find("div.box_s2").Text())
		peers, _ := strconv.Atoi(s.Find("div.box_l2").Text())
		torrentId, _ := s.Next().Next().Attr("id")
		torrent := fmt.Sprintf("https://ncore.pro/torrents.php?action=download&id=%s&key=%s", torrentId, downloadKey)

		if name == "" || torrentId == "" {
			return
		}

		season, episode := utils.GuessSeasonEpisodeNumberFromString(name)

		torrents = append(torrents, types.ShowTorrent{
			Title:    name,
			Provider: "NCORE",
			Lang:     utils.GuessLanguageFromString(name),
			Quality:  utils.GuessQualityFromString(name),
			Size:     strconv.FormatInt(size, 10),
			Seeds:    strconv.Itoa(seeds),
			Peers:    strconv.Itoa(peers),
			Torrent:  torrent,
			Season:   season,
			Episode:  episode,
		})
	})

	return torrents, nil
}

func parseSize(size string) int64 {
	units := map[string]int64{
		"TiB": 1024 * 1024 * 1024 * 1024,
		"GiB": 1024 * 1024 * 1024,
		"MiB": 1024 * 1024,
		"KiB": 1024,
		"B":   1,
	}

	parts := strings.Split(size, " ")
	if len(parts) != 2 {
		return 0
	}

	sizeNum, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0
	}

	unit, ok := units[parts[1]]
	if !ok {
		return 0
	}

	return int64(math.Ceil(sizeNum * float64(unit)))
}

func getDownloadKey(doc *goquery.Document) string {
	rssUrl, exists := doc.Find("link[rel=alternate]").Attr("href")
	if !exists {
		return ""
	}
	parts := strings.Split(rssUrl, "=")
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}
