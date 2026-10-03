package cinemeta

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const baseURL = "https://v3-cinemeta.strem.io"

var imdbIdPattern = regexp.MustCompile(`^tt\d+$`)

var httpClient = &http.Client{
	Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	Timeout:   15 * time.Second,
}

type searchResponse struct {
	Metas []struct {
		ID string `json:"id"`
	} `json:"metas"`
}

type metaResponse struct {
	Meta struct {
		Videos []Video `json:"videos"`
	} `json:"meta"`
}

type Video struct {
	Season   int    `json:"season"`
	Episode  int    `json:"episode"`
	Name     string `json:"name"`
	Released string `json:"released"`
}

func SearchSeries(name string) ([]string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("empty series name")
	}

	endpoint := baseURL + "/catalog/series/top/search=" + url.PathEscape(name) + ".json"
	var data searchResponse
	if err := fetchJSON(endpoint, &data); err != nil {
		return nil, err
	}

	for _, meta := range data.Metas {
		id := strings.TrimSpace(meta.ID)
		if imdbIdPattern.MatchString(id) {
			return []string{id}, nil
		}
	}
	return nil, nil
}

func GetSeriesVideos(imdbId string) ([]Video, error) {
	if !imdbIdPattern.MatchString(imdbId) {
		return nil, fmt.Errorf("invalid IMDb ID %q", imdbId)
	}

	endpoint := baseURL + "/meta/series/" + imdbId + ".json"
	var data metaResponse
	if err := fetchJSON(endpoint, &data); err != nil {
		return nil, err
	}
	return data.Meta.Videos, nil
}

func fetchJSON(endpoint string, target any) error {
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")

	response, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, response.Body)
		return fmt.Errorf("cinemeta returned %s", response.Status)
	}

	return json.NewDecoder(response.Body).Decode(target)
}
