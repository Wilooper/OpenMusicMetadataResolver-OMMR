package spotify

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/models/provider"
)

var (
	spotifyNextDataRegex = regexp.MustCompile(`<script id="__NEXT_DATA__" type="application/json">([^<]+)</script>`)
	spotifyEmbedRegex    = regexp.MustCompile(`<script id="resource" type="application/json">([^<]+)</script>`)
)

// ParseOembed converts Spotify oEmbed response into a canonical Track.
func ParseOembed(spotifyID string, oembed provider.SpotifyOembedResponse) canonical.Track {
	title := oembed.Title
	artist := oembed.AuthorName

	if strings.Contains(oembed.Title, " by ") {
		parts := strings.SplitN(oembed.Title, " by ", 2)
		title = strings.TrimSpace(parts[0])
		artist = strings.TrimSpace(parts[1])
	}

	artists := []canonical.Artist{
		{Name: artist, Role: "main"},
	}

	images := make([]canonical.Image, 0, 1)
	if oembed.ThumbnailURL != "" {
		images = append(images, canonical.Image{
			URL:    oembed.ThumbnailURL,
			Width:  oembed.Width,
			Height: oembed.Height,
			Type:   "cover",
		})
	}

	return canonical.Track{
		Title:   title,
		Artists: artists,
		Album: canonical.Album{
			Title:  title,
			Images: images,
		},
		Images:  images,
		IDs:     map[string]string{"spotify": spotifyID},
		Sources: []string{ProviderName},
	}
}

// ParseEmbedHTML extracts embedded JSON state from open.spotify.com/embed/track/{id} HTML.
// It prefers the __NEXT_DATA__ payload (rich metadata) and falls back to the
// legacy "resource" script tag.
func ParseEmbedHTML(spotifyID string, htmlContent string) (*canonical.Track, error) {
	if track, err := ParseEmbedNextData(spotifyID, []byte(htmlContent)); err == nil && track != nil && track.Title != "" {
		return track, nil
	}

	matches := spotifyEmbedRegex.FindStringSubmatch(htmlContent)
	if len(matches) < 2 {
		return nil, nil
	}

	var stateData map[string]interface{}
	if err := json.Unmarshal([]byte(matches[1]), &stateData); err != nil {
		return nil, err
	}

	title, _ := stateData["name"].(string)
	if title == "" {
		return nil, nil
	}

	return &canonical.Track{
		Title:   title,
		IDs:     map[string]string{"spotify": spotifyID},
		Sources: []string{ProviderName},
	}, nil
}

// ParseEmbedNextData extracts the rich track entity from the __NEXT_DATA__
// JSON payload embedded in open.spotify.com/embed/track/{id}.
func ParseEmbedNextData(spotifyID string, body []byte) (*canonical.Track, error) {
	matches := spotifyNextDataRegex.FindStringSubmatch(string(body))
	if len(matches) < 2 {
		return nil, nil
	}

	var next provider.SpotifyEmbedNextData
	if err := json.Unmarshal([]byte(matches[1]), &next); err != nil {
		return nil, err
	}

	entity := next.Props.PageProps.State.Data.Entity
	if entity.Name == "" {
		return nil, nil
	}

	artists := make([]canonical.Artist, 0, len(entity.Artists))
	for _, a := range entity.Artists {
		artist := canonical.Artist{Name: a.Name, Role: "main"}
		if a.ID != "" {
			artist.IDs = map[string]string{"spotify": a.ID}
		}
		artists = append(artists, artist)
	}

	images := make([]canonical.Image, 0, len(entity.Album.Images))
	for _, img := range entity.Album.Images {
		images = append(images, canonical.Image{URL: img.URL, Width: img.Width, Height: img.Height, Type: "cover"})
	}

	copyrights := make([]canonical.Copyright, 0, len(entity.Album.Copyright))
	for _, c := range entity.Album.Copyright {
		copyrights = append(copyrights, canonical.Copyright{Type: c.Type, Text: c.Text})
	}

	durationMS := entity.DurationMS
	if durationMS == 0 {
		durationMS = entity.Duration
	}

	album := canonical.Album{
		Title:       entity.Album.Name,
		ReleaseDate: entity.Album.ReleaseDate.ISOString,
		Label:       entity.Album.Label,
		TotalTracks: entity.Album.TotalTracks,
		Copyrights:  copyrights,
		Images:      images,
	}
	if entity.Album.ID != "" {
		album.IDs = map[string]string{"spotify": entity.Album.ID}
	}

	return &canonical.Track{
		Title:       entity.Name,
		Artists:     artists,
		Album:       album,
		DurationMS:  durationMS,
		ReleaseDate: album.ReleaseDate,
		Explicit:    entity.Explicit,
		ISRC:        entity.ISRC,
		PreviewURL:  entity.PreviewURL,
		TrackNumber: entity.TrackNumber,
		DiscNumber:  entity.DiscNumber,
		Label:       entity.Album.Label,
		Copyrights:  copyrights,
		Images:      images,
		IDs:         map[string]string{"spotify": spotifyID},
		Sources:     []string{ProviderName},
	}, nil
}
