package spotify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/models/provider"
	"github.com/ommr/ommr/pkg/query"
)

const (
	ProviderName    = "spotify"
	ProviderVersion = "spotify-web-v2" // dual-mode: official Web API + anonymous Web API / oEmbed / embed scraping
	apiBaseURL      = "https://api.spotify.com/v1"
)

// Config carries optional Spotify for Developers credentials. When both are
// provided the official client-credentials OAuth flow is used; otherwise the
// adapter falls back to the unofficial anonymous token / oEmbed / embed modes.
type Config struct {
	ClientID     string
	ClientSecret string
}

type Provider struct {
	httpClient *http.Client
	tokens     *tokenManager
	config     Config
}

var _ adapters.ProviderAdapter = (*Provider)(nil)

func New(cfg Config) *Provider {
	return &Provider{
		httpClient: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }},
		tokens:     newTokenManager(&http.Client{Timeout: 8 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}, cfg.ClientID, cfg.ClientSecret),
		config:     cfg,
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

	// Preferred path: Web API via official (developer credentials) or
	// unofficial (anonymous web-player token) authentication.
	if cand, err := p.fetchByWebAPI(ctx, id); err == nil {
		return cand, nil
	}

	// Fallback path 1: embed page embedded JSON state.
	if cand, err := p.fetchByEmbed(ctx, id); err == nil {
		return cand, nil
	}

	// Fallback path 2: public oEmbed endpoint.
	return p.fetchByOembed(ctx, id)
}

func (p *Provider) Search(ctx context.Context, q adapters.Query) ([]canonical.TrackCandidate, error) {
	token, err := p.tokens.Get(ctx)
	if err != nil {
		return nil, err
	}

	searchQ, err := buildSearchQuery(q)
	if err != nil {
		return nil, err
	}

	reqURL := fmt.Sprintf("%s/search?q=%s&type=track&limit=10", apiBaseURL, url.QueryEscape(searchQ))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify web search status %d: %s", resp.StatusCode, string(body))
	}

	var searchResp provider.SpotifyWebSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, err
	}

	candidates := make([]canonical.TrackCandidate, 0, len(searchResp.Tracks.Items))
	for _, item := range searchResp.Tracks.Items {
		cand := canonical.TrackCandidate{
			Provider:    ProviderName,
			Track:       p.normalizeWebTrack(item),
			MatchScore:  0.90,
			RawResponse: body,
		}
		candidates = append(candidates, cand)
	}
	return candidates, nil
}

// fetchByWebAPI resolves a track through the Spotify Web API using either the
// official or anonymous token.
func (p *Provider) fetchByWebAPI(ctx context.Context, id string) (*canonical.TrackCandidate, error) {
	token, err := p.tokens.Get(ctx)
	if err != nil {
		return nil, err
	}

	reqURL := fmt.Sprintf("%s/tracks/%s", apiBaseURL, url.PathEscape(id))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify web track status %d", resp.StatusCode)
	}

	var track provider.SpotifyWebTrack
	if err := json.Unmarshal(body, &track); err != nil {
		return nil, err
	}
	if track.ID == "" && track.Name == "" {
		return nil, fmt.Errorf("spotify track %s not found", id)
	}

	normalized := p.normalizeWebTrack(track)
	p.enrichArtistGenres(ctx, token, &normalized)

	return &canonical.TrackCandidate{
		Provider:    ProviderName,
		Track:       normalized,
		MatchScore:  1.0,
		RawResponse: body,
	}, nil
}

// fetchByEmbed scrapes the embedded JSON state from open.spotify.com/embed/track/{id}.
func (p *Provider) fetchByEmbed(ctx context.Context, id string) (*canonical.TrackCandidate, error) {
	reqURL := fmt.Sprintf("https://open.spotify.com/embed/track/%s", url.PathEscape(id))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify embed status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	track, err := ParseEmbedNextData(id, body)
	if err != nil {
		return nil, err
	}

	return &canonical.TrackCandidate{
		Provider:    ProviderName,
		Track:       *track,
		MatchScore:  1.0,
		RawResponse: body,
	}, nil
}

// fetchByOembed resolves track metadata using Spotify's public oEmbed endpoint.
func (p *Provider) fetchByOembed(ctx context.Context, id string) (*canonical.TrackCandidate, error) {
	trackURL := fmt.Sprintf("https://open.spotify.com/track/%s", url.PathEscape(id))
	reqURL := fmt.Sprintf("https://open.spotify.com/oembed?url=%s", url.QueryEscape(trackURL))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.httpClient.Do(req)
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

	track := ParseOembed(id, oembed)
	return &canonical.TrackCandidate{
		Provider:    ProviderName,
		Track:       track,
		MatchScore:  1.0,
		RawResponse: body,
	}, nil
}

func (p *Provider) normalizeWebTrack(t provider.SpotifyWebTrack) canonical.Track {
	artists := make([]canonical.Artist, 0, len(t.Artists))
	for _, a := range t.Artists {
		artist := canonical.Artist{Name: a.Name, Role: "main"}
		if a.ID != "" {
			artist.IDs = map[string]string{"spotify": a.ID}
		}
		artists = append(artists, artist)
	}

	images := make([]canonical.Image, 0, len(t.Album.Images))
	for _, img := range t.Album.Images {
		images = append(images, canonical.Image{URL: img.URL, Width: img.Width, Height: img.Height, Type: "cover"})
	}

	copyrights := make([]canonical.Copyright, 0, len(t.Album.Copyrights))
	for _, c := range t.Album.Copyrights {
		copyrights = append(copyrights, canonical.Copyright{Type: c.Type, Text: c.Text})
	}

	isrc := t.ISRC
	if isrc == "" {
		isrc = t.ExternalIDs.ISRC
	}

	album := canonical.Album{
		Title:       t.Album.Name,
		ReleaseDate: t.Album.ReleaseDate,
		Type:        t.Album.AlbumType,
		Label:       t.Album.Label,
		UPC:         t.Album.UPC,
		TotalTracks: t.Album.TotalTracks,
		Copyrights:  copyrights,
		Images:      images,
	}
	if t.Album.ID != "" {
		album.IDs = map[string]string{"spotify": t.Album.ID}
	}

	return canonical.Track{
		Title:       t.Name,
		Artists:     artists,
		Album:       album,
		DurationMS:  t.DurationMS,
		ReleaseDate: t.Album.ReleaseDate,
		Explicit:    t.Explicit,
		ISRC:        isrc,
		PreviewURL:  t.PreviewURL,
		TrackNumber: t.TrackNumber,
		DiscNumber:  t.DiscNumber,
		Label:       t.Album.Label,
		Barcode:     t.Album.UPC,
		Copyrights:  copyrights,
		Images:      images,
		IDs:         map[string]string{"spotify": t.ID},
		Sources:     []string{ProviderName},
	}
}

// enrichArtistGenres best-effort populates genres from the primary artist.
func (p *Provider) enrichArtistGenres(ctx context.Context, token string, track *canonical.Track) {
	if len(track.Artists) == 0 || track.Artists[0].IDs == nil {
		return
	}
	artistID := track.Artists[0].IDs["spotify"]
	if artistID == "" {
		return
	}

	reqURL := fmt.Sprintf("%s/artists/%s", apiBaseURL, url.PathEscape(artistID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var artist provider.SpotifyArtistWebResponse
	if err := json.NewDecoder(resp.Body).Decode(&artist); err != nil {
		return
	}
	track.Genres = artist.Genres
}

func buildSearchQuery(q adapters.Query) (string, error) {
	if q.ISRC != "" {
		return fmt.Sprintf("isrc:%q", strings.ToUpper(q.ISRC)), nil
	}
	if q.Artist != "" && q.Title != "" {
		return fmt.Sprintf("artist:%q track:%q", query.CleanArtist(q.Artist), query.CleanTitle(q.Title)), nil
	}
	if q.Title != "" {
		return query.CleanTitle(q.Title), nil
	}
	if q.Album != "" {
		return fmt.Sprintf("album:%q", query.CleanTitle(q.Album)), nil
	}
	return "", fmt.Errorf("no searchable fields in query")
}
