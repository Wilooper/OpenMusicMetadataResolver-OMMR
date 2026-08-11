package query

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var (
	// Noise patterns commonly present in audio/video titles and artist names
	noiseRegexes = []*regexp.Regexp{
		regexp.MustCompile(`(?i)[\(\[\{]\s*(official\s+(video|audio|music\s+video|lyric\s+video|visualizer|clip)|remastered|remaster|hd|4k|\d{4}\s+remaster)\s*[\)\]\}]`),
		regexp.MustCompile(`(?i)\b(official\s+(video|audio|music\s+video|lyric\s+video|visualizer|clip))\b`),
		regexp.MustCompile(`(?i)\b(remastered|remaster|hd|4k|vevo)\b`),
		regexp.MustCompile(`(?i)\b(19|20)\d{2}\s+remaster\b`),
	}
	artistNoiseRegexes = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\s*-\s*topic$`),
		regexp.MustCompile(`(?i)\s*vevo$`),
	}
	emptyBracketsRegex = regexp.MustCompile(`[\(\[\{]\s*[\)\]\}]`)
	multiSpaceRegex    = regexp.MustCompile(`\s+`)
)

// CleanTitle normalizes a track title by stripping common noise patterns (e.g. "Official Video", "[Remastered]"),
// converting unicode diacritics to ASCII, and folding whitespace.
func CleanTitle(title string) string {
	if title == "" {
		return ""
	}

	cleaned := title
	for _, re := range noiseRegexes {
		cleaned = re.ReplaceAllString(cleaned, "")
	}

	cleaned = emptyBracketsRegex.ReplaceAllString(cleaned, "")
	cleaned = NormalizeUnicode(cleaned)
	cleaned = strings.ToLower(cleaned)
	cleaned = multiSpaceRegex.ReplaceAllString(cleaned, " ")
	return strings.TrimSpace(cleaned)
}

// CleanArtist normalizes artist names by stripping channel suffixes (e.g. "- Topic", "VEVO"), diacritics, and converting to lower case.
func CleanArtist(artist string) string {
	if artist == "" {
		return ""
	}

	cleaned := artist
	for _, re := range artistNoiseRegexes {
		cleaned = re.ReplaceAllString(cleaned, "")
	}

	cleaned = NormalizeUnicode(cleaned)
	cleaned = strings.ToLower(cleaned)
	cleaned = multiSpaceRegex.ReplaceAllString(cleaned, " ")
	return strings.TrimSpace(cleaned)
}

// NormalizeUnicode removes accents/diacritics and converts non-ASCII characters to closest representation.
func NormalizeUnicode(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, err := transform.String(t, s)
	if err != nil {
		return s
	}
	return result
}
