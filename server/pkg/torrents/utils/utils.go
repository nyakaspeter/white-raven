package utils

import (
	"path"
	"regexp"
	"strconv"
	"strings"
)

func GetMagnetLinkFromInfoHash(hash string) string {
	return "magnet:?xt=urn:btih:" + hash
}

func GetInfoHashFromMagnetLink(magnet string) string {
	re := regexp.MustCompile("magnet:\\?xt=urn:btih:([a-zA-Z0-9]*)")
	hash := re.FindAllSubmatch([]byte(magnet), -1)
	if hash == nil {
		return ""
	} else {
		return string(hash[0][1])
	}
}

func GuessQualityFromString(value string) string {
	// Try to decode quality information from string (url, title, filename)
	lowstr := strings.ToLower(value)
	quality := ""
	if strings.Contains(lowstr, "3d") == true {
		quality = "3D"
	} else if strings.Contains(lowstr, "2160p") == true {
		quality = "2160p"
	} else if strings.Contains(lowstr, "1080p") == true {
		quality = "1080p"
	} else if strings.Contains(lowstr, "720p") == true {
		quality = "720p"
	} else if strings.Contains(lowstr, "480p") == true {
		quality = "480p"
	} else if strings.Contains(lowstr, "360p") == true {
		quality = "360p"
	} else {
		quality = ""
	}
	return quality
}

func GuessLanguageFromString(value string) string {
	for _, language := range torrentLanguages {
		if language.pattern.MatchString(value) {
			return language.code
		}
	}

	return "en"
}

type torrentLanguage struct {
	code    string
	pattern *regexp.Regexp
}

// Keep these codes in sync with the widget's languageListText shortcodes.
// Longer release tags and language names avoid false positives from movie
// titles containing words such as "it" or "no".
var torrentLanguages = []torrentLanguage{
	{"hu", languagePattern(`hun|hungarian|magyar`)},
	{"pb", languagePattern(`pt[._ -]?br|ptbr|brazilian(?:[._ -]+portuguese)?`)},
	{"ar", languagePattern(`ara|arabic`)},
	{"bg", languagePattern(`bul|bulgarian`)},
	{"hr", languagePattern(`hrv|cro|croatian`)},
	{"cs", languagePattern(`cze|ces|czech`)},
	{"da", languagePattern(`dan|danish`)},
	{"nl", languagePattern(`dut|nld|dutch`)},
	{"et", languagePattern(`est|estonian`)},
	{"fi", languagePattern(`fin|finnish`)},
	{"fr", languagePattern(`fre|fra|french|truefrench|vff|vfq`)},
	{"de", languagePattern(`ger|deu|german|deutsch`)},
	{"el", languagePattern(`gre|ell|greek`)},
	{"he", languagePattern(`heb|hebrew`)},
	{"id", languagePattern(`ind|indonesian`)},
	{"it", languagePattern(`ita|italian`)},
	{"ko", languagePattern(`kor|korean`)},
	{"lv", languagePattern(`lav|latvian`)},
	{"lt", languagePattern(`lit|lithuanian`)},
	{"no", languagePattern(`nor|norwegian`)},
	{"fa", languagePattern(`per|fas|persian|farsi`)},
	{"pl", languagePattern(`pol|polish`)},
	{"pt", languagePattern(`por|portuguese|pt[._ -]?pt`)},
	{"ro", languagePattern(`rum|ron|romanian`)},
	{"ru", languagePattern(`rus|russian`)},
	{"sr", languagePattern(`srp|serbian`)},
	{"sk", languagePattern(`slo|slk|slovak`)},
	{"es", languagePattern(`spa|esp|spanish|castilian|castellano`)},
	{"sw", languagePattern(`swa|swahili`)},
	{"sv", languagePattern(`swe|swedish`)},
	{"th", languagePattern(`tha|thai`)},
	{"tr", languagePattern(`tur|turkish`)},
	{"ur", languagePattern(`urd|urdu`)},
	{"vi", languagePattern(`vie|vietnamese`)},
	{"en", languagePattern(`eng|english`)},
}

func languagePattern(aliases string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)(?:^|[^\pL\pN])(?:` + aliases + `)(?:$|[^\pL\pN])`)
}

func GuessSeasonEpisodeNumberFromString(value string) (string, string) {
	seasonRegex := regexp.MustCompile(`(s0*\d+)`)
	episodeRegex := regexp.MustCompile(`(e0*\d+)`)

	seasonPrefixRegex := regexp.MustCompile(`(s0*)`)
	episodePrefixRegex := regexp.MustCompile(`(e0*)`)

	season := seasonRegex.FindString(strings.ToLower(value))
	episode := episodeRegex.FindString(strings.ToLower(value))

	season = seasonPrefixRegex.ReplaceAllString(season, "")
	episode = episodePrefixRegex.ReplaceAllString(episode, "")

	return season, episode
}

// ShowTorrentMatches reports whether a show release can contain the requested
// episode. Besides individual episodes, trackers commonly return season packs
// (S01), season ranges (S01-S08), and complete-series packs.
func ShowTorrentMatches(title string, torrentSeason string, torrentEpisode string, requestedSeason string, requestedEpisode string) bool {
	seasonMatches := requestedSeason == "0" || torrentSeason == requestedSeason
	episodeMatches := requestedEpisode == "0" || torrentEpisode == requestedEpisode

	// Individual episode releases must match both values exactly (unless the
	// corresponding request value is the wildcard "0").
	if torrentEpisode != "" {
		return seasonMatches && episodeMatches
	}

	// A season pack contains every episode in that season.
	if seasonMatches && torrentSeason != "" {
		return true
	}

	// A multi-season range can contain the requested season even though the
	// basic parser records only its first season.
	if requestedSeason != "0" && seasonRangeContains(title, requestedSeason) {
		return true
	}

	// Markerless complete-series releases contain every season and episode.
	return torrentSeason == "" && strings.Contains(strings.ToLower(title), "complete")
}

var seasonRangeRegex = regexp.MustCompile(`(?i)s0*(\d{1,3})\s*-\s*s?0*(\d{1,3})`)

func seasonRangeContains(title string, requestedSeason string) bool {
	season, err := strconv.Atoi(requestedSeason)
	if err != nil {
		return false
	}

	match := seasonRangeRegex.FindStringSubmatch(title)
	if len(match) != 3 {
		return false
	}

	first, firstErr := strconv.Atoi(match[1])
	last, lastErr := strconv.Atoi(match[2])
	return firstErr == nil && lastErr == nil && season >= first && season <= last
}

func DecodeSize(value string) string {
	re := regexp.MustCompile("[0-9.]+")
	stringsize := re.FindAllString(value, -1)
	f, _ := strconv.ParseFloat(stringsize[0], 64)
	re = regexp.MustCompile("(?:GB|MB)")
	unit := re.FindAllString(value, -1)
	if unit[0] == "GB" {
		f = f * 1024 * 1024 * 1024
	} else if unit[0] == "MB" {
		f = f * 1024 * 1024
	} else if unit[0] == "KB" {
		f = f * 1024
	}
	return strconv.FormatFloat(f, 'f', 0, 64)
}

func DecodeLanguage(value string, language string) string {
	value = strings.TrimSpace(value)
	value = strings.Title(value)
	var enLangArray = [...][2]string{
		{"ar", "Arabic"}, {"bg", "Bulgarian"}, {"hr", "Croatian"}, {"cs", "Czech"}, {"da", "Danish"}, {"nl", "Dutch"}, {"en", "English"}, {"et", "Estonian"}, {"fi", "Finnish"},
		{"fr", "French"}, {"de", "German"}, {"el", "Greek"}, {"he", "Hebrew"}, {"hu", "Hungarian"}, {"id", "Indonesian"}, {"it", "Italian"}, {"ko", "Korean"}, {"lv", "Latvian"},
		{"lt", "Lithuanian"}, {"no", "Norwegian"}, {"fa", "Persian"}, {"pl", "Polish"}, {"pt", "Portuguese"}, {"ro", "Romanian"}, {"ru", "Russian"}, {"sr", "Serbian"}, {"sk", "Slovak"},
		{"es", "Spanish"}, {"sw", "Swahili"}, {"sv", "Swedish"}, {"th", "Thai"}, {"tr", "Turkish"}, {"ur", "Urdu"}, {"vi", "Vietnamese"},
	}

	var huLangArray = [...][2]string{
		{"ar", "Arab"}, {"bg", "Bolgár"}, {"hr", "Horvát"}, {"cs", "Cseh"}, {"da", "Dán"}, {"nl", "Holland"}, {"en", "Angol"}, {"et", "Észt"}, {"fi", "Finn"},
		{"fr", "Francia"}, {"de", "Német"}, {"el", "Görög"}, {"he", "Héber"}, {"hu", "Magyar"}, {"id", "Indonéz"}, {"it", "Olasz"}, {"ko", "Koreai"}, {"lv", "Lett"},
		{"lt", "Litván"}, {"no", "Norvég"}, {"fa", "Perzsa"}, {"pl", "Lengyel"}, {"pt", "Portugál"}, {"ro", "Román"}, {"ru", "Orosz"}, {"sr", "Szerb"}, {"sk", "Szlovák"},
		{"es", "Spanyol"}, {"sw", "Szuahéli"}, {"sv", "Svéd"}, {"th", "Thai"}, {"tr", "Török"}, {"ur", "Urdu"}, {"vi", "Vietnámi"},
	}

	langArray := enLangArray

	switch language {
	case "hu":
		langArray = huLangArray
	}

	for _, lang := range langArray {
		if lang[1] == value {
			return lang[0]
		}
	}

	return "en"
}

func RemoveFileExtension(filename string) string {
	return filename[0 : len(filename)-len(path.Ext(filename))]
}

func CleanString(value string) string {
	unwanted, err := regexp.Compile("[^a-zA-Z0-9 _:.+-]+")
	if err == nil {
		value = unwanted.ReplaceAllString(value, "")
	}

	return strings.TrimSpace(value)
}
