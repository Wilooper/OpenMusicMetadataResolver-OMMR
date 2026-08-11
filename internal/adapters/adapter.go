package adapters

import (
	"context"

	"github.com/ommr/ommr/internal/models/canonical"
)

// Query represents a resolution query sent to provider adapters.
type Query struct {
	SpotifyID  string
	YouTubeID  string
	DeezerID   string
	AppleID    string
	ISRC       string
	Artist     string
	Title      string
}

// ProviderAdapter defines the common contract implemented by all music metadata providers.
type ProviderAdapter interface {
	Name() string
	Version() string
	FetchByID(ctx context.Context, idType string, id string) (*canonical.TrackCandidate, error)
	Search(ctx context.Context, query Query) ([]canonical.TrackCandidate, error)
}
