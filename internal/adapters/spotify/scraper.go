package spotify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/models/provider"
)

type Scraper struct {
	httpClient *http.Client
}

func NewScraper() *Scraper {
	return &Scraper{
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// FetchOembed fetches track metadata using Spotify's public oEmbed endpoint.
func (s *Scraper) FetchOembed(ctx context.Context, spotifyID string) (*canonical.TrackCandidate, error) {
	trackURL := fmt.Sprintf("https://open.spotify.com/track/%s", url.PathEscape(spotifyID))
	reqURL := fmt.Sprintf("https://open.spotify.com/oembed?url=%s", url.QueryEscape(trackURL))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify oembed status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var oembed provider.SpotifyOembedResponse
	if err := json.Unmarshal(body, &oembed); err != nil {
		return nil, err
	}

	track := ParseOembed(spotifyID, oembed)
	return &canonical.TrackCandidate{
		Provider:    ProviderName,
		Track:       track,
		MatchScore:  1.0,
		RawResponse: body,
	}, nil
}
