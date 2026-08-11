package ytmusic

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
	ProviderName    = "ytmusic"
	ProviderVersion = "ytmusic-oembed-v1"
	OembedURL       = "https://www.youtube.com/oembed"
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
	if idType != "ytmusic" && idType != "youtube" && idType != "youtube_id" {
		return nil, fmt.Errorf("unsupported ID type %q for YTMusic", idType)
	}

	watchURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", url.QueryEscape(id))
	reqURL := fmt.Sprintf("%s?url=%s&format=json", OembedURL, url.QueryEscape(watchURL))

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
		return nil, fmt.Errorf("ytmusic oembed returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var oembed provider.YTOembedResponse
	if err := json.Unmarshal(body, &oembed); err != nil {
		return nil, err
	}

	track := p.normalizeOembed(id, oembed)
	return &canonical.TrackCandidate{
		Provider:    ProviderName,
		Track:       track,
		MatchScore:  1.0,
		RawResponse: body,
	}, nil
}

func (p *Provider) Search(ctx context.Context, q adapters.Query) ([]canonical.TrackCandidate, error) {
	// YouTube oEmbed does not support arbitrary text search without video ID;
	// If YouTube ID is provided in query, fetch by ID.
	if q.YouTubeID != "" {
		cand, err := p.FetchByID(ctx, "youtube", q.YouTubeID)
		if err != nil {
			return nil, err
		}
		return []canonical.TrackCandidate{*cand}, nil
	}
	return nil, nil
}

func (p *Provider) normalizeOembed(videoID string, oembed provider.YTOembedResponse) canonical.Track {
	cleanedTitle := query.CleanTitle(oembed.Title)
	cleanedArtist := query.CleanArtist(oembed.AuthorName)

	// If title is formatted as "Artist - Song", parse out artist and title
	title := oembed.Title
	artistName := oembed.AuthorName
	if strings.Contains(oembed.Title, " - ") {
		parts := strings.SplitN(oembed.Title, " - ", 2)
		artistName = strings.TrimSpace(parts[0])
		title = strings.TrimSpace(parts[1])
	}

	artists := []canonical.Artist{
		{
			Name: artistName,
			Role: "main",
		},
	}

	images := make([]canonical.Image, 0, 1)
	if oembed.ThumbnailURL != "" {
		images = append(images, canonical.Image{
			URL:    oembed.ThumbnailURL,
			Width:  oembed.Width,
			Height: oembed.Height,
			Type:   "thumbnail",
		})
	}

	_ = cleanedTitle
	_ = cleanedArtist

	return canonical.Track{
		Title:   title,
		Artists: artists,
		Album: canonical.Album{
			Title:  title,
			Images: images,
		},
		Images:  images,
		IDs:     map[string]string{"ytmusic": videoID},
		Sources: []string{ProviderName},
	}
}
