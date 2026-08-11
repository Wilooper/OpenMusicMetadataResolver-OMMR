package spotify

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/models/provider"
)

var (
	spotifyEmbedStateRegex = regexp.MustCompile(`<script id="resource" type="application/json">([^<]+)</script>`)
	spotifyEntityRegex     = regexp.MustCompile(`"type":"track"`)
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
func ParseEmbedHTML(spotifyID string, htmlContent string) (*canonical.Track, error) {
	matches := spotifyEmbedStateRegex.FindStringSubmatch(htmlContent)
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
		Title: title,
		IDs:   map[string]string{"spotify": spotifyID},
		Sources: []string{ProviderName},
	}, nil
}
