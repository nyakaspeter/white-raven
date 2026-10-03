package mediainfo

import (
	"sort"
	"sync"

	"github.com/nyakaspeter/white-raven/server/pkg/mediainfo/tmdb"
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

type ShowEpisodeData struct {
	Episodes        []types.ShowEpisode
	StreamReference *types.StreamReference
}

func GetShowEpisodesByTmdb(tmdbId int) (ShowEpisodeData, error) {
	info, err := tmdb.GetShowInfo(tmdbId, "en")
	if err != nil {
		return ShowEpisodeData{}, err
	}

	seasons := append([]types.Season(nil), info.Seasons...)
	sort.Slice(seasons, func(i, j int) bool {
		return seasons[i].SeasonNumber < seasons[j].SeasonNumber
	})

	targets := []types.Season{}
	for _, season := range seasons {
		if season.EpisodeCount > 0 && season.SeasonNumber >= 0 {
			targets = append(targets, season)
		}
	}

	const maxConcurrentSeasons = 8
	seasonInfos := make([]types.SeasonInfo, len(targets))
	semaphore := make(chan struct{}, maxConcurrentSeasons)
	var seasonGroup sync.WaitGroup
	for i, season := range targets {
		seasonGroup.Add(1)
		semaphore <- struct{}{}
		go func(index int, seasonNumber int) {
			defer seasonGroup.Done()
			defer func() { <-semaphore }()
			seasonInfos[index], _ = tmdb.GetShowSeason(tmdbId, seasonNumber, "en")
		}(i, season.SeasonNumber)
	}
	seasonGroup.Wait()

	data := ShowEpisodeData{Episodes: []types.ShowEpisode{}}
	seasonInputs := []SeasonEpisodes{}

	for i, seasonInfo := range seasonInfos {
		for _, episode := range seasonInfo.Episodes {
			data.Episodes = append(data.Episodes, types.ShowEpisode{
				Id:             episode.Id,
				Title:          episode.Title,
				SeasonNumber:   episode.SeasonNumber,
				EpisodeNumber:  episode.EpisodeNumber,
				AirDate:        episode.AirDate,
				RuntimeMinutes: episode.RuntimeMinutes,
				Description:    episode.Description,
				Images:         episodeImages(episode.StillPath),
			})
		}

		if info.ExternalIds.ImdbId == "" && targets[i].SeasonNumber > 0 {
			seasonInputs = append(seasonInputs, SeasonEpisodes{
				Number:   targets[i].SeasonNumber,
				Episodes: seasonInfo.Episodes,
			})
		}
	}

	if len(seasonInputs) > 0 {
		name := info.OriginalTitle
		if name == "" {
			name = info.Title
		}
		data.StreamReference = ResolveStreamReferences(name, seasonInputs)
	}

	return data, nil
}

func episodeImages(stillPath string) types.EpisodeImages {
	if stillPath == "" {
		return types.EpisodeImages{}
	}
	return types.EpisodeImages{
		MediumImageUrl:   "https://image.tmdb.org/t/p/w500" + stillPath,
		OriginalImageUrl: "https://image.tmdb.org/t/p/original" + stillPath,
	}
}
