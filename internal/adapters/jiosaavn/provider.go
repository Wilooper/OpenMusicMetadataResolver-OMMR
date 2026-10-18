package jiosaavn

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/adapters/webdata"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/models/provider"
)

const (
	ProviderName    = "jiosaavn"
	ProviderVersion = "jiosaavn-search-v3-catalog"
)

var SearchURL = "https://www.jiosaavn.com/api.php"

type Provider struct {
	httpClient *http.Client
	web        *webdata.Fallback
}

var _ adapters.ProviderAdapter = (*Provider)(nil)

func New() *Provider {
	return &Provider{
		web:        webdata.NewFallback("www.jiosaavn.com"),
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

	req.Header.Set("Accept", "application/json")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jiosaavn getDetails status %d", resp.StatusCode)
	}
	body, err := webdata.Read(resp, webdata.Request{Provider: ProviderName, Mode: "track", ID: id})
	if err != nil {
		return nil, err
	}

	// getDetails returns a map keyed by song id.
	var m map[string]provider.JioSaavnSongResult
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	detail, ok := m[id]
	if !ok || detail.Song == "" || (detail.ID != "" && detail.ID != id && jioSaavnID(detail) != id) {
		return nil, fmt.Errorf("jiosaavn song %s not found", id)
	}

	var raw map[string]json.RawMessage
	_ = json.Unmarshal(body, &raw)
	track := p.normalize(detail)
	return &canonical.TrackCandidate{
		Provider:    ProviderName,
		Track:       track,
		MatchScore:  1.0,
		RawResponse: raw[id],
	}, nil
}

func (p *Provider) Search(ctx context.Context, q adapters.Query) ([]canonical.TrackCandidate, error) {
	params := url.Values{}
	params.Set("__call", "search.getResults")
	params.Set("api_version", "4")
	params.Set("p", "0")
	params.Set("n", "20")
	params.Set("q", strings.TrimSpace(q.Artist+" "+q.Title))
	params.Set("_format", "json")
	params.Set("_marker", "0")
	params.Set("ctx", "web6dot0")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, SearchURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	req.Header.Set("Accept", "application/json")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jiosaavn search status %d", resp.StatusCode)
	}
	body, err := webdata.Read(resp, webdata.Request{Provider: ProviderName, Mode: "search"})
	if q.ISRC == "" && webdata.Recoverable(err) && p.web != nil {
		body, err = p.web.Try(ctx, "https://www.jiosaavn.com/search/song/"+url.PathEscape(strings.TrimSpace(q.Artist+" "+q.Title)), webdata.Request{Provider: ProviderName, Mode: "search"})
	}
	if err != nil {
		return nil, err
	}

	var search provider.JioSaavnSearchResponse
	if err := json.Unmarshal(body, &search); err != nil {
		return nil, err
	}

	cands := make([]canonical.TrackCandidate, 0, len(search.Results))
	for _, r := range search.Results {
		if strings.TrimSpace(r.Song) == "" || (r.ID == "" && jioSaavnID(r) == "") || (r.Type != "" && r.Type != "song") {
			continue
		}
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
	artists := catalogArtists(r.ArtistMap.Primary, "main")
	if len(artists) == 0 {
		artists = splitNames(r.PrimaryArtists)
	}
	if len(artists) == 0 {
		artists = splitNames(r.Singers)
	}
	if len(artists) == 0 && r.Music != "" {
		artists = []canonical.Artist{{Name: r.Music, Role: "main"}}
	}
	if len(artists) == 0 {
		artists = []canonical.Artist{{Name: "Unknown Artist", Role: "main"}}
	}

	if len(r.ArtistMap.Featured) > 0 {
		artists = append(artists, catalogArtists(r.ArtistMap.Featured, "featured")...)
	} else if len(r.FeaturedArtists) > 0 {
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
		Explicit:       string(r.ExplicitContent) == "1" || string(r.ExplicitContent) == "true",
		Language:       r.Language,
		IDs:            ids,
		Sources:        []string{ProviderName},
		MatchBreakdown: canonical.MatchBreakdown{},
	}

	switch string(r.ExplicitContent) {
	case "1", "true":
		track.Extensions = map[string]json.RawMessage{"explicit_status": json.RawMessage(`"explicit"`)}
	case "0", "false":
		track.Extensions = map[string]json.RawMessage{"explicit_status": json.RawMessage(`"clean"`)}
	}
	if r.AlbumID != "" && r.AlbumID != "null" {
		track.Album.IDs = map[string]string{ProviderName: string(r.AlbumID)}
	}
	if r.CopyrightText != "" {
		track.Copyrights = []canonical.Copyright{{Text: r.CopyrightText}}
	}
	for _, artist := range r.ArtistMap.All {
		role := strings.ToLower(strings.TrimSpace(artist.Role))
		if artist.Name != "" && role != "" && role != "primary_artists" && role != "featured_artists" && role != "singer" {
			track.Credits = append(track.Credits, canonical.Credit{Name: artist.Name, Roles: []string{role}})
		}
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

func (p *Provider) SetFallbackWait(wait func(context.Context) error) {
	if p.web != nil {
		p.web.SetWait(wait)
	}
}

func catalogArtists(values []provider.JioSaavnArtist, role string) []canonical.Artist {
	var result []canonical.Artist
	for _, value := range values {
		if value.Name == "" {
			continue
		}
		artist := canonical.Artist{Name: value.Name, Role: role}
		if value.ID != "" && value.ID != "null" && value.ID != "0" {
			artist.IDs = map[string]string{ProviderName: string(value.ID)}
		}
		result = append(result, artist)
	}
	return result
}
