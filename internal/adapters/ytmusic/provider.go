package ytmusic

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
	ProviderName    = "ytmusic"
	ProviderVersion = "ytmusic-innertube-v1"
	OembedURL       = "https://www.youtube.com/oembed"
)

type Provider struct {
	httpClient *http.Client
	innerTube  *InnerTube
}

var _ adapters.ProviderAdapter = (*Provider)(nil)

func New() *Provider {
	client := &http.Client{Timeout: 8 * time.Second}
	return &Provider{
		httpClient: client,
		innerTube:  NewInnerTube(client),
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

	// Preferred path: InnerTube player metadata (exact title, channel, duration).
	if player, err := p.innerTube.FetchPlayer(ctx, id); err == nil {
		track := p.normalizePlayer(id, *player)
		return &canonical.TrackCandidate{
			Provider:   ProviderName,
			Track:      track,
			MatchScore: 1.0,
		}, nil
	}

	// Fallback path: public oEmbed endpoint.
	return p.fetchByOembed(ctx, id)
}

func (p *Provider) fetchByOembed(ctx context.Context, id string) (*canonical.TrackCandidate, error) {
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
	// If YouTube ID is provided, fetch by ID.
	if q.YouTubeID != "" {
		cand, err := p.FetchByID(ctx, "youtube", q.YouTubeID)
		if err != nil {
			return nil, err
		}
		return []canonical.TrackCandidate{*cand}, nil
	}

	searchTerm := q.Title
	if q.Artist != "" && q.Title != "" {
		searchTerm = fmt.Sprintf("%s %s", query.CleanArtist(q.Artist), query.CleanTitle(q.Title))
	} else if q.Artist != "" {
		searchTerm = query.CleanArtist(q.Artist)
	}
	if searchTerm == "" {
		return nil, nil
	}

	videos, err := p.innerTube.SearchYouTube(ctx, searchTerm, 8)
	if err != nil {
		return nil, err
	}

	candidates := make([]canonical.TrackCandidate, 0, len(videos))
	for _, v := range videos {
		if v.VideoID == "" {
			continue
		}
		candidates = append(candidates, canonical.TrackCandidate{
			Provider:   ProviderName,
			Track:      p.innerTube.parseVideo(v),
			MatchScore: 0.90,
		})
	}
	return candidates, nil
}

func (p *Provider) normalizePlayer(videoID string, player provider.YTPlayerResponse) canonical.Track {
	details := player.VideoDetails

	title := details.Title
	artistName := CleanChannelName(details.Author)
	if artistName == "" {
		artistName = details.Author
	}

	durationMS := int64(0)
	if details.LengthSeconds != "" {
		if secs, err := strconv.ParseInt(details.LengthSeconds, 10, 64); err == nil {
			durationMS = secs * 1000
		}
	}

	images := make([]canonical.Image, 0, len(details.Thumbnail.Thumbnails))
	for _, th := range details.Thumbnail.Thumbnails {
		images = append(images, canonical.Image{URL: th.URL, Width: th.Width, Height: th.Height, Type: "thumbnail"})
	}

	return canonical.Track{
		Title:      title,
		Artists:    []canonical.Artist{{Name: artistName, Role: "main"}},
		DurationMS: durationMS,
		Images:     images,
		IDs:        map[string]string{"ytmusic": videoID},
		Sources:    []string{ProviderName},
	}
}

func (p *Provider) normalizeOembed(videoID string, oembed provider.YTOembedResponse) canonical.Track {
	cleanedTitle := query.CleanTitle(oembed.Title)
	cleanedArtist := query.CleanArtist(oembed.AuthorName)

	// If title is formatted as "Artist - Song", parse out artist and title
	title := oembed.Title
	artistName := CleanChannelName(oembed.AuthorName)
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
