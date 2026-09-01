package torrentio

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nyakaspeter/white-raven/server/pkg/torrents/types"
	"github.com/nyakaspeter/white-raven/server/pkg/torrents/utils"
)

const baseURL = "https://torrentio.strem.fun"

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"

var client = &http.Client{Timeout: 15 * time.Second}

type streamResponse struct {
	Streams []stream `json:"streams"`
}

type stream struct {
	Name          string   `json:"name"`
	Title         string   `json:"title"`
	InfoHash      string   `json:"infoHash"`
	Sources       []string `json:"sources"`
	BehaviorHints struct {
		Filename string `json:"filename"`
	} `json:"behaviorHints"`
}

func GetMovieTorrentsByImdbId(imdb string, ch chan<- []types.MovieTorrent) {
	streams, err := search("movie", imdb)
	if err != nil {
		ch <- []types.MovieTorrent{}
		return
	}

	output := make([]types.MovieTorrent, 0, len(streams))
	for _, item := range streams {
		if torrent, ok := movieTorrent(item); ok {
			output = append(output, torrent)
		}
	}

	ch <- output
}

func GetShowTorrentsByImdbId(imdb string, season string, episode string, ch chan<- []types.ShowTorrent) {
	streams, err := search("series", imdb+":"+season+":"+episode)
	if err != nil {
		ch <- []types.ShowTorrent{}
		return
	}

	output := make([]types.ShowTorrent, 0, len(streams))
	for _, item := range streams {
		if torrent, ok := showTorrent(item, season, episode); ok {
			output = append(output, torrent)
		}
	}

	ch <- output
}

func search(category string, id string) ([]stream, error) {
	requestURL := fmt.Sprintf(
		"%s/stream/%s/%s.json",
		baseURL,
		category,
		url.PathEscape(id),
	)

	request, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", userAgent)

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, response.Body)
		return nil, fmt.Errorf("torrentio returned %s", response.Status)
	}

	var result streamResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Streams, nil
}

func movieTorrent(item stream) (types.MovieTorrent, bool) {
	common, ok := torrentFields(item)
	if !ok {
		return types.MovieTorrent{}, false
	}

	return types.MovieTorrent{
		Hash:     common.Hash,
		Quality:  common.Quality,
		Size:     common.Size,
		Provider: common.Provider,
		Lang:     common.Lang,
		Title:    common.Title,
		Seeds:    common.Seeds,
		Peers:    "0",
		Magnet:   common.Magnet,
	}, true
}

func showTorrent(item stream, season string, episode string) (types.ShowTorrent, bool) {
	common, ok := torrentFields(item)
	if !ok {
		return types.ShowTorrent{}, false
	}

	return types.ShowTorrent{
		Hash:     common.Hash,
		Quality:  common.Quality,
		Season:   season,
		Episode:  episode,
		Size:     common.Size,
		Provider: common.Provider,
		Lang:     common.Lang,
		Title:    common.Title,
		Seeds:    common.Seeds,
		Peers:    "0",
		Magnet:   common.Magnet,
	}, true
}

type commonTorrentFields struct {
	Hash     string
	Quality  string
	Size     string
	Provider string
	Lang     string
	Title    string
	Seeds    string
	Magnet   string
}

func torrentFields(item stream) (commonTorrentFields, bool) {
	hash := strings.ToLower(strings.TrimSpace(item.InfoHash))
	if hash == "" {
		return commonTorrentFields{}, false
	}

	title := firstLine(item.Title)
	title = strings.TrimSpace(strings.ReplaceAll(title, "⭐", ""))
	if title == "" {
		title = strings.TrimSpace(item.BehaviorHints.Filename)
	}
	if title == "" {
		title = firstLine(item.Name)
	}

	description := strings.Join([]string{item.Name, item.Title, item.BehaviorHints.Filename}, " ")

	return commonTorrentFields{
		Hash:     hash,
		Quality:  utils.GuessQualityFromString(description),
		Size:     parseSize(item.Title),
		Provider: parseProvider(item.Title),
		Lang:     utils.GuessLanguageFromString(description),
		Title:    title,
		Seeds:    parseSeeds(item.Title),
		Magnet:   magnetLink(hash, item.Sources),
	}, true
}

func firstLine(value string) string {
	line, _, _ := strings.Cut(value, "\n")
	return strings.TrimSpace(line)
}

func parseProvider(title string) string {
	_, suffix, found := strings.Cut(title, "⚙️ ")
	if !found {
		return "Torrentio"
	}

	provider := firstLine(suffix)
	if provider == "" {
		return "Torrentio"
	}
	return provider
}

var sizePattern = regexp.MustCompile(`(?i)💾\s*([0-9]+(?:\.[0-9]+)?)\s*(TB|GB|MB|KB|B)`)

func parseSize(title string) string {
	match := sizePattern.FindStringSubmatch(title)
	if len(match) != 3 {
		return "0"
	}

	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return "0"
	}

	multipliers := map[string]float64{
		"TB": 1024 * 1024 * 1024 * 1024,
		"GB": 1024 * 1024 * 1024,
		"MB": 1024 * 1024,
		"KB": 1024,
		"B":  1,
	}

	return strconv.FormatInt(int64(math.Ceil(value*multipliers[strings.ToUpper(match[2])])), 10)
}

var seedsPattern = regexp.MustCompile(`👤\s*([0-9]+)`)

func parseSeeds(title string) string {
	match := seedsPattern.FindStringSubmatch(title)
	if len(match) != 2 {
		return "0"
	}
	return match[1]
}

func magnetLink(hash string, sources []string) string {
	magnet := utils.GetMagnetLinkFromInfoHash(hash)
	seen := make(map[string]struct{})

	for _, source := range sources {
		tracker := strings.TrimSpace(source)
		if strings.HasPrefix(tracker, "tracker:") {
			tracker = strings.TrimPrefix(tracker, "tracker:")
		} else if !strings.HasPrefix(tracker, "http://") &&
			!strings.HasPrefix(tracker, "https://") &&
			!strings.HasPrefix(tracker, "udp://") &&
			!strings.HasPrefix(tracker, "wss://") {
			continue
		}

		if tracker == "" {
			continue
		}
		if _, ok := seen[tracker]; ok {
			continue
		}
		seen[tracker] = struct{}{}
		magnet += "&tr=" + url.QueryEscape(tracker)
	}

	return magnet
}
