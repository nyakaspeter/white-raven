package mediainfo

import (
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/nyakaspeter/white-raven/server/pkg/mediainfo/cinemeta"
	"github.com/nyakaspeter/white-raven/server/pkg/mediainfo/types"
)

const streamReferenceTTL = 10 * time.Minute

type SeasonEpisodes struct {
	Number   int
	Episodes []types.Episode
}

type streamReferenceCacheEntry struct {
	reference *types.StreamReference
	expires   time.Time
}

var streamReferenceCache sync.Map

var titleMarkPattern = regexp.MustCompile(`\p{M}`)

func normalizeTitle(value string) string {
	decomposed := titleMarkPattern.ReplaceAllString(norm.NFKD.String(value), "")
	var builder strings.Builder
	for _, runeValue := range strings.ToLower(decomposed) {
		if unicode.IsLetter(runeValue) || unicode.IsNumber(runeValue) {
			builder.WriteRune(runeValue)
		}
	}
	return builder.String()
}

func ResolveStreamReferences(name string, seasons []SeasonEpisodes) *types.StreamReference {
	name = strings.TrimSpace(name)
	if name == "" || len(seasons) == 0 {
		return nil
	}

	entry, loaded := streamReferenceCache.LoadOrStore(name, &streamReferenceCacheEntry{
		expires: time.Now().Add(streamReferenceTTL),
	})
	cached := entry.(*streamReferenceCacheEntry)
	if loaded && time.Now().Before(cached.expires) {
		return cached.reference
	}

	streamReferenceCache.Delete(name)
	entry, _ = streamReferenceCache.LoadOrStore(name, &streamReferenceCacheEntry{
		expires: time.Now().Add(streamReferenceTTL),
	})
	cached = entry.(*streamReferenceCacheEntry)

	cached.reference = resolveStreamReferences(name, seasons)
	return cached.reference
}

type seasonMatch struct {
	imdbId string
	season int
}

func resolveStreamReferences(name string, seasons []SeasonEpisodes) *types.StreamReference {
	candidates, err := cinemeta.SearchSeries(name)
	if err != nil || len(candidates) == 0 {
		return nil
	}

	videos := fetchCandidateVideos(candidates)
	if len(videos) == 0 {
		return nil
	}

	unique := make(map[int]seasonMatch, len(seasons))
	for _, season := range seasons {
		if season.Number <= 0 || !seasonEpisodesComplete(season.Episodes) {
			continue
		}

		perCandidate := make(map[string]int)
		for candidate, candidateVideos := range videos {
			for _, candidateSeason := range candidateSeasons(candidateVideos) {
				if seasonMatchesEpisodes(candidateVideos, candidateSeason, season.Episodes) {
					perCandidate[candidate] = candidateSeason
				}
			}
		}
		if len(perCandidate) != 1 {
			continue
		}
		for imdbId, candidateSeason := range perCandidate {
			unique[season.Number] = seasonMatch{imdbId: imdbId, season: candidateSeason}
		}
	}
	if len(unique) == 0 {
		return nil
	}

	resolved := make(map[string]int, len(unique))
	imdbId := ""
	for seasonNumber, match := range unique {
		if imdbId == "" {
			imdbId = match.imdbId
		} else if match.imdbId != imdbId {
			return nil
		}
		resolved[strconv.Itoa(seasonNumber)] = match.season
	}

	return &types.StreamReference{
		ImdbId:  imdbId,
		Seasons: resolved,
	}
}

func fetchCandidateVideos(candidates []string) map[string][]cinemeta.Video {
	results := make(map[string][]cinemeta.Video, len(candidates))
	type fetchResult struct {
		id     string
		videos []cinemeta.Video
	}
	done := make(chan fetchResult, len(candidates))

	for _, candidate := range candidates {
		go func(id string) {
			videos, err := cinemeta.GetSeriesVideos(id)
			if err != nil {
				videos = nil
			}
			done <- fetchResult{id: id, videos: videos}
		}(candidate)
	}

	for i := 0; i < len(candidates); i++ {
		result := <-done
		if result.videos != nil {
			results[result.id] = result.videos
		}
	}
	return results
}

func seasonEpisodesComplete(episodes []types.Episode) bool {
	if len(episodes) < 2 {
		return false
	}
	for _, episode := range episodes {
		if episode.Title == "" || episode.AirDate == "" {
			return false
		}
	}
	return true
}

func candidateSeasons(videos []cinemeta.Video) []int {
	seen := make(map[int]struct{})
	output := []int{}
	for _, video := range videos {
		if video.Season <= 0 {
			continue
		}
		if _, ok := seen[video.Season]; ok {
			continue
		}
		seen[video.Season] = struct{}{}
		output = append(output, video.Season)
	}
	return output
}

func seasonMatchesEpisodes(videos []cinemeta.Video, candidateSeason int, episodes []types.Episode) bool {
	for _, episode := range episodes {
		found := false
		for _, video := range videos {
			if video.Season != candidateSeason || video.Episode != episode.EpisodeNumber {
				continue
			}
			if len(video.Released) < 10 || video.Released[:10] != episode.AirDate {
				continue
			}
			if normalizeTitle(video.Name) == normalizeTitle(episode.Title) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
