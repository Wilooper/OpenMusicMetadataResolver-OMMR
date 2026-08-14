package deezer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/models/provider"
	"github.com/ommr/ommr/pkg/query"
)

const (
	ProviderName    = "deezer"
	ProviderVersion = "deezer-v1"
	BaseURL         = "https://api.deezer.com"
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
	if idType != "deezer" && idType != "deezer_id" {
		return nil, fmt.Errorf("unsupported ID type %q for Deezer", idType)
	}

	reqURL := fmt.Sprintf("%s/track/%s", BaseURL, url.PathEscape(id))
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
		return nil, fmt.Errorf("deezer API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var trackResp provider.DeezerTrackResponse
	if err := json.Unmarshal(body, &trackResp); err != nil {
		return nil, err
	}
	if trackResp.ID == 0 {
		return nil, fmt.Errorf("deezer track %s not found", id)
	}

	track := p.normalizeTrack(trackResp)
	return &canonical.TrackCandidate{
		Provider:    ProviderName,
		Track:       track,
		MatchScore:  1.0,
		RawResponse: body,
	}, nil
}

func (p *Provider) Search(ctx context.Context, q adapters.Query) ([]canonical.TrackCandidate, error) {
	var searchQ string
	if q.ISRC != "" {
		searchQ = fmt.Sprintf("isrc:\"%s\"", q.ISRC)
	} else if q.Artist != "" && q.Title != "" {
		searchQ = fmt.Sprintf("artist:\"%s\" track:\"%s\"", query.CleanArtist(q.Artist), query.CleanTitle(q.Title))
	} else if q.Title != "" {
		searchQ = query.CleanTitle(q.Title)
	} else {
		return nil, nil
	}

	reqURL := fmt.Sprintf("%s/search?q=%s&limit=5", BaseURL, url.QueryEscape(searchQ))
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
		return nil, fmt.Errorf("deezer search returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var searchResp provider.DeezerSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, err
	}

	candidates := make([]canonical.TrackCandidate, 0, len(searchResp.Data))
	for _, item := range searchResp.Data {
		cand := canonical.TrackCandidate{
			Provider:    ProviderName,
			Track:       p.normalizeTrack(item),
			MatchScore:  0.90,
			RawResponse: body,
		}
		candidates = append(candidates, cand)
	}

	return candidates, nil
}

func (p *Provider) normalizeTrack(dt provider.DeezerTrackResponse) canonical.Track {
	artists := []canonical.Artist{
		{
			Name: dt.Artist.Name,
			Role: "main",
			IDs:  map[string]string{"deezer": strconv.FormatInt(dt.Artist.ID, 10)},
		},
	}

	for _, contrib := range dt.Contributors {
		if contrib.ID != dt.Artist.ID {
			artists = append(artists, canonical.Artist{
				Name: contrib.Name,
				Role: contrib.Role,
				IDs:  map[string]string{"deezer": strconv.FormatInt(contrib.ID, 10)},
			})
		}
	}

	images := make([]canonical.Image, 0, 3)
	if dt.Album.CoverXL != "" {
		images = append(images, canonical.Image{URL: dt.Album.CoverXL, Width: 1000, Height: 1000, Type: "cover"})
	}
	if dt.Album.CoverBig != "" {
		images = append(images, canonical.Image{URL: dt.Album.CoverBig, Width: 500, Height: 500, Type: "cover"})
	}
	if dt.Album.CoverMedium != "" {
		images = append(images, canonical.Image{URL: dt.Album.CoverMedium, Width: 250, Height: 250, Type: "cover"})
	}

	album := canonical.Album{
		Title:       dt.Album.Title,
		ReleaseDate: dt.Album.ReleaseDate,
		Label:       dt.Album.Label,
		UPC:         dt.Album.UPC,
		Images:      images,
		IDs:         map[string]string{"deezer": strconv.FormatInt(dt.Album.ID, 10)},
	}

	return canonical.Track{
		Title:       dt.Title,
		Artists:     artists,
		Album:       album,
		DurationMS:  dt.Duration * 1000, // convert seconds to ms
		ReleaseDate: dt.ReleaseDate,
		Explicit:    dt.ExplicitLyrics,
		ISRC:        dt.ISRC,
		ISWC:        dt.ISWC,
		PreviewURL:  dt.Preview,
		TrackNumber: dt.TrackPosition,
		DiscNumber:  dt.DiskNumber,
		Label:       dt.Label,
		Barcode:     dt.Barcode,
		PlayCount:   dt.Fans,
		Images:      images,
		IDs:         map[string]string{"deezer": strconv.FormatInt(dt.ID, 10)},
		Sources:     []string{ProviderName},
	}
}
