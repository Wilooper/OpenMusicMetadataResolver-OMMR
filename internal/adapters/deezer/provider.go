package deezer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/adapters/webdata"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/models/provider"
	"github.com/ommr/ommr/pkg/query"
)

const (
	ProviderName    = "deezer"
	ProviderVersion = "deezer-v3-search"
	BaseURL         = "https://api.deezer.com"
)

type Provider struct {
	httpClient *http.Client
	web        *webdata.Fallback
}

var _ adapters.ProviderAdapter = (*Provider)(nil)

func New() *Provider {
	return &Provider{
		web:        webdata.NewFallback("www.deezer.com"),
		httpClient: &http.Client{Timeout: 15 * time.Second},
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

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en")
	req.Header.Set("User-Agent", "OMMR/1.0 (https://github.com/Wilooper/OpenMusicMetadataResolver-OMMR)")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deezer API returned status %d", resp.StatusCode)
	}

	body, err := webdata.Read(resp, webdata.Request{Provider: ProviderName, Mode: "track", ID: id})
	if webdata.Recoverable(err) && p.web != nil && validWebTrackID(id) {
		body, err = p.web.Try(ctx, "https://www.deezer.com/track/"+id, webdata.Request{Provider: ProviderName, Mode: "track", ID: id})
	}
	if err != nil {
		return nil, err
	}

	var trackResp provider.DeezerTrackResponse
	if err := json.Unmarshal(body, &trackResp); err != nil {
		return nil, err
	}
	if trackResp.ID == 0 || strconv.FormatInt(trackResp.ID, 10) != id {
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
		// Live catalog search omits valid recordings with combined field filters.
		// Retrieve bounded text candidates; the resolver still verifies identity.
		searchQ = strings.TrimSpace(q.Artist + " " + q.Title)
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

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en")
	req.Header.Set("User-Agent", "OMMR/1.0 (https://github.com/Wilooper/OpenMusicMetadataResolver-OMMR)")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deezer search returned status %d", resp.StatusCode)
	}

	body, err := webdata.Read(resp, webdata.Request{Provider: ProviderName, Mode: "search"})
	if q.ISRC == "" && webdata.Recoverable(err) && p.web != nil {
		body, err = p.web.Try(ctx, "https://www.deezer.com/search/"+url.PathEscape(strings.TrimSpace(q.Artist+" "+q.Title))+"/track", webdata.Request{Provider: ProviderName, Mode: "search"})
	}
	if err != nil {
		return nil, err
	}

	var searchResp provider.DeezerSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, err
	}

	candidates := make([]canonical.TrackCandidate, 0, len(searchResp.Data))
	for _, item := range searchResp.Data {
		// Text search includes tribute artists whose names can pass fuzzy scoring.
		// Require a reported artist name rather than promoting a near-name cover.
		if q.ISRC == "" && q.Artist != "" && !searchArtist(item, q.Artist) {
			continue
		}
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

func searchArtist(item provider.DeezerTrackResponse, name string) bool {
	name = query.CleanArtist(name)
	if name != "" && query.CleanArtist(item.Artist.Name) == name {
		return true
	}
	for _, artist := range item.Contributors {
		if name != "" && query.CleanArtist(artist.Name) == name {
			return true
		}
	}
	return false
}

func (p *Provider) normalizeTrack(dt provider.DeezerTrackResponse) canonical.Track {
	artists := []canonical.Artist{
		{
			Name: dt.Artist.Name,
			Role: "main",
			IDs:  deezerEntityIDs(dt.Artist.ID),
		},
	}

	for _, contrib := range dt.Contributors {
		if strings.TrimSpace(contrib.Name) != "" && !strings.EqualFold(contrib.Name, dt.Artist.Name) && (contrib.ID <= 0 || contrib.ID != dt.Artist.ID) {
			artists = append(artists, canonical.Artist{
				Name: contrib.Name,
				Role: contrib.Role,
				IDs:  deezerEntityIDs(contrib.ID),
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

	if len(dt.StructuredMetadata) > 0 {
		for i := range images {
			images[i].Width = 0
			images[i].Height = 0
		}
	}
	album := canonical.Album{
		Title:       dt.Album.Title,
		ReleaseDate: dt.Album.ReleaseDate,
		Label:       dt.Album.Label,
		UPC:         dt.Album.UPC,
		Images:      images,
		IDs:         deezerEntityIDs(dt.Album.ID),
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

func validWebTrackID(id string) bool {
	if id == "" || len(id) > 18 {
		return false
	}
	for _, v := range id {
		if v < '0' || v > '9' {
			return false
		}
	}
	return true
}

func (p *Provider) SetFallbackWait(wait func(context.Context) error) {
	if p.web != nil {
		p.web.SetWait(wait)
	}
}

func deezerEntityIDs(id int64) map[string]string {
	if id > 0 {
		return map[string]string{"deezer": strconv.FormatInt(id, 10)}
	}
	return map[string]string{}
}
