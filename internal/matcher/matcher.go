package matcher

import (
	"math"
	"strings"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/pkg/query"
)

// Matcher computes calibrated composite match scores, breakdown components, and structured confidence.
type Matcher struct{}

func New() *Matcher {
	return &Matcher{}
}

func floatPtr(v float64) *float64 {
	val := math.Round(v*100) / 100
	return &val
}

// ScoreCandidate calculates composite match score [0.0 - 1.0] and detailed MatchBreakdown.
func (m *Matcher) ScoreCandidate(cand canonical.TrackCandidate, target adapters.Query) (float64, canonical.MatchBreakdown) {
	var breakdown canonical.MatchBreakdown

	// 1. ISRC Match
	if target.ISRC != "" && cand.Track.ISRC != "" {
		if strings.EqualFold(target.ISRC, cand.Track.ISRC) {
			breakdown.ISRC = floatPtr(1.0)
			breakdown.Title = floatPtr(1.0)
			breakdown.Artists = floatPtr(1.0)
			breakdown.Duration = floatPtr(1.0)
			if cand.Track.ReleaseDate != "" {
				breakdown.ReleaseDate = floatPtr(1.0)
			}
			return 1.0, breakdown
		}
	} else if cand.Track.ISRC != "" {
		breakdown.ISRC = floatPtr(1.0)
	}

	// 2. Title Similarity
	targetTitle := query.CleanTitle(target.Title)
	candTitle := query.CleanTitle(cand.Track.Title)

	if targetTitle != "" && candTitle != "" {
		jw := JaroWinkler(targetTitle, candTitle)
		ts := TokenSetRatio(targetTitle, candTitle)
		score := 0.6*jw + 0.4*ts
		breakdown.Title = floatPtr(score)
	}

	// 3. Artist Similarity
	targetArtist := query.CleanArtist(target.Artist)
	if targetArtist != "" && len(cand.Track.Artists) > 0 {
		candArtist := query.CleanArtist(cand.Track.Artists[0].Name)
		score := JaroWinkler(targetArtist, candArtist)
		breakdown.Artists = floatPtr(score)
	} else if len(cand.Track.Artists) > 0 {
		breakdown.Artists = floatPtr(0.8)
	}

	// 4. Album Similarity
	if targetTitle != "" && cand.Track.Album.Title != "" {
		candAlbum := query.CleanTitle(cand.Track.Album.Title)
		score := JaroWinkler(targetTitle, candAlbum)
		breakdown.Album = floatPtr(score)
	}

	// 5. Duration & Release Date Availability
	if cand.Track.DurationMS > 0 {
		breakdown.Duration = floatPtr(1.0)
	}
	if cand.Track.ReleaseDate != "" {
		breakdown.ReleaseDate = floatPtr(1.0)
	}

	titleScore := 0.0
	if breakdown.Title != nil {
		titleScore = *breakdown.Title
	}
	artistScore := 0.0
	if breakdown.Artists != nil {
		artistScore = *breakdown.Artists
	}

	albumScore := 0.0
	if breakdown.Album != nil {
		albumScore = *breakdown.Album
	}

	// Calibrated composite scoring: Title (0.50), Artist (0.40), Album (0.10)
	rawScore := 0.50*titleScore + 0.40*artistScore + 0.10*albumScore

	// Cap exact non-ISRC matches at 0.96 (Reserved 1.00 for verified ISRC match)
	if rawScore >= 0.99 && (target.ISRC == "" || cand.Track.ISRC == "") {
		rawScore = 0.96
	}

	score := math.Round(rawScore*100) / 100

	return score, breakdown
}

// ScoreCandidates evaluates a list of candidates against target query and assigns MatchScore, MatchBreakdown, and Confidence.
func (m *Matcher) ScoreCandidates(candidates []canonical.TrackCandidate, target adapters.Query) []canonical.TrackCandidate {
	scored := make([]canonical.TrackCandidate, len(candidates))
	for i, c := range candidates {
		score, bd := m.ScoreCandidate(c, target)
		c.MatchScore = score
		c.Track.MatchScore = score
		c.Track.MatchBreakdown = bd
		c.Track.Confidence = canonical.Confidence{
			Level: canonical.GetConfidenceLevel(score, 0),
			Score: score,
		}
		scored[i] = c
	}
	return scored
}
