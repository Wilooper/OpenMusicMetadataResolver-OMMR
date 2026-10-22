package artists

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/ommr/ommr/internal/cache"
	"github.com/ommr/ommr/internal/extractors"
	"github.com/ommr/ommr/internal/providerhealth"
	"github.com/ommr/ommr/pkg/query"
)

var Providers = []string{"applemusic", "deezer", "jiosaavn", "musicbrainz", "ytmusic"}
var numericID = regexp.MustCompile(`^[0-9]{1,18}$`)
var mbID = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)
var channelID = regexp.MustCompile(`^UC[A-Za-z0-9_-]{20,30}$`)
var ErrNotFound = errors.New("artist not found")
var ErrIdentity = errors.New("artist identity mismatch")
var ErrResponse = errors.New("invalid artist response")
var errBudget = errors.New("artist request budget unavailable")

type Caller interface {
	Call(context.Context, string, string, int) (json.RawMessage, error)
}

type Service struct {
	HTTP       *http.Client
	Cache      cache.Cache
	Wait       func(context.Context, string) error
	Available  func(string) bool
	Health     *providerhealth.Manager
	Pool       Caller
	Storefront string
}

func New(wait func(context.Context, string) error, backend cache.Cache) *Service {
	return &Service{HTTP: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, Wait: wait, Cache: backend, Storefront: "us"}
}

func Supported(provider string) bool {
	for _, name := range Providers {
		if provider == name {
			return true
		}
	}
	return false
}
func ValidID(provider, id string) bool {
	switch provider {
	case "deezer", "jiosaavn", "applemusic":
		return numericID.MatchString(id)
	case "musicbrainz":
		return mbID.MatchString(id)
	case "ytmusic":
		return channelID.MatchString(id)
	}
	return false
}

func Validate(q Query) error {
	if q.Limit < 1 || q.Limit > 50 {
		return errors.New("limit must be between 1 and 50")
	}
	if (q.Provider == "") != (q.ID == "") {
		return errors.New("provider and id must be supplied together")
	}
	if q.Provider != "" && (!Supported(q.Provider) || !ValidID(q.Provider, q.ID)) {
		return errors.New("invalid artist provider or native artist ID")
	}
	if q.Provider != "" && len(q.Sources) > 0 {
		return errors.New("use provider/id or sources, not both")
	}
	if q.Artist == "" && q.ID == "" {
		return errors.New("artist name or provider/id is required")
	}
	if len(q.Artist) > 200 || strings.ContainsAny(q.Artist, "\r\n") || strings.Contains(q.Artist, "://") || strings.IndexFunc(q.Artist, unicode.IsControl) >= 0 {
		return errors.New("invalid artist name")
	}
	for _, name := range q.Sources {
		if !Supported(name) {
			return fmt.Errorf("unsupported artist source %q", name)
		}
	}
	return nil
}

type sourceResult struct {
	Profile    *Profile     `json:"profile,omitempty"`
	Candidates []Reference  `json:"candidates,omitempty"`
	Status     SourceStatus `json:"status"`
}

func (s *Service) Resolve(ctx context.Context, q Query) Result {
	names := q.Sources
	if q.Provider != "" {
		names = []string{q.Provider}
	} else if len(names) == 0 {
		names = Providers
	}
	seen := map[string]bool{}
	unique := []string{}
	for _, name := range names {
		if !seen[name] {
			seen[name] = true
			unique = append(unique, name)
		}
	}
	sort.Strings(unique)
	results := make([]sourceResult, len(unique))
	var wg sync.WaitGroup
	for i, name := range unique {
		wg.Add(1)
		go func() { defer wg.Done(); results[i] = s.run(ctx, name, q) }()
	}
	wg.Wait()
	result := Result{Profiles: []Profile{}}
	for _, r := range results {
		result.Sources = append(result.Sources, r.Status)
		if r.Profile != nil {
			result.Profiles = append(result.Profiles, *r.Profile)
		}
		result.Candidates = append(result.Candidates, r.Candidates...)
	}
	return result
}

func (s *Service) run(ctx context.Context, provider string, q Query) sourceResult {
	r := sourceResult{Status: SourceStatus{Provider: provider, Status: "unavailable"}}
	if s.Available != nil && !s.Available(provider) {
		return r
	}
	if provider == "ytmusic" && s.Pool == nil {
		r.Status.Error = "worker_unavailable"
		return r
	}
	signature := ""
	healthName := provider
	if provider == "ytmusic" {
		healthName = "ytmusic_python"
	}
	if s.Health != nil {
		signature = s.Health.Signature()
	}
	keyData, _ := json.Marshal([]any{provider, q.Artist, q.ID, q.Limit, s.Storefront, signature})
	sum := sha256.Sum256(keyData)
	key := "artists:v1:" + hex.EncodeToString(sum[:])
	if !q.BypassCache && s.Cache != nil {
		if data, found, err := s.Cache.Get(ctx, key); err == nil && found && json.Unmarshal(data, &r) == nil {
			r.Status.Cached = true
			return r
		}
	}
	r = sourceResult{Status: SourceStatus{Provider: provider, Status: "error"}}
	var lease uint64
	if s.Health != nil {
		var err error
		lease, err = s.Health.Begin(healthName)
		if err != nil {
			r.Status.Status = "unavailable"
			return r
		}
	}
	refs := []Reference{}
	var err error
	id := q.ID
	if id == "" {
		refs, err = s.search(ctx, provider, q.Artist, q.BypassCache)
		matches := []Reference{}
		for _, ref := range refs {
			if query.CleanArtist(ref.Name) == query.CleanArtist(q.Artist) {
				matches = append(matches, ref)
			}
		}
		refs = deduplicate(matches)
		if err == nil && len(refs) == 1 {
			id = refs[0].ID
		}
	}
	if err == nil && id == "" {
		if len(refs) > 1 {
			r.Status.Status = "ambiguous"
			r.Candidates = refs
		} else {
			r.Status.Status = "not_found"
		}
	} else if err == nil {
		r.Profile, r.Status.Partial, err = s.profile(ctx, provider, id, q.Limit, q.BypassCache)
		if err == nil && (r.Profile == nil || r.Profile.ID != id || r.Profile.Name == "") {
			err = ErrIdentity
			r.Profile = nil
		}
		if err == nil && q.Artist != "" && query.CleanArtist(r.Profile.Name) != query.CleanArtist(q.Artist) {
			err = ErrIdentity
			r.Profile = nil
		}
		if err == nil {
			r.Status.Status = "ok"
			if provider != "ytmusic" {
				r.Profile.ObservedAt = time.Now().UTC()
			}
			r.Profile.IdentityMethod = "unique_exact_name"
			if q.ID != "" {
				r.Profile.IdentityMethod = "exact_id"
			}
		}
	}
	if s.Health != nil {
		if errors.Is(err, extractors.ErrBusy) || errors.Is(err, errBudget) {
			s.Health.Skip(healthName, lease)
		} else {
			s.Health.Finish(ctx, healthName, lease, err)
		}
	}
	if err != nil {
		r.Profile = nil
		switch {
		case errors.Is(err, ErrNotFound):
			r.Status.Status = "not_found"
		case errors.Is(err, ErrIdentity):
			r.Status.Error = "identity_mismatch"
		case errors.Is(err, ErrResponse):
			r.Status.Error = "invalid_response"
		case errors.Is(err, extractors.ErrBusy):
			r.Status.Error = "queue_full"
		case errors.Is(err, errBudget):
			r.Status.Error = "request_budget_exhausted"
		case errors.Is(err, context.Canceled):
			r.Status.Error = "cancelled"
		case errors.Is(err, context.DeadlineExceeded):
			r.Status.Error = "timeout"
		default:
			r.Status.Error = "upstream_unavailable"
		}
		return r
	}
	if s.Cache != nil && ctx.Err() == nil {
		ttl := 5 * time.Minute
		if r.Status.Partial || r.Status.Status != "ok" {
			ttl = 30 * time.Second
		}
		data, _ := json.Marshal(r)
		_ = s.Cache.Set(ctx, key, data, ttl)
	}
	return r
}

func deduplicate(refs []Reference) []Reference {
	seen := map[string]bool{}
	out := []Reference{}
	for _, ref := range refs {
		if ValidID(ref.Provider, ref.ID) && !seen[ref.ID] {
			out = append(out, ref)
			seen[ref.ID] = true
		}
	}
	return out
}

func (s *Service) get(ctx context.Context, provider, endpoint string) (map[string]any, error) {
	u, err := url.Parse(endpoint)
	hosts := map[string]string{"deezer": "api.deezer.com", "jiosaavn": "www.jiosaavn.com", "musicbrainz": "musicbrainz.org", "applemusic": "itunes.apple.com"}
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
	response, err := s.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("artist upstream request failed")
	}
	defer response.Body.Close()
	if response.StatusCode == 404 {
		return nil, ErrNotFound
	}
	if response.StatusCode != 200 {
		return nil, errors.New("artist upstream HTTP error")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (4<<20)+1))
	if err != nil || len(data) > 4<<20 {
		return nil, ErrResponse
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	var value map[string]any
	if decoder.Decode(&value) != nil || value == nil {
		return nil, ErrResponse
	}
	var tail any
	if decoder.Decode(&tail) != io.EOF {
		return nil, ErrResponse
	}
	if apiError(value["error"]) || apiError(value["errors"]) {
		return nil, errors.New("artist upstream API error")
	}
	return value, nil
}

func apiError(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		return strings.TrimSpace(v) != ""
	case json.Number:
		return string(v) != "0"
	case []any:
		return len(v) > 0
	default:
		return true
	}
}
