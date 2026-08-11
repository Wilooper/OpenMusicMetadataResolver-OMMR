package applemusic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/models/provider"
	"github.com/ommr/ommr/pkg/query"
)

const (
	ProviderName    = "applemusic"
	ProviderVersion = "applemusic-v1"
	BaseURL         = "https://itunes.apple.com"
)

type Provider struct {
	httpClient *http.Client
}

var _ adapters.ProviderAdapter = (*Provider)(nil)

func New() *Provider {
	return &Provider{
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func (p *Provider) Name() string {
	return ProviderName
}

func (p *Provider) Version() string {
	return ProviderVersion
}

func (p *Provider) FetchByID(ctx context.Context, idType string, id string) (*canonical.TrackCandidate, error) {
	if idType != "applemusic" && idType != "apple_id" && idType != "itunes" {
		return nil, fmt.Errorf("unsupported ID type %q for Apple Music", idType)
	}

	reqURL := fmt.Sprintf("%s/lookup?id=%s&entity=song", BaseURL, url.QueryEscape(id))
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
		return nil, fmt.Errorf("apple music API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var searchResp provider.AppleSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil || len(searchResp.Results) == 0 {
		return nil, fmt.Errorf("apple music track %s not found", id)
	}

	track := p.normalizeResult(searchResp.Results[0])
	return &canonical.TrackCandidate{
		Provider:    ProviderName,
		Track:       track,
		MatchScore:  1.0,
		RawResponse: body,
	}, nil
}

func (p *Provider) Search(ctx context.Context, q adapters.Query) ([]canonical.TrackCandidate, error) {
	var term string
	if q.Artist != "" && q.Title != "" {
		term = fmt.Sprintf("%s %s", query.CleanArtist(q.Artist), query.CleanTitle(q.Title))
	} else if q.Title != "" {
		term = query.CleanTitle(q.Title)
	} else if q.ISRC != "" {
		term = q.ISRC
	} else {
		return nil, nil
	}

	reqURL := fmt.Sprintf("%s/search?term=%s&entity=song&limit=5", BaseURL, url.QueryEscape(term))
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
		return nil, fmt.Errorf("apple music search returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var searchResp provider.AppleSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, err
	}

	candidates := make([]canonical.TrackCandidate, 0, len(searchResp.Results))
	for _, item := range searchResp.Results {
		if item.WrapperType != "track" && item.Kind != "song" {
			continue
		}
		cand := canonical.TrackCandidate{
			Provider:    ProviderName,
			Track:       p.normalizeResult(item),
			MatchScore:  0.90,
			RawResponse: body,
		}
		candidates = append(candidates, cand)
	}

	return candidates, nil
}

func (p *Provider) normalizeResult(item provider.AppleTrackResult) canonical.Track {
	artists := []canonical.Artist{
		{
			Name: item.ArtistName,
			Role: "main",
			IDs:  map[string]string{"applemusic": strconv.FormatInt(item.ArtistID, 10)},
		},
	}

	images := make([]canonical.Image, 0, 2)
	if item.ArtworkUrl100 != "" {
		// Apple artwork URLs end with 100x100bb.jpg - we can scale to 600x600bb.jpg
		hiRes := strings.Replace(item.ArtworkUrl100, "100x100bb", "600x600bb", 1)
		images = append(images, canonical.Image{URL: hiRes, Width: 600, Height: 600, Type: "cover"})
		images = append(images, canonical.Image{URL: item.ArtworkUrl100, Width: 100, Height: 100, Type: "cover"})
	}

	album := canonical.Album{
		Title:       item.CollectionName,
		ReleaseDate: parseDate(item.ReleaseDate),
		Images:      images,
		IDs:         map[string]string{"applemusic": strconv.FormatInt(item.CollectionID, 10)},
	}

	genres := make([]string, 0, 1)
	if item.PrimaryGenreName != "" {
		genres = append(genres, item.PrimaryGenreName)
	}

	explicit := strings.EqualFold(item.TrackExplicitness, "explicit")

	return canonical.Track{
		Title:       item.TrackName,
		Artists:     artists,
		Album:       album,
		DurationMS:  item.TrackTimeMillis,
		ReleaseDate: parseDate(item.ReleaseDate),
		Explicit:    explicit,
		ISRC:        item.ISRC,
		Genres:      genres,
		Images:      images,
		IDs:         map[string]string{"applemusic": strconv.FormatInt(item.TrackID, 10)},
		Sources:     []string{ProviderName},
	}
}

func parseDate(d string) string {
	if len(d) >= 10 {
		return d[:10]
	}
	return d
}
