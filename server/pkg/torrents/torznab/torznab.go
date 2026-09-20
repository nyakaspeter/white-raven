package torznab

import (
	"crypto/tls"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nyakaspeter/white-raven/server/internal/settings"
	"github.com/nyakaspeter/white-raven/server/pkg/torrents/types"
	"github.com/nyakaspeter/white-raven/server/pkg/torrents/utils"
)

const maxResponseSize = 16 << 20

var httpClient = &http.Client{
	Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	Timeout:   15 * time.Second,
}

type response struct {
	XMLName xml.Name
	Code    string  `xml:"code,attr"`
	Message string  `xml:"description,attr"`
	Channel channel `xml:"channel"`
}

type channel struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Generator   string `xml:"generator"`
	Items       []item `xml:"item"`
}

type item struct {
	Title           string    `xml:"title"`
	GUID            string    `xml:"guid"`
	Link            string    `xml:"link"`
	JackettIndexer  string    `xml:"jackettindexer"`
	ProwlarrIndexer string    `xml:"prowlarrindexer"`
	Enclosure       enclosure `xml:"enclosure"`
	Attrs           []attr    `xml:"attr"`
}

type enclosure struct {
	URL    string `xml:"url,attr"`
	Length string `xml:"length,attr"`
}

type attr struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type release struct {
	DedupKey string
	Title    string
	Provider string
	Size     string
	Seeders  string
	Peers    string
	InfoHash string
	Magnet   string
	Torrent  string
}

func request(feed settings.TorznabFeed, params url.Values) ([]release, error) {
	endpoint, err := url.Parse(strings.TrimSpace(feed.URL))
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" {
		return nil, errors.New("invalid endpoint URL")
	}

	query := endpoint.Query()
	for key, values := range params {
		query.Del(key)
		for _, value := range values {
			query.Add(key, value)
		}
	}
	if feed.APIKey != "" {
		query.Set("apikey", feed.APIKey)
	}
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxResponseSize {
		return nil, errors.New("response is larger than 16 MB")
	}

	var wire response
	if err := xml.Unmarshal(data, &wire); err != nil {
		return nil, err
	}
	if wire.XMLName.Local == "error" {
		if wire.Message != "" {
			return nil, errors.New(wire.Message)
		}
		return nil, fmt.Errorf("Torznab error %s", wire.Code)
	}
	if wire.XMLName.Local != "rss" {
		return nil, fmt.Errorf("unexpected XML root %q", wire.XMLName.Local)
	}

	base := *endpoint
	base.RawQuery = ""
	base.Fragment = ""
	feedIdentity := base.String()
	results := make([]release, 0, len(wire.Channel.Items))
	for _, entry := range wire.Channel.Items {
		attrs := attributes(entry.Attrs)
		magnet := firstMagnet(attrs["magneturl"], entry.Enclosure.URL, entry.Link, entry.GUID)
		hash := attrs["infohash"]
		if hash == "" && magnet != "" {
			hash = utils.GetInfoHashFromMagnetLink(magnet)
		}
		if magnet == "" && hash != "" {
			magnet = utils.GetMagnetLinkFromInfoHash(hash)
		}

		provider := firstNonEmpty(
			entry.JackettIndexer, entry.ProwlarrIndexer,
			attrs["jackettindexer"], attrs["prowlarrindexer"], attrs["indexer"],
		)
		if provider == "" {
			provider = sourceProvider(wire.Channel)
		}
		size := numericValue(firstNonEmpty(attrs["size"], entry.Enclosure.Length))
		seeders := numericValue(attrs["seeders"])
		peers := numericValue(firstNonEmpty(attrs["peers"], attrs["leechers"]))
		torrent := firstDownloadURL(&base, entry.Enclosure.URL, entry.Link, entry.GUID)
		guid := strings.TrimSpace(entry.GUID)
		dedupKey := ""
		if guid != "" {
			dedupKey = feedIdentity + "\x00" + guid
		}

		results = append(results, release{
			DedupKey: dedupKey, Title: strings.TrimSpace(entry.Title), Provider: provider, Size: size,
			Seeders: seeders, Peers: peers, InfoHash: hash, Magnet: magnet, Torrent: torrent,
		})
	}
	return results, nil
}

func searchAll(params url.Values) []release {
	feeds := append([]settings.TorznabFeed(nil), (*settings.TorznabFeeds)...)
	type result struct {
		releases []release
	}
	results := make(chan result, len(feeds))
	count := 0
	for _, feed := range feeds {
		if strings.TrimSpace(feed.URL) == "" {
			continue
		}
		count++
		go func(feed settings.TorznabFeed) {
			releases, _ := request(feed, params)
			results <- result{releases: releases}
		}(feed)
	}

	var output []release
	for i := 0; i < count; i++ {
		output = append(output, (<-results).releases...)
	}
	return output
}

func GetMovieTorrentsByText(searchText string, ch chan<- []types.MovieTorrent) {
	params := url.Values{"t": {"search"}, "q": {searchText}, "cat": {"2000,2030,2040"}, "extended": {"1"}}
	ch <- movieTorrents(searchAll(params))
}

func GetMovieTorrentsByImdbId(imdb string, ch chan<- []types.MovieTorrent) {
	params := url.Values{"t": {"movie"}, "imdbid": {imdb}, "cat": {"2000,2030,2040"}, "extended": {"1"}}
	ch <- movieTorrents(searchAll(params))
}

func GetShowTorrentsByText(searchText string, season string, episode string, ch chan<- []types.ShowTorrent) {
	params := showParams("tvsearch", season, episode)
	params.Set("q", searchText)
	ch <- showTorrents(searchAll(params), season, episode)
}

func GetShowTorrentsByImdbId(imdb string, season string, episode string, ch chan<- []types.ShowTorrent) {
	params := showParams("tvsearch", season, episode)
	params.Set("imdbid", imdb)
	ch <- showTorrents(searchAll(params), season, episode)
}

func showParams(searchType string, season string, episode string) url.Values {
	params := url.Values{"t": {searchType}, "cat": {"5000,5030,5040"}, "extended": {"1"}}
	if season != "" && season != "0" {
		params.Set("season", season)
	}
	if episode != "" && episode != "0" {
		params.Set("ep", episode)
	}
	return params
}

func movieTorrents(releases []release) []types.MovieTorrent {
	output := make([]types.MovieTorrent, 0, len(releases))
	for _, entry := range releases {
		output = append(output, types.MovieTorrent{
			DedupKey: entry.DedupKey, Hash: entry.InfoHash, Quality: utils.GuessQualityFromString(entry.Title), Size: entry.Size,
			Provider: entry.Provider, Lang: utils.GuessLanguageFromString(entry.Title), Title: entry.Title,
			Seeds: entry.Seeders, Peers: entry.Peers, Magnet: entry.Magnet, Torrent: entry.Torrent,
		})
	}
	return output
}

func showTorrents(releases []release, season string, episode string) []types.ShowTorrent {
	output := make([]types.ShowTorrent, 0, len(releases))
	for _, entry := range releases {
		torrentSeason, torrentEpisode := utils.GuessSeasonEpisodeNumberFromString(entry.Title)
		if !utils.ShowTorrentMatches(entry.Title, torrentSeason, torrentEpisode, season, episode) {
			continue
		}
		output = append(output, types.ShowTorrent{
			DedupKey: entry.DedupKey, Hash: entry.InfoHash, Quality: utils.GuessQualityFromString(entry.Title),
			Season: torrentSeason, Episode: torrentEpisode, Size: entry.Size, Provider: entry.Provider,
			Lang: utils.GuessLanguageFromString(entry.Title), Title: entry.Title, Seeds: entry.Seeders,
			Peers: entry.Peers, Magnet: entry.Magnet, Torrent: entry.Torrent,
		})
	}
	return output
}

func attributes(values []attr) map[string]string {
	result := make(map[string]string, len(values))
	for _, value := range values {
		result[strings.ToLower(value.Name)] = strings.TrimSpace(value.Value)
	}
	return result
}

func firstMagnet(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if strings.HasPrefix(strings.ToLower(value), "magnet:?") {
			return value
		}
	}
	return ""
}

func firstDownloadURL(base *url.URL, values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || strings.HasPrefix(strings.ToLower(value), "magnet:?") {
			continue
		}
		reference, err := url.Parse(value)
		if err == nil && (reference.IsAbs() || strings.HasPrefix(value, "/")) {
			return base.ResolveReference(reference).String()
		}
	}
	return ""
}

func numericValue(value string) string {
	value = strings.TrimSpace(value)
	if _, err := strconv.ParseInt(value, 10, 64); err != nil {
		return "0"
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func sourceProvider(feed channel) string {
	identity := strings.ToLower(strings.Join([]string{feed.Title, feed.Description, feed.Generator}, " "))
	switch {
	case strings.Contains(identity, "prowlarr"):
		return "Prowlarr"
	case strings.Contains(identity, "harbrr"):
		return "Harbrr"
	case strings.Contains(identity, "jackett"),
		strings.EqualFold(strings.TrimSpace(feed.Title), "AggregateSearch"),
		strings.Contains(identity, "configured trackers"):
		return "Jackett"
	default:
		return "Torznab"
	}
}
