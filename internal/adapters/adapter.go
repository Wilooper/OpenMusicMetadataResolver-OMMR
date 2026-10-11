package adapters

import (
	"context"

	"github.com/ommr/ommr/internal/models/canonical"
)

// Query represents a resolution query sent to provider adapters.
type Query struct {
	SpotifyID     string `json:"spotify_id,omitempty"`
	YouTubeID     string `json:"youtube_id,omitempty"`
	DeezerID      string `json:"deezer_id,omitempty"`
	AppleID       string `json:"apple_id,omitempty"`
	SoundCloudID  string `json:"soundcloud_id,omitempty"`
	QobuzID       string `json:"qobuz_id,omitempty"`
	TidalID       string `json:"tidal_id,omitempty"`
	AmazonMusicID string `json:"amazonmusic_id,omitempty"`
	PandoraID     string `json:"pandora_id,omitempty"`
	ISRC          string `json:"isrc,omitempty"`
	Artist        string `json:"artist,omitempty"`
	Title         string `json:"title,omitempty"`
	Album         string `json:"album,omitempty"`
}

// ProviderAdapter defines the common contract implemented by all music metadata providers.
type ProviderAdapter interface {
	Name() string
	Version() string
	FetchByID(ctx context.Context, idType string, id string) (*canonical.TrackCandidate, error)
	Search(ctx context.Context, query Query) ([]canonical.TrackCandidate, error)
}

// MetadataEnricher loads richer metadata only after a candidate is accepted.
type MetadataEnricher interface {
	Enrich(ctx context.Context, track canonical.Track) (*canonical.Track, error)
}
