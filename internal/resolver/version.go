package resolver

import (
	"regexp"
	"strings"
)

var versionQualifier = regexp.MustCompile(`(?i)(?:[\(\[]|\s[-–—]\s)([^\)\]]+)`)
var versionMarkers = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(remix|remixed)\b`),
	regexp.MustCompile(`(?i)\blive\b`),
	regexp.MustCompile(`(?i)\bacoustic\b`),
	regexp.MustCompile(`(?i)\binstrumental\b`),
	regexp.MustCompile(`(?i)\bkaraoke\b`),
	regexp.MustCompile(`(?i)\bsped[ -]?up\b`),
	regexp.MustCompile(`(?i)\bslowed\b`),
	regexp.MustCompile(`(?i)\bradio[ -]?edit\b`),
}

func compatibleVersion(target, candidate string) bool {
	qualifiers := func(title string) string {
		var parts []string
		for _, group := range versionQualifier.FindAllStringSubmatch(title, -1) {
			parts = append(parts, group[1])
		}
		return strings.Join(parts, " ")
	}
	wanted, found := qualifiers(target), qualifiers(candidate)
	for _, marker := range versionMarkers {
		if marker.MatchString(wanted) != marker.MatchString(found) {
			return false
		}
	}
	return true
}
