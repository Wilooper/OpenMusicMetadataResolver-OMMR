package catalogapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/platformlinks"
)

type Config struct {
	Name         string
	Version      string
	BaseURL      string
	Token        string
	APIKey       string
	FetchPath    func(string) (string, error)
	FetchRequest func(string) (string, string, []byte, error)
	Search       func(adapters.Query) (string, string, []byte, error)
	ExtraHeader  map[string]string
}

type Provider struct {
	cfg    Config
	client *http.Client
}

var _ adapters.ProviderAdapter = (*Provider)(nil)

func New(cfg Config) *Provider {
	return &Provider{cfg: cfg, client: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (p *Provider) Name() string    { return p.cfg.Name }
func (p *Provider) Version() string { return p.cfg.Version }

func (p *Provider) FetchByID(ctx context.Context, idType, id string) (*canonical.TrackCandidate, error) {
	if strings.ToLower(idType) != p.cfg.Name && strings.ToLower(idType) != p.cfg.Name+"_id" {
		return nil, fmt.Errorf("unsupported ID type %q for %s", idType, p.cfg.Name)
	}
	method, path, body := http.MethodGet, "", []byte(nil)
	var err error
	if p.cfg.FetchRequest != nil {
		method, path, body, err = p.cfg.FetchRequest(id)
	} else {
		path, err = p.cfg.FetchPath(id)
	}
	if err != nil {
		return nil, err
	}
	candidate, err := p.requestCandidate(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	returnedID := candidate.Track.IDs[p.cfg.Name]
	if returnedID != "" && returnedID != id {
		return nil, fmt.Errorf("%s returned track ID %q for requested ID %q", p.cfg.Name, returnedID, id)
	}
	if candidate.Track.IDs == nil {
		candidate.Track.IDs = make(map[string]string)
	}
	if returnedID == "" {
		candidate.Track.IDs[p.cfg.Name] = id
	}
	return candidate, nil
}

func (p *Provider) Search(ctx context.Context, q adapters.Query) ([]canonical.TrackCandidate, error) {
	if p.cfg.Search == nil {
		return nil, nil
	}
	method, path, body, err := p.cfg.Search(q)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil
	}
	req, err := p.newRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s API returned status %d", p.cfg.Name, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	items := findTrackItems(root)
	out := make([]canonical.TrackCandidate, 0, len(items))
	for _, item := range items {
		track := normalize(item, p.cfg.Name)
		if track.Title == "" {
			continue
		}
		out = append(out, canonical.TrackCandidate{Provider: p.cfg.Name, Track: track, MatchScore: 0.75, RawResponse: data})
	}
	return out, nil
}

func (p *Provider) requestCandidate(ctx context.Context, method, path string, body []byte) (*canonical.TrackCandidate, error) {
	req, err := p.newRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s API returned status %d", p.cfg.Name, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	item := findSingleTrack(root)
	if item == nil {
		return nil, errors.New("provider response contained no track")
	}
	track := normalize(item, p.cfg.Name)
	if track.Title == "" {
		return nil, errors.New("provider response contained an empty track title")
	}
	return &canonical.TrackCandidate{Provider: p.cfg.Name, Track: track, MatchScore: 1, RawResponse: data}, nil
}

func (p *Provider) newRequest(ctx context.Context, method, path string, body []byte) (*http.Request, error) {
	base := strings.TrimRight(p.cfg.BaseURL, "/")
	if base == "" {
		return nil, fmt.Errorf("%s provider is not configured", p.cfg.Name)
	}
	requestURL := base + "/" + strings.TrimLeft(path, "/")
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if p.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+p.cfg.Token)
	}
	if p.cfg.APIKey != "" {
		req.Header.Set("x-api-key", p.cfg.APIKey)
	}
	for key, value := range p.cfg.ExtraHeader {
		if value != "" {
			req.Header.Set(key, value)
		}
	}
	return req, nil
}

func findSingleTrack(root any) map[string]any {
	for _, obj := range walkObjects(root) {
		candidate := obj
		if attributes, ok := obj["attributes"].(map[string]any); ok {
			candidate = mergeMaps(attributes, obj)
			candidate["_relationships"] = obj["relationships"]
			candidate["_included"] = objectArray(root, "included")
		}
		if firstString(candidate, "title", "name") != "" && (candidate["artists"] != nil || candidate["artist"] != nil || candidate["album"] != nil || candidate["_relationships"] != nil || strings.Contains(strings.ToLower(firstString(obj, "type")), "track")) {
			return candidate
		}
	}
	return nil
}

func findTrackItems(root any) []map[string]any {
	var found []map[string]any
	for _, obj := range walkObjects(root) {
		if arr, ok := obj["items"].([]any); ok {
			for _, entry := range arr {
				if item, ok := entry.(map[string]any); ok && firstString(item, "title", "name") != "" {
					found = append(found, item)
				}
			}
		}
		if arr, ok := obj["tracks"].([]any); ok {
			for _, entry := range arr {
				if item, ok := entry.(map[string]any); ok && firstString(item, "title", "name") != "" {
					found = append(found, item)
				}
			}
		}
		if arr, ok := obj["entities"].([]any); ok {
			for _, entry := range arr {
				if item, ok := entry.(map[string]any); ok && firstString(item, "title", "name") != "" {
					found = append(found, item)
				}
			}
		}
	}
	return found
}

func walkObjects(root any) []map[string]any {
	var out []map[string]any
	var walk func(any)
	walk = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			out = append(out, typed)
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(root)
	return out
}

func normalize(obj map[string]any, provider string) canonical.Track {
	if data, ok := obj["attributes"].(map[string]any); ok {
		obj = mergeMaps(data, obj)
	}
	title := firstString(obj, "title", "name")
	artists := normalizeArtists(obj["artists"], provider)
	if len(artists) == 0 {
		artists = normalizeArtists(obj["artist"], provider)
	}
	if len(artists) == 0 {
		artists = normalizeRelationshipArtists(obj, provider)
	}
	albumObj, _ := obj["album"].(map[string]any)
	album := canonical.Album{Title: firstString(albumObj, "title", "name"), ReleaseDate: firstString(albumObj, "releaseDate", "release_date", "releaseDatePrecision")}
	images := normalizeImages(obj["images"], "cover")
	if len(images) == 0 {
		images = normalizeImages(albumObj["images"], "cover")
	}
	if len(images) == 0 {
		images = normalizeImages(albumObj["image"], "cover")
	}
	if len(images) == 0 {
		images = normalizeImages(obj["art"], "cover")
	}
	if len(images) == 0 {
		images = normalizeImages(obj["artwork"], "cover")
	}
	album.Images = images
	duration := number(obj["duration_ms"])
	if duration == 0 {
		duration = number(obj["durationMs"])
	}
	if duration == 0 {
		duration = number(obj["duration"]) * 1000
	}
	releaseDate := firstString(obj, "releaseDate", "release_date")
	if releaseDate == "" {
		releaseDate = album.ReleaseDate
	}
	track := canonical.Track{Title: title, Artists: artists, Album: album, DurationMS: duration, ReleaseDate: releaseDate, ISRC: firstString(obj, "isrc", "ISRC"), Images: images, IDs: map[string]string{}, Sources: []string{provider}}
	id := firstString(obj, "id", "trackId")
	if id != "" {
		track.IDs[provider] = id
	}
	platformlinks.SetProviderURL(&track, provider, firstString(obj, "url", "webUrl", "shareUrl", "permalink_url"))
	if external, ok := obj["externalLinks"].([]any); ok {
		for _, item := range external {
			if link, ok := item.(map[string]any); ok {
				platformlinks.SetProviderURL(&track, provider, firstString(link, "href", "url"))
			}
		}
	}
	if preview := firstString(obj, "previewUrl", "preview_url"); preview != "" {
		track.PreviewURL = preview
	}
	if label := firstString(obj, "label"); label != "" {
		track.Label = label
	}
	if credits, ok := obj["credits"].([]any); ok {
		for _, raw := range credits {
			if c, ok := raw.(map[string]any); ok {
				name := firstString(c, "name", "artistName")
				roles := stringSlice(c["roles"])
				if role := firstString(c, "role"); role != "" {
					roles = append(roles, role)
				}
				if name != "" {
					track.Credits = append(track.Credits, canonical.Credit{Name: name, Roles: roles})
				}
			}
		}
	}
	return track
}

func normalizeArtists(raw any, provider string) []canonical.Artist {
	items, ok := raw.([]any)
	if !ok {
		if item, ok := raw.(map[string]any); ok {
			items = []any{item}
		} else if name, ok := raw.(string); ok && name != "" {
			return []canonical.Artist{{Name: name, Role: "main"}}
		}
	}
	var artists []canonical.Artist
	for _, item := range items {
		if obj, ok := item.(map[string]any); ok {
			name := firstString(obj, "name", "title")
			if name != "" {
				artist := canonical.Artist{Name: name, Role: "main"}
				if id := firstString(obj, "id"); id != "" {
					artist.IDs = map[string]string{provider: id}
				}
				artists = append(artists, artist)
			}
		}
	}
	return artists
}

func normalizeRelationshipArtists(obj map[string]any, provider string) []canonical.Artist {
	relationships, _ := obj["_relationships"].(map[string]any)
	artistRelation, _ := relationships["artists"].(map[string]any)
	refs, _ := artistRelation["data"].([]any)
	included, _ := obj["_included"].([]any)
	var artists []canonical.Artist
	for _, ref := range refs {
		refObj, _ := ref.(map[string]any)
		id := firstString(refObj, "id")
		name := firstString(refObj, "name")
		for _, item := range included {
			inc, _ := item.(map[string]any)
			if id != "" && firstString(inc, "id") == id {
				attrs, _ := inc["attributes"].(map[string]any)
				name = firstString(attrs, "name", "title")
				if name == "" {
					name = firstString(inc, "name", "title")
				}
				break
			}
		}
		if name != "" {
			artist := canonical.Artist{Name: name, Role: "main"}
			if id != "" {
				artist.IDs = map[string]string{provider: id}
			}
			artists = append(artists, artist)
		}
	}
	return artists
}

func normalizeImages(raw any, imageType string) []canonical.Image {
	items, ok := raw.([]any)
	if !ok {
		if item, ok := raw.(map[string]any); ok {
			if u := firstString(item, "url", "src", "href"); u != "" {
				items = []any{item}
			} else {
				for _, size := range []string{"large", "extralarge", "xl", "medium", "small", "thumbnail"} {
					if u := firstString(item, size); u != "" {
						items = append(items, map[string]any{"url": u})
					}
				}
			}
		} else if u, ok := raw.(string); ok && u != "" {
			return []canonical.Image{{URL: u, Type: imageType}}
		}
	}
	var images []canonical.Image
	for _, item := range items {
		if obj, ok := item.(map[string]any); ok {
			u := firstString(obj, "url", "src", "href")
			if u != "" {
				images = append(images, canonical.Image{URL: u, Width: int(number(obj["width"])), Height: int(number(obj["height"])), Type: imageType})
			}
		}
	}
	return images
}

func objectArray(root any, key string) []any {
	if obj, ok := root.(map[string]any); ok {
		if values, ok := obj[key].([]any); ok {
			return values
		}
		if data, ok := obj["data"].(map[string]any); ok {
			if values, ok := data[key].([]any); ok {
				return values
			}
		}
	}
	return nil
}

func firstString(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		if s, ok := obj[key].(string); ok && s != "" {
			return s
		}
		if v, ok := obj[key].(float64); ok && v != 0 {
			return strconv.FormatInt(int64(v), 10)
		}
	}
	return ""
}

func number(value any) int64 {
	switch v := value.(type) {
	case float64:
		return int64(v)
	case string:
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	}
	return 0
}
func stringSlice(value any) []string {
	if values, ok := value.([]any); ok {
		out := make([]string, 0, len(values))
		for _, v := range values {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
func mergeMaps(primary, fallback map[string]any) map[string]any {
	result := make(map[string]any, len(primary)+len(fallback))
	for k, v := range fallback {
		result[k] = v
	}
	for k, v := range primary {
		result[k] = v
	}
	return result
}

func EscapedID(id string) string { return url.PathEscape(strings.TrimSpace(id)) }
