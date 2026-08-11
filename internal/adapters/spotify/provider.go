package spotify

import (
	"context"
	"fmt"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/canonical"
)

const (
	ProviderName    = "spotify"
	ProviderVersion = "spotify-web-oembed"
)

type Provider struct {
	scraper *Scraper
}

var _ adapters.ProviderAdapter = (*Provider)(nil)

func New() *Provider {
	return &Provider{
		scraper: NewScraper(),
	}
}

func (p *Provider) Name() string {
	return ProviderName
}

func (p *Provider) Version() string {
	return ProviderVersion
}

func (p *Provider) FetchByID(ctx context.Context, idType string, id string) (*canonical.TrackCandidate, error) {
	if idType != "spotify" && idType != "spotify_id" {
		return nil, fmt.Errorf("unsupported ID type %q for Spotify", idType)
	}
	return p.scraper.FetchOembed(ctx, id)
}

func (p *Provider) Search(ctx context.Context, q adapters.Query) ([]canonical.TrackCandidate, error) {
	if q.SpotifyID != "" {
		cand, err := p.FetchByID(ctx, "spotify", q.SpotifyID)
		if err != nil {
			return nil, err
		}
		return []canonical.TrackCandidate{*cand}, nil
	}
	return nil, nil
}
