package applemusic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/pkg/query"
)

var storefrontPattern = regexp.MustCompile(`^[a-z]{2}$`)

type catalogSong struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		Name          string   `json:"name"`
		ArtistName    string   `json:"artistName"`
		AlbumName     string   `json:"albumName"`
		ComposerName  string   `json:"composerName"`
		ReleaseDate   string   `json:"releaseDate"`
		ISRC          string   `json:"isrc"`
		Duration      int64    `json:"durationInMillis"`
		DiscNumber    int      `json:"discNumber"`
		TrackNumber   int      `json:"trackNumber"`
		ContentRating string   `json:"contentRating"`
		GenreNames    []string `json:"genreNames"`
		Artwork       struct {
			URL    string `json:"url"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
		} `json:"artwork"`
		Previews []struct {
			URL string `json:"url"`
		} `json:"previews"`
	} `json:"attributes"`
}

func (p *Provider) catalogRequest(ctx context.Context, path string, params url.Values) ([]byte, error) {
	if !storefrontPattern.MatchString(p.storefront) {
		return nil, fmt.Errorf("invalid Apple storefront")
	}
	u, err := url.Parse(p.catalogBaseURL + "/v1/catalog/" + p.storefront + path)
	if err != nil {
		return nil, err
	}
	u.RawQuery = params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.catalogToken)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Apple catalog request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Apple catalog returned status %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 4<<20))
}

func (p *Provider) fetchCatalogID(ctx context.Context, id string) (*canonical.TrackCandidate, error) {
	if id == "" || strings.ContainsAny(id, "/?#") {
		return nil, fmt.Errorf("invalid Apple song ID")
	}
	data, err := p.catalogRequest(ctx, "/songs/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		Data []catalogSong `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	for _, song := range result.Data {
		if song.ID == id && song.Type == "songs" {
			return &canonical.TrackCandidate{Provider: ProviderName, Track: normalizeCatalogSong(song), MatchScore: 1, RawResponse: data}, nil
		}
	}
	return nil, fmt.Errorf("Apple catalog song not found")
}

func (p *Provider) searchCatalog(ctx context.Context, q adapters.Query) ([]canonical.TrackCandidate, error) {
	var path string
	params := url.Values{}
	if q.ISRC != "" {
		path = "/songs"
		params.Set("filter[isrc]", q.ISRC)
	} else {
		if q.Title == "" {
			return nil, nil
		}
		path = "/search"
		params.Set("types", "songs")
		params.Set("limit", "5")
		params.Set("term", strings.TrimSpace(query.CleanArtist(q.Artist)+" "+query.CleanTitle(q.Title)))
	}
	data, err := p.catalogRequest(ctx, path, params)
	if err != nil {
		return nil, err
	}
	var result struct {
		Data    []catalogSong `json:"data"`
		Results struct {
			Songs struct {
				Data []catalogSong `json:"data"`
			} `json:"songs"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	songs := result.Data
	if path == "/search" {
		songs = result.Results.Songs.Data
	}
	candidates := make([]canonical.TrackCandidate, 0, len(songs))
	for _, song := range songs {
		if song.ID == "" || song.Type != "songs" || q.ISRC != "" && !strings.EqualFold(song.Attributes.ISRC, q.ISRC) {
			continue
		}
		candidates = append(candidates, canonical.TrackCandidate{Provider: ProviderName, Track: normalizeCatalogSong(song), MatchScore: 0.9, RawResponse: data})
	}
	return candidates, nil
}

func normalizeCatalogSong(song catalogSong) canonical.Track {
	a := song.Attributes
	images := []canonical.Image{}
	if a.Artwork.URL != "" {
		art := strings.ReplaceAll(strings.ReplaceAll(a.Artwork.URL, "{w}", "600"), "{h}", "600")
		images = append(images, canonical.Image{URL: art, Width: 600, Height: 600, Type: "cover"})
	}
	credits := []canonical.Credit{}
	if a.ComposerName != "" {
		credits = append(credits, canonical.Credit{Name: a.ComposerName, Roles: []string{"Composer"}})
	}
	preview := ""
	if len(a.Previews) > 0 {
		preview = a.Previews[0].URL
	}
	return canonical.Track{Title: a.Name, Artists: []canonical.Artist{{Name: a.ArtistName, Role: "main"}},
		Album:      canonical.Album{Title: a.AlbumName, ReleaseDate: a.ReleaseDate, Images: images},
		DurationMS: a.Duration, ReleaseDate: a.ReleaseDate, ISRC: a.ISRC,
		Explicit: a.ContentRating == "explicit", Genres: a.GenreNames, Images: images, Credits: credits,
		PreviewURL: preview, TrackNumber: a.TrackNumber, DiscNumber: a.DiscNumber,
		IDs: map[string]string{ProviderName: song.ID}, Sources: []string{ProviderName}}
}
