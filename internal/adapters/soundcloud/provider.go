package soundcloud

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/models/provider"
	"github.com/ommr/ommr/internal/platformlinks"
)

const (
	ProviderName    = "soundcloud"
	ProviderVersion = "soundcloud-oembed-v1"
)

var OembedURL = "https://soundcloud.com/oembed"

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

func (p *Provider) FetchByID(ctx context.Context, idType string, id string) (*canonical.TrackCandidate, error) {
	if idType != "soundcloud" && idType != "soundcloud_id" {
		return nil, fmt.Errorf("unsupported ID type %q for SoundCloud", idType)
	}

	trackURL := normalizeTrackURL(id)
	reqURL := fmt.Sprintf("%s?format=json&url=%s", OembedURL, url.QueryEscape(trackURL))

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
		return nil, fmt.Errorf("soundcloud oembed status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var oembed provider.SoundCloudOembedResponse
	if err := json.Unmarshal(body, &oembed); err != nil {
		return nil, err
	}
	if oembed.Title == "" {
		return nil, fmt.Errorf("soundcloud track %s not found", id)
	}

	track := p.normalizeOembed(permalinkOf(trackURL), oembed)
	platformlinks.SetProviderURL(&track, ProviderName, trackURL)
	return &canonical.TrackCandidate{
		Provider:    ProviderName,
		Track:       track,
		MatchScore:  1.0,
		RawResponse: body,
	}, nil
}

func (p *Provider) Search(ctx context.Context, q adapters.Query) ([]canonical.TrackCandidate, error) {
	if q.SoundCloudID != "" {
		cand, err := p.FetchByID(ctx, "soundcloud", q.SoundCloudID)
		if err != nil {
			return nil, err
		}
		return []canonical.TrackCandidate{*cand}, nil
	}
	// SoundCloud's search API requires OAuth; text search is intentionally
	// not supported in zero-key mode.
	return nil, nil
}

func (p *Provider) normalizeOembed(permalink string, oembed provider.SoundCloudOembedResponse) canonical.Track {
	title := oembed.Title
	artist := oembed.AuthorName

	// SoundCloud oEmbed titles follow the "Track by Artist" convention.
	if strings.Contains(oembed.Title, " by ") {
		parts := strings.SplitN(oembed.Title, " by ", 2)
		title = strings.TrimSpace(parts[0])
		artist = strings.TrimSpace(parts[1])
	}

	images := make([]canonical.Image, 0, 1)
	if oembed.ThumbnailURL != "" {
		images = append(images, canonical.Image{
			URL:    oembed.ThumbnailURL,
			Width:  int(oembed.Width),
			Height: oembed.Height,
			Type:   "cover",
		})
	}

	track := canonical.Track{
		Title:   title,
		Artists: []canonical.Artist{{Name: artist, Role: "main"}},
		Album: canonical.Album{
			Title:  title,
			Images: images,
		},
		Images:  images,
		IDs:     map[string]string{"soundcloud": permalink},
		Sources: []string{ProviderName},
	}
	if id := nativeTrackID(oembed.HTML); id != "" {
		track.IDs["soundcloud_track_id"] = id
	}
	return track
}

var iframeSource = regexp.MustCompile(`(?i)<iframe\b[^>]*\bsrc\s*=\s*["']([^"']+)["']`)
var numericTrackPath = regexp.MustCompile(`^/tracks/([0-9]+)$`)

func nativeTrackID(embedHTML string) string {
	for _, match := range iframeSource.FindAllStringSubmatch(embedHTML, -1) {
		widget, err := url.Parse(html.UnescapeString(match[1]))
		if err != nil || widget.Scheme != "https" || widget.Host != "w.soundcloud.com" || widget.User != nil {
			continue
		}
		values := widget.Query()["url"]
		if len(values) != 1 {
			continue
		}
		api, err := url.Parse(values[0])
		if err != nil || api.Scheme != "https" || api.Host != "api.soundcloud.com" || api.User != nil || api.RawQuery != "" {
			continue
		}
		if match := numericTrackPath.FindStringSubmatch(api.Path); len(match) == 2 {
			return match[1]
		}
	}
	return ""
}

// normalizeTrackURL converts a SoundCloud permalink or URL into a full URL.
func normalizeTrackURL(id string) string {
	id = strings.TrimSpace(id)
	if strings.HasPrefix(id, "http://") || strings.HasPrefix(id, "https://") {
		return id
	}
	return "https://soundcloud.com/" + strings.TrimPrefix(id, "/")
}

func permalinkOf(trackURL string) string {
	parsed, err := url.Parse(trackURL)
	if err != nil {
		return trackURL
	}
	return strings.TrimPrefix(parsed.Path, "/")
}
