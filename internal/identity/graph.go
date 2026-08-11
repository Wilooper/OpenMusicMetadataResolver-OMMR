package identity

import (
	"math"
	"strings"

	"github.com/ommr/ommr/internal/models/canonical"
)

// ProviderTrustWeights defines trust multipliers per metadata provider.
var ProviderTrustWeights = map[string]float64{
	"musicbrainz": 1.00,
	"isrc":        1.00,
	"applemusic":  0.95,
	"spotify":     0.95,
	"deezer":      0.90,
	"ytmusic":     0.80,
}

// IdentityGraph stores canonical identity mappings across platform IDs.
type IdentityGraph struct {
	CanonicalID   string                    `json:"canonical_id"`
	SpotifyID     string                    `json:"spotify_id,omitempty"`
	YouTubeID     string                    `json:"youtube_id,omitempty"`
	AppleMusicID  string                    `json:"applemusic_id,omitempty"`
	DeezerID      string                    `json:"deezer_id,omitempty"`
	MusicBrainzID string                    `json:"musicbrainz_id,omitempty"`
	ISRC          string                    `json:"isrc,omitempty"`
	Matches       []canonical.IdentityMatch `json:"matches"`
}

// CalculateIdentityConfidence returns the weighted identity confidence given match score and provider name.
func CalculateIdentityConfidence(score float64, provider string) float64 {
	weight, ok := ProviderTrustWeights[strings.ToLower(provider)]
	if !ok {
		weight = 0.85
	}
	conf := score * weight
	return math.Round(conf*100) / 100
}

// BuildGraph constructs an IdentityGraph from a resolved canonical Track.
func BuildGraph(t canonical.Track) IdentityGraph {
	return IdentityGraph{
		CanonicalID:   t.CanonicalID,
		SpotifyID:     t.IDs["spotify"],
		YouTubeID:     t.IDs["ytmusic"],
		AppleMusicID:  t.IDs["applemusic"],
		DeezerID:      t.IDs["deezer"],
		MusicBrainzID: t.IDs["musicbrainz"],
		ISRC:          t.ISRC,
		Matches:       t.IdentityMatches,
	}
}
