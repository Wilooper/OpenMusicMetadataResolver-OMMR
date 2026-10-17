package resolver

import (
	"strings"
	"unicode"

	"github.com/ommr/ommr/internal/matcher"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/pkg/query"
)

func artistTokens(name string) []string {
	tokens := strings.FieldsFunc(query.CleanArtist(name), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	result := tokens[:0]
	for _, token := range tokens {
		if token != "and" && token != "feat" && token != "featuring" && token != "ft" {
			result = append(result, token)
		}
	}
	if len(result) > 1 && result[0] == "the" {
		result = result[1:]
	}
	return result
}

func hasArtistEvidence(expected string, artists []canonical.Artist) bool {
	target := artistTokens(expected)
	if len(target) == 0 || len(artists) == 0 {
		return false
	}
	names := make([]string, len(artists))
	for i, artist := range artists {
		names[i] = artist.Name
	}
	candidates := append(append([]string(nil), names...), strings.Join(names, " "))
	for _, name := range candidates {
		tokens := artistTokens(name)
		for start := 0; start+len(target) <= len(tokens); start++ {
			match := true
			for index, word := range target {
				if word != tokens[start+index] {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
		// Allow close spelling variants, never a shared surname/token alone.
		a, b := strings.Join(target, " "), strings.Join(tokens, " ")
		if matcher.JaroWinkler(a, b) >= 0.94 && matcher.TokenSetRatio(a, b) >= 0.85 {
			return true
		}
	}
	return false
}
