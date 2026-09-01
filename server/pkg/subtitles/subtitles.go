package subtitles

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	subs "github.com/martinlindhe/subtitles"
	"github.com/nyakaspeter/white-raven/server/internal/settings"
	"github.com/nyakaspeter/white-raven/server/pkg/subtitles/types"
	"github.com/nyakaspeter/white-raven/server/pkg/utils"
)

const (
	openSubtitlesDefaultBaseURL = "https://api.opensubtitles.com/api/v1"
	openSubtitlesUserAgent      = "White Raven Server"
)

var openSubtitlesHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
}

var openSubtitlesSession = struct {
	sync.Mutex
	token         string
	baseURL       string
	expiresAt     time.Time
	loginRejected bool
}{
	baseURL: openSubtitlesDefaultBaseURL,
}

type openSubtitlesAPIError struct {
	StatusCode int
	Message    string
}

func (e *openSubtitlesAPIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("OpenSubtitles API returned HTTP %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("OpenSubtitles API returned HTTP %d", e.StatusCode)
}

type openSubtitlesErrorResponse struct {
	Message string `json:"message"`
}

type openSubtitlesLoginResponse struct {
	BaseURL string `json:"base_url"`
	Token   string `json:"token"`
	Status  int    `json:"status"`
}

type openSubtitlesSearchResponse struct {
	Data []openSubtitlesSubtitle `json:"data"`
}

type openSubtitlesSubtitle struct {
	Type       string                          `json:"type"`
	Attributes openSubtitlesSubtitleAttributes `json:"attributes"`
}

type openSubtitlesSubtitleAttributes struct {
	Language string                  `json:"language"`
	Release  string                  `json:"release"`
	Files    []openSubtitlesFileInfo `json:"files"`
}

type openSubtitlesFileInfo struct {
	FileID   int64  `json:"file_id"`
	FileName string `json:"file_name"`
}

type openSubtitlesDownloadResponse struct {
	Link         string `json:"link"`
	FileName     string `json:"file_name"`
	Requests     int    `json:"requests"`
	Remaining    int    `json:"remaining"`
	Message      string `json:"message"`
	ResetTime    string `json:"reset_time"`
	ResetTimeUTC string `json:"reset_time_utc"`
}

func GetSubtitles(movie types.MediaParams, languages []string) []types.SubtitleFile {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	params := url.Values{}
	params.Set("type", "movie")
	setLanguages(params, languages)

	if imdbID := normalizeIMDBID(movie.ImdbId); imdbID != "" {
		params.Set("imdb_id", imdbID)
	}
	if movie.Title != "" {
		params.Set("query", movie.Title)
	}
	if movie.FileHash != "" {
		params.Set("moviehash", movie.FileHash)
	}

	items, err := searchOpenSubtitles(ctx, params)
	if err != nil {
		log.Println("OpenSubtitles movie search failed:", err)
		return []types.SubtitleFile{}
	}
	if len(items) == 0 {
		return []types.SubtitleFile{}
	}

	preferredLanguage := firstPreferredLanguage(languages)
	return subtitleFilesList(items, preferredLanguage)
}

func GetSubtitlesForEpisode(show types.MediaParams, episode types.EpisodeParams, languages []string) []types.SubtitleFile {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	params := url.Values{}
	params.Set("type", "episode")
	params.Set("season_number", strconv.FormatInt(episode.Season, 10))
	params.Set("episode_number", strconv.FormatInt(episode.Episode, 10))
	setLanguages(params, languages)

	if imdbID := normalizeIMDBID(show.ImdbId); imdbID != "" {
		params.Set("parent_imdb_id", imdbID)
	} else if show.Title != "" {
		params.Set("query", show.Title)
	}
	if show.FileHash != "" {
		params.Set("moviehash", show.FileHash)
	}

	items, err := searchOpenSubtitles(ctx, params)
	if err != nil {
		log.Println("OpenSubtitles episode search failed:", err)
		return []types.SubtitleFile{}
	}
	if len(items) == 0 {
		return []types.SubtitleFile{}
	}

	preferredLanguage := firstPreferredLanguage(languages)
	return subtitleFilesList(items, preferredLanguage)
}

func GetSubtitleContents(params types.SubtitleParams) types.SubtitleContents {
	fileID, err := strconv.ParseInt(params.FileId, 10, 64)
	if err != nil {
		log.Println("Invalid OpenSubtitles file ID:", err)
		return types.SubtitleContents{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	download, err := getOpenSubtitlesDownload(ctx, fileID)
	if err != nil {
		log.Println("OpenSubtitles download-link request failed:", err)
		return types.SubtitleContents{}
	}
	if download.Link == "" {
		log.Println("OpenSubtitles returned an empty subtitle download link")
		return types.SubtitleContents{}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, download.Link, nil)
	if err != nil {
		log.Println("Failed to create subtitle download request:", err)
		return types.SubtitleContents{}
	}
	req.Header.Set("User-Agent", openSubtitlesUserAgent)

	resp, err := openSubtitlesHTTPClient.Do(req)
	if err != nil {
		log.Println("Failed to download subtitle file:", err)
		return types.SubtitleContents{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Subtitle file download returned HTTP %d\n", resp.StatusCode)
		return types.SubtitleContents{}
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Failed to read subtitle file:", err)
		return types.SubtitleContents{}
	}

	subtitle, err := subs.NewFromSRT(string(bodyBytes))
	if err != nil {
		log.Println("Failed to parse downloaded subtitle as SRT:", err)
		return types.SubtitleContents{}
	}

	contents := types.SubtitleContents{}

	switch params.TargetType {
	case "srt":
		contents.Text = subtitle.RemoveAds().AsSRT()
		contents.ContentType = "text/plain; charset=utf-8"
		contents.ContentDisposition = "filename=subtitle.srt"
	case "vtt":
		contents.Text = subtitle.RemoveAds().AsVTT()
		contents.ContentType = "text/vtt; charset=utf-8"
		contents.ContentDisposition = "filename=subtitle.vtt"
	default:
		return types.SubtitleContents{}
	}

	return contents
}

func searchOpenSubtitles(ctx context.Context, params url.Values) ([]openSubtitlesSubtitle, error) {
	if err := requireOpenSubtitlesAPIKey(); err != nil {
		return nil, err
	}

	var response openSubtitlesSearchResponse
	requestURL := openSubtitlesDefaultBaseURL + "/subtitles?" + params.Encode()

	if err := doOpenSubtitlesJSONRequest(
		ctx,
		http.MethodGet,
		requestURL,
		nil,
		"",
		&response,
	); err != nil {
		return nil, err
	}

	return response.Data, nil
}

func getOpenSubtitlesDownload(ctx context.Context, fileID int64) (openSubtitlesDownloadResponse, error) {
	if err := requireOpenSubtitlesAPIKey(); err != nil {
		return openSubtitlesDownloadResponse{}, err
	}

	body := map[string]interface{}{
		"file_id":    fileID,
		"sub_format": "srt",
	}

	baseURL := openSubtitlesDefaultBaseURL
	token := ""

	// User login is optional. We only log in when an actual subtitle
	// download is requested; searches never waste a login request.
	if hasOpenSubtitlesCredentials() {
		authBaseURL, authToken, err := getOpenSubtitlesSession(ctx)
		if err != nil {
			log.Println(
				"OpenSubtitles login failed; trying anonymous subtitle download:",
				err,
			)
		} else {
			baseURL = authBaseURL
			token = authToken
		}
	} else if hasPartialOpenSubtitlesCredentials() {
		log.Println(
			"OpenSubtitles authentication ignored because both -osuser and -ospassword must be supplied",
		)
	}

	var response openSubtitlesDownloadResponse

	err := doOpenSubtitlesJSONRequest(
		ctx,
		http.MethodPost,
		baseURL+"/download",
		body,
		token,
		&response,
	)

	if !isOpenSubtitlesUnauthorized(err) || token == "" {
		return response, err
	}

	// The cached token may have expired or been invalidated earlier
	// than expected. Login once more and retry.
	invalidateOpenSubtitlesSession()

	authBaseURL, authToken, loginErr := getOpenSubtitlesSession(ctx)
	if loginErr != nil {
		return openSubtitlesDownloadResponse{}, err
	}

	response = openSubtitlesDownloadResponse{}

	err = doOpenSubtitlesJSONRequest(
		ctx,
		http.MethodPost,
		authBaseURL+"/download",
		body,
		authToken,
		&response,
	)

	return response, err
}

func requireOpenSubtitlesAPIKey() error {
	if strings.TrimSpace(*settings.OpenSubtitlesKey) == "" {
		return fmt.Errorf(
			"OpenSubtitles API key is not configured; start the server with -osapikey=YOUR_KEY",
		)
	}
	return nil
}

func getOpenSubtitlesSession(ctx context.Context) (string, string, error) {
	openSubtitlesSession.Lock()
	defer openSubtitlesSession.Unlock()

	if openSubtitlesSession.loginRejected {
		return "", "", fmt.Errorf(
			"OpenSubtitles rejected the configured username/password earlier in this process",
		)
	}

	if openSubtitlesSession.token != "" &&
		time.Now().Before(openSubtitlesSession.expiresAt) {
		return openSubtitlesSession.baseURL, openSubtitlesSession.token, nil
	}

	requestBody := map[string]string{
		"username": *settings.OpenSubtitlesUser,
		"password": *settings.OpenSubtitlesPassword,
	}

	var response openSubtitlesLoginResponse

	if err := doOpenSubtitlesJSONRequest(
		ctx,
		http.MethodPost,
		openSubtitlesDefaultBaseURL+"/login",
		requestBody,
		"",
		&response,
	); err != nil {
		// OpenSubtitles specifically recommends not repeatedly attempting
		// login with credentials that returned HTTP 401.
		if apiError, ok := err.(*openSubtitlesAPIError); ok &&
			apiError.StatusCode == http.StatusUnauthorized {
			openSubtitlesSession.loginRejected = true
		}

		return "", "", err
	}

	if response.Token == "" {
		return "", "", fmt.Errorf(
			"OpenSubtitles login succeeded without returning a token",
		)
	}

	openSubtitlesSession.token = response.Token
	openSubtitlesSession.baseURL =
		normalizeOpenSubtitlesBaseURL(response.BaseURL)

	// OpenSubtitles currently documents tokens as valid for 12 hours.
	// Refresh slightly early instead of waiting for exact expiry.
	openSubtitlesSession.expiresAt =
		time.Now().Add(11 * time.Hour)

	return openSubtitlesSession.baseURL,
		openSubtitlesSession.token,
		nil
}

func invalidateOpenSubtitlesSession() {
	openSubtitlesSession.Lock()
	defer openSubtitlesSession.Unlock()

	openSubtitlesSession.token = ""
	openSubtitlesSession.baseURL = openSubtitlesDefaultBaseURL
	openSubtitlesSession.expiresAt = time.Time{}
	openSubtitlesSession.loginRejected = false
}

func doOpenSubtitlesJSONRequest(
	ctx context.Context,
	method string,
	requestURL string,
	body interface{},
	token string,
	destination interface{},
) error {
	var requestBody io.Reader

	if body != nil {
		encodedBody, err := json.Marshal(body)
		if err != nil {
			return err
		}

		requestBody = bytes.NewReader(encodedBody)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		method,
		requestURL,
		requestBody,
	)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set(
		"Api-Key",
		strings.TrimSpace(*settings.OpenSubtitlesKey),
	)
	req.Header.Set("User-Agent", openSubtitlesUserAgent)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := openSubtitlesHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices {

		apiError := &openSubtitlesAPIError{
			StatusCode: resp.StatusCode,
		}

		var errorResponse openSubtitlesErrorResponse

		if json.Unmarshal(responseBody, &errorResponse) == nil {
			apiError.Message = errorResponse.Message
		}

		if apiError.Message == "" {
			apiError.Message =
				strings.TrimSpace(string(responseBody))
		}

		return apiError
	}

	if destination == nil || len(responseBody) == 0 {
		return nil
	}

	if err := json.Unmarshal(responseBody, destination); err != nil {
		return fmt.Errorf(
			"failed to decode OpenSubtitles response: %w",
			err,
		)
	}

	return nil
}

func hasOpenSubtitlesCredentials() bool {
	return strings.TrimSpace(*settings.OpenSubtitlesUser) != "" &&
		strings.TrimSpace(*settings.OpenSubtitlesPassword) != ""
}

func hasPartialOpenSubtitlesCredentials() bool {
	userSet := strings.TrimSpace(*settings.OpenSubtitlesUser) != ""
	passwordSet := strings.TrimSpace(*settings.OpenSubtitlesPassword) != ""

	return userSet != passwordSet
}

func isOpenSubtitlesUnauthorized(err error) bool {
	apiError, ok := err.(*openSubtitlesAPIError)

	return ok &&
		(apiError.StatusCode == http.StatusUnauthorized ||
			apiError.StatusCode == http.StatusForbidden)
}

func normalizeOpenSubtitlesBaseURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)

	if baseURL == "" {
		return openSubtitlesDefaultBaseURL
	}

	if !strings.HasPrefix(baseURL, "http://") &&
		!strings.HasPrefix(baseURL, "https://") {
		baseURL = "https://" + baseURL
	}

	baseURL = strings.TrimRight(baseURL, "/")

	if !strings.HasSuffix(baseURL, "/api/v1") {
		baseURL += "/api/v1"
	}

	return baseURL
}

func normalizeIMDBID(imdbID string) string {
	imdbID = strings.TrimSpace(strings.ToLower(imdbID))
	imdbID = strings.TrimPrefix(imdbID, "tt")
	imdbID = strings.TrimLeft(imdbID, "0")

	return imdbID
}

func setLanguages(params url.Values, languages []string) {
	normalized := normalizeLanguages(languages)

	if len(normalized) == 0 {
		normalized = []string{"en"}
	}

	params.Set("languages", strings.Join(normalized, ","))
}

func normalizeLanguages(languages []string) []string {
	result := make([]string, 0, len(languages))
	seen := make(map[string]bool)

	for _, language := range languages {
		language = strings.ToLower(strings.TrimSpace(language))

		switch language {
		case "", "auto":
			continue

		case "pb":
			// White Raven's legacy code uses "pb" for Brazilian Portuguese.
			language = "pt-br"

		case "pt":
			// Current OpenSubtitles code for European Portuguese.
			language = "pt-pt"
		}

		if !seen[language] {
			seen[language] = true
			result = append(result, language)
		}
	}

	return result
}

func firstPreferredLanguage(languages []string) string {
	normalized := normalizeLanguages(languages)

	if len(normalized) == 0 {
		return "en"
	}

	return normalized[0]
}

func subtitleFilesList(
	subtitles []openSubtitlesSubtitle,
	firstLanguage string,
) []types.SubtitleFile {
	sort.SliceStable(subtitles, func(i, j int) bool {
		iPreferred :=
			subtitles[i].Attributes.Language == firstLanguage

		jPreferred :=
			subtitles[j].Attributes.Language == firstLanguage

		return iPreferred && !jPreferred
	})

	results := make([]types.SubtitleFile, 0)

	for _, sub := range subtitles {
		if sub.Type != "subtitle" {
			continue
		}

		for _, file := range sub.Attributes.Files {
			if file.FileID == 0 {
				continue
			}

			baseLink :=
				"http://" +
					utils.GetLocalIP() +
					":" +
					strconv.Itoa(*settings.Port) +
					"/subtitle/" +
					strconv.FormatInt(file.FileID, 10)

			results = append(results, types.SubtitleFile{
				Lang:         sub.Attributes.Language,
				SubtitleName: file.FileName,
				ReleaseName:  sub.Attributes.Release,
				SubData:      baseLink + "/srt",
				VttData:      baseLink + "/vtt",
			})
		}
	}

	return results
}
