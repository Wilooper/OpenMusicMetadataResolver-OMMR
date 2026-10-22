package resolver

import (
	"regexp"
	"strings"

	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/pkg/query"
)

var featuredTitle = regexp.MustCompile(`(?i)^(.*?)\s*[\(\[]\s*(?:feat\.?|ft\.?|featuring)\s+([^\)\]]+)[\)\]]\s*$`)

func hasTitleEvidence(expected string, candidate canonical.TrackCandidate) bool {
	if score := candidate.Track.MatchBreakdown.Title; score != nil && *score >= 0.72 {
		return true
	}
	// A source can report a featured performer in artists rather than in its
	// title. Require both the exact base title and that explicit artist evidence.
	parts := featuredTitle.FindStringSubmatch(expected)
	return len(parts) == 3 && strings.TrimSpace(parts[1]) != "" && query.CleanTitle(parts[1]) == query.CleanTitle(candidate.Track.Title) && hasArtistEvidence(parts[2], candidate.Track.Artists)
}
