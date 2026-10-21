package albums

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/ommr/ommr/internal/cache"
	"github.com/ommr/ommr/internal/providerhealth"
)

var ErrInput = errors.New("invalid album provider or native album ID (MusicBrainz requires a release ID)")
var ErrResponse = errors.New("invalid album response")
var ErrNotFound = errors.New("album not found")
var ErrUnavailable = errors.New("album provider unavailable")
var errBudget = errors.New("album request budget exhausted")
var numericID = regexp.MustCompile(`^[1-9][0-9]{0,17}$`)
var releaseID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func Supported(provider string) bool {
	switch provider {
	case "applemusic", "deezer", "jiosaavn", "musicbrainz":
		return true
	}
	return false
}
func Validate(q Query) error {
	if !Supported(q.Provider) || (q.Provider == "musicbrainz" && !releaseID.MatchString(q.ID)) || (q.Provider != "musicbrainz" && !numericID.MatchString(q.ID)) {
		return ErrInput
	}
	if q.Limit < 1 || q.Limit > MaxTracks {
		return fmt.Errorf("limit must be between 1 and %d", MaxTracks)
	}
	return nil
}

type Service struct {
	HTTP       *http.Client
	Cache      cache.Cache
	Wait       func(context.Context, string) error
	Available  func(string) bool
	Health     *providerhealth.Manager
	Storefront string
}

func New(wait func(context.Context, string) error, backend cache.Cache) *Service {
	return &Service{HTTP: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, Cache: backend, Wait: wait, Storefront: "us"}
}
func (s *Service) Resolve(ctx context.Context, q Query) (Result, error) {
	result := Result{Source: &Diagnostics{Status: "error"}}
	if err := Validate(q); err != nil {
		return result, err
	}
	if (s.Available != nil && !s.Available(q.Provider)) || (s.Health != nil && !s.Health.Available(q.Provider)) {
		return result, ErrUnavailable
	}
	signature := ""
	if s.Health != nil {
		signature = s.Health.Signature()
	}
	key := fmt.Sprintf("albums:v5:%x", sha256.Sum256([]byte(fmt.Sprintf("%s/%s/%d/%s/%s", q.Provider, q.ID, q.Limit, s.Storefront, signature))))
	if !q.BypassCache && s.Cache != nil {
		if data, found, err := s.Cache.Get(ctx, key); err == nil && found && json.Unmarshal(data, &result) == nil && result.Album != nil && result.Source != nil {
			result.Source.Cached = true
			return result, nil
		}
	}
	result = Result{Source: &Diagnostics{Status: "error"}}
	var lease uint64
	if s.Health != nil {
		var err error
		lease, err = s.Health.Begin(q.Provider)
		if err != nil {
			return result, ErrUnavailable
		}
	}
	album, err := s.fetch(ctx, q)
	if album != nil && (album.ID != q.ID || album.Title == "") {
		album = nil
		err = ErrResponse
	}
	if s.Health != nil {
		if errors.Is(err, errBudget) {
			s.Health.Skip(q.Provider, lease)
		} else {
			s.Health.Finish(ctx, q.Provider, lease, err)
		}
	}
	if album == nil {
		return result, err
	}
	if album.ID != q.ID || album.Title == "" {
		return result, ErrResponse
	}
	attachGraph(album)
	album.ObservedAt = time.Now().UTC()
	album.TrackList.Returned = len(album.Tracks)
	if album.Tracks == nil {
		album.Tracks = []Track{}
	}
	if err != nil {
		album.TrackList.Complete = false
		album.TrackList.Reason = "upstream_error"
		result.Source.Error = "track_list_unavailable"
	}
	result.Album = album
	result.Source.Status = "ok"
	if !album.TrackList.Complete {
		result.Source.Status = "partial"
	}
	if s.Cache != nil && ctx.Err() == nil {
		ttl := 5 * time.Minute
		if !album.TrackList.Complete {
			ttl = 30 * time.Second
		}
		data, _ := json.Marshal(result)
		_ = s.Cache.Set(ctx, key, data, ttl)
	}
	return result, nil
}
func (s *Service) get(ctx context.Context, provider, endpoint string) (map[string]any, error) {
	hosts := map[string]string{"applemusic": "itunes.apple.com", "deezer": "api.deezer.com", "jiosaavn": "www.jiosaavn.com", "musicbrainz": "musicbrainz.org"}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Host != hosts[provider] || u.User != nil {
		return nil, ErrResponse
	}
	if s.Wait != nil {
		if err := s.Wait(ctx, provider); err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, errBudget
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, ErrResponse
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en")
	req.Header.Set("User-Agent", "OMMR/1.0.0 (https://github.com/Wilooper/OpenMusicMetadataResolver-OMMR)")
	res, err := s.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("album upstream request failed")
	}
	defer res.Body.Close()
	if res.StatusCode == 404 {
		return nil, ErrNotFound
	}
	if res.StatusCode != 200 {
		return nil, errors.New("album upstream HTTP error")
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, (4<<20)+1))
	if err != nil || len(data) > 4<<20 {
		return nil, ErrResponse
	}
	d := json.NewDecoder(strings.NewReader(string(data)))
	d.UseNumber()
	var m map[string]any
	if d.Decode(&m) != nil || m == nil {
		return nil, ErrResponse
	}
	var tail any
	if d.Decode(&tail) != io.EOF {
		return nil, ErrResponse
	}
	if v := m["error"]; v != nil && v != false && v != "" {
		return nil, ErrResponse
	}
	if v := m["errors"]; v != nil && v != false && v != "" {
		return nil, ErrResponse
	}
	return m, nil
}
