package mediainfo

import (
	"strconv"
	"strings"

	"github.com/nyakaspeter/white-raven/server/pkg/mediainfo/tmdb"
	"github.com/nyakaspeter/white-raven/server/pkg/mediainfo/tvmaze"
	"github.com/nyakaspeter/white-raven/server/pkg/mediainfo/types"
)

func DiscoverMovies(params types.MovieDiscoverParams, language string, page int) types.MovieResults {
	output, err := tmdb.DiscoverMovies(params, language, page)
	if err != nil {
		return types.MovieResults{}
	}

	return output
}

func DiscoverShows(params types.ShowDiscoverParams, language string, page int) types.ShowResults {
	output, err := tmdb.DiscoverShows(params, language, page)
	if err != nil {
		return types.ShowResults{}
	}

	return output
}

func SearchMovies(title string, language string, page int) types.MovieResults {
	output, err := tmdb.SearchMovies(title, language, page)
	if err != nil {
		return types.MovieResults{}
	}

	return output
}

func SearchShows(title string, language string, page int) types.ShowResults {
	output, err := tmdb.SearchShows(title, language, page)
	if err != nil {
		return types.ShowResults{}
	}

	return output
}

func GetMovieInfo(tmdbId int, language string) types.MovieInfo {
	output, err := tmdb.GetMovieInfo(tmdbId, language)
	if err != nil {
		return types.MovieInfo{}
	}

	return output
}

func GetShowInfo(tmdbId int, language string) types.ShowInfo {
	output, err := tmdb.GetShowInfo(tmdbId, language)
	if err != nil {
		return types.ShowInfo{}
	}

	return output
}

func GetShowSeason(tmdbId int, seasonNumber int, language string) types.SeasonInfo {
	output, err := tmdb.GetShowSeason(tmdbId, seasonNumber, language)
	if err != nil {
		return types.SeasonInfo{}
	}

	return output
}

func GetShowEpisodes(showId types.ShowIds) []types.TvMazeEpisode {
	output, err := tvmaze.GetEpisodes(showId)
	if err != nil {
		return []types.TvMazeEpisode{}
	}

	return output
}

func GetShowEpisodesByTmdb(tmdbId int) []types.TvMazeEpisode {
	info, err := tmdb.GetShowInfo(tmdbId, "en")
	if err != nil || info.Id == 0 {
		return []types.TvMazeEpisode{}
	}

	if info.ExternalIds.TvdbId != 0 || info.ExternalIds.ImdbId != "" {
		showIds := types.ShowIds{ImdbId: info.ExternalIds.ImdbId}
		if info.ExternalIds.TvdbId != 0 {
			showIds.TvdbId = strconv.Itoa(info.ExternalIds.TvdbId)
		}
		return GetShowEpisodes(showIds)
	}

	year := parseYear(info.FirstAirDate)
	bestId := ""
	bestScore := 0.0
	for _, title := range uniqueTitles(info.Title, info.OriginalTitle) {
		results, err := tvmaze.SearchShows(title)
		if err != nil {
			continue
		}
		for _, result := range results {
			if !premiereYearMatches(result.Show.Premiered, year) {
				continue
			}
			if result.Score > bestScore {
				bestScore = result.Score
				bestId = strconv.Itoa(result.Show.Id)
			}
		}
	}

	if bestId == "" || bestScore < 1.0 {
		return []types.TvMazeEpisode{}
	}

	episodes, err := tvmaze.GetEpisodesByTvMazeId(bestId)
	if err != nil {
		return []types.TvMazeEpisode{}
	}

	return episodes
}

func uniqueTitles(titles ...string) []string {
	seen := make(map[string]struct{})
	output := make([]string, 0, len(titles))
	for _, title := range titles {
		title = strings.TrimSpace(title)
		if title == "" {
			continue
		}
		if _, ok := seen[title]; ok {
			continue
		}
		seen[title] = struct{}{}
		output = append(output, title)
	}
	return output
}

func parseYear(date string) int {
	if len(date) < 4 {
		return 0
	}
	year, err := strconv.Atoi(date[:4])
	if err != nil {
		return 0
	}
	return year
}

func premiereYearMatches(premiered string, year int) bool {
	if year == 0 {
		return true
	}
	premiereYear := parseYear(premiered)
	if premiereYear == 0 {
		return true
	}
	diff := premiereYear - year
	if diff < 0 {
		diff = -diff
	}
	return diff <= 1
}
