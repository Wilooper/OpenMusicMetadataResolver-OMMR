package jiosaavn

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/models/provider"
)

const (
	ProviderName    = "jiosaavn"
	ProviderVersion = "jiosaavn-search-v1"
)

var SearchURL = "https://www.jiosaavn.com/api.php"

type Provider struct {
	httpClient *http.Client
}

var _ adapters.ProviderAdapter = (*Provider)(nil)

func New() *Provider {
	return &Provider{
		httpClient: &http.Client{Timeout: 8 * time.Second},
	}
}

func (p *Provider) Name() string {
	return ProviderName
}

func (p *Provider) Version() string {
	return ProviderVersion
}

// FetchByID resolves a JioSaavn song id (the short id returned by search,
// e.g. "6uEI9gj0") via the song.getDetails API.
func (p *Provider) FetchByID(ctx context.Context, idType string, id string) (*canonical.TrackCandidate, error) {
	if idType != "jiosaavn" && idType != "jiosaavn_id" {
		return nil, fmt.Errorf("unsupported ID type %q for JioSaavn", idType)
	}

	// A JioSaavn "id" is an 8-char token such as "6uEI9gj0".
	id = strings.TrimSpace(id)
	params := url.Values{}
	params.Set("__call", "song.getDetails")
	params.Set("pids", id)
	params.Set("_format", "json")
	params.Set("_marker", "0")
	params.Set("ctx", "web6dot0")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, SearchURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jiosaavn getDetails status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// getDetails returns a map keyed by song id.
	var m map[string]provider.JioSaavnSongResult
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	detail, ok := m[id]
	if !ok || detail.Song == "" {
		return nil, fmt.Errorf("jiosaavn song %s not found", id)
	}

	track := p.normalize(detail)
	return &canonical.TrackCandidate{
		Provider:    ProviderName,
		Track:       track,
		MatchScore:  1.0,
		RawResponse: body,
	}, nil
}

func (p *Provider) Search(ctx context.Context, q adapters.Query) ([]canonical.TrackCandidate, error) {
	params := url.Values{}
	params.Set("__call", "search.getResults")
	params.Set("q", strings.TrimSpace(q.Artist+" "+q.Title))
	params.Set("_format", "json")
	params.Set("_marker", "0")
	params.Set("ctx", "web6dot0")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, SearchURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jiosaavn search status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var search provider.JioSaavnSearchResponse
	if err := json.Unmarshal(body, &search); err != nil {
		return nil, err
	}

	cands := make([]canonical.TrackCandidate, 0, len(search.Results))
	for _, r := range search.Results {
		cands = append(cands, canonical.TrackCandidate{
			Provider:    ProviderName,
			Track:       p.normalize(r),
			MatchScore:  0,
			RawResponse: body,
		})
	}
	return cands, nil
}

func (p *Provider) normalize(r provider.JioSaavnSongResult) canonical.Track {
	artists := splitNames(r.PrimaryArtists)
	if len(artists) == 0 {
		artists = splitNames(r.Singers)
	}
	if len(artists) == 0 && r.Music != "" {
		artists = []canonical.Artist{{Name: r.Music, Role: "main"}}
	}
	if len(artists) == 0 {
		artists = []canonical.Artist{{Name: "Unknown Artist", Role: "main"}}
	}

	if len(r.FeaturedArtists) > 0 {
		for _, fa := range splitNames(r.FeaturedArtists) {
			artists = append(artists, canonical.Artist{Name: fa.Name, Role: "featured"})
		}
	}

	images := make([]canonical.Image, 0, 1)
	if r.Image != "" {
		images = append(images, canonical.Image{
			URL:    upscaleImage(r.Image, 500),
			Width:  500,
			Height: 500,
			Type:   "cover",
		})
	}

	ids := map[string]string{"jiosaavn": jioSaavnID(r)}
	if r.ID != "" {
		ids["jiosaavn_track_id"] = r.ID
	}
	if r.PermaURL != "" {
		ids["jiosaavn_url"] = r.PermaURL
	}

	track := canonical.Track{
		Title:   r.Song,
		Artists: artists,
		Album: canonical.Album{
			Title:  r.Album,
			Images: images,
		},
		Images:         images,
		Label:          r.Label,
		Explicit:       string(r.ExplicitContent) == "1",
		Language:       r.Language,
		IDs:            ids,
		Sources:        []string{ProviderName},
		MatchBreakdown: canonical.MatchBreakdown{},
	}

	if secs := r.DurationSeconds(); secs > 0 {
		track.DurationMS = int64(secs) * 1000
	}
	if n := r.PlayCountInt(); n > 0 {
		track.PlayCount = n
	}
	if r.ReleaseDate != "" {
		track.ReleaseDate = r.ReleaseDate
	}
	return track
}

// jioSaavnID returns the stable song token from the perma_url when present,
// falling back to the short search id.
func jioSaavnID(r provider.JioSaavnSongResult) string {
	if r.PermaURL != "" {
		parts := strings.Split(strings.TrimSuffix(r.PermaURL, "/"), "/")
		last := parts[len(parts)-1]
		if last != "" {
			return last
		}
	}
	return r.ID
}

func splitNames(s string) []canonical.Artist {
	var artists []canonical.Artist
	for _, part := range strings.Split(s, ",") {
		name := strings.TrimSpace(part)
		if name != "" {
			artists = append(artists, canonical.Artist{Name: name, Role: "main"})
		}
	}
	return artists
}

func upscaleImage(imgURL string, size int) string {
	// Saavn serves images at e.g. "...-150x150.jpg"; bump to a larger square.
	re := regexp.MustCompile(`-\d+x\d+(\.\w+)$`)
	return re.ReplaceAllString(imgURL, fmt.Sprintf("-%dx%d$1", size, size))
}

var userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
