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

	if target.ISRC != "" && cand.Track.ISRC != "" && !strings.EqualFold(strings.TrimSpace(target.ISRC), strings.TrimSpace(cand.Track.ISRC)) {
		breakdown.ISRC = floatPtr(0)
		return 0, breakdown
	}

	// 0. Direct Platform ID Match
	// When the query carried a specific platform ID (e.g. youtube_id) and the
	// candidate's provider ID matches it exactly, this is a verified direct
	// lookup and is scored at full confidence.
	if directIDMatch(target, cand) {
		breakdown.Title = floatPtr(1.0)
		breakdown.Artists = floatPtr(1.0)
		if cand.Track.DurationMS > 0 {
			breakdown.Duration = floatPtr(1.0)
		}
		if cand.Track.ReleaseDate != "" {
			breakdown.ReleaseDate = floatPtr(1.0)
		}
		return 1.0, breakdown
	}

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

	// 3. Artist Similarity (composite: Jaro-Winkler + Token Set Ratio, so
	//    unrelated artists sharing a common prefix are not over-scored)
	targetArtist := query.CleanArtist(target.Artist)
	if targetArtist != "" && len(cand.Track.Artists) > 0 {
		candArtist := query.CleanArtist(cand.Track.Artists[0].Name)
		jw := JaroWinkler(targetArtist, candArtist)
		ts := TokenSetRatio(targetArtist, candArtist)
		score := 0.6*jw + 0.4*ts
		breakdown.Artists = floatPtr(score)
	} else if len(cand.Track.Artists) > 0 {
		breakdown.Artists = floatPtr(0.8)
	}

	// 4. Album Similarity (only evaluated when the target query carries album context)
	if target.Album != "" && cand.Track.Album.Title != "" {
		targetAlbum := query.CleanTitle(target.Album)
		candAlbum := query.CleanTitle(cand.Track.Album.Title)
		if targetAlbum != "" && candAlbum != "" {
			jw := JaroWinkler(targetAlbum, candAlbum)
			ts := TokenSetRatio(targetAlbum, candAlbum)
			score := 0.6*jw + 0.4*ts
			breakdown.Album = floatPtr(score)
		}
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

	// Calibrated composite scoring.
	// When album context is unavailable in the target query, weights are
	// normalized across Title (0.55) and Artist (0.45) so a missing album
	// reference never deflates otherwise-exact matches.
	var rawScore float64
	if breakdown.Album == nil {
		rawScore = 0.55*titleScore + 0.45*artistScore
	} else {
		rawScore = 0.50*titleScore + 0.40*artistScore + 0.10*albumScore
	}

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

// directIDMatch reports whether the target query carries a platform ID that
// exactly matches one of the candidate's IDs.
func directIDMatch(target adapters.Query, c canonical.TrackCandidate) bool {
	for k, v := range c.Track.IDs {
		if !strings.EqualFold(k, c.Provider) {
			continue
		}
		switch k {
		case "spotify":
			if target.SpotifyID != "" && v == target.SpotifyID {
				return true
			}
		case "ytmusic":
			if target.YouTubeID != "" && v == target.YouTubeID {
				return true
			}
		case "deezer":
			if target.DeezerID != "" && v == target.DeezerID {
				return true
			}
		case "applemusic":
			if target.AppleID != "" && v == target.AppleID {
				return true
			}
		case "soundcloud":
			if target.SoundCloudID != "" && v == target.SoundCloudID {
				return true
			}
		case "qobuz":
			if target.QobuzID != "" && v == target.QobuzID {
				return true
			}
		case "tidal":
			if target.TidalID != "" && v == target.TidalID {
				return true
			}
		case "amazonmusic":
			if target.AmazonMusicID != "" && v == target.AmazonMusicID {
				return true
			}
		case "pandora":
			if target.PandoraID != "" && v == target.PandoraID {
				return true
			}
		}
	}
	return false
}
