package diagnostics

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/adapters/webdata"
	"github.com/ommr/ommr/internal/models/canonical"
	"golang.org/x/time/rate"
)

const UserAgent = "OMMR/1.0.0 (https://github.com/Wilooper/OpenMusicMetadataResolver-OMMR)"
const maxBody = 8 << 20

type Event struct {
	Phase       string `json:"phase"`
	Method      string `json:"method"`
	URL         string `json:"url"`
	Status      int    `json:"status,omitempty"`
	Protocol    string `json:"protocol,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Server      string `json:"server,omitempty"`
	Bytes       int    `json:"bytes,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
	Kind        string `json:"kind"`
	ElapsedMS   int64  `json:"elapsed_ms"`
}

type Transport struct {
	Base     http.RoundTripper
	mu       sync.Mutex
	events   []Event
	limiters map[string]*rate.Limiter
}

type phaseKey struct{}

func phase(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, phaseKey{}, name)
}

// Limit by service, so the API and public-site fallback share one request budget.
func service(host string) string {
	switch host {
	case "api.deezer.com", "www.deezer.com":
		return "deezer"
	case "www.jiosaavn.com":
		return "jiosaavn"
	case "musicbrainz.org":
		return "musicbrainz"
	default:
		return host
	}
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	name, _ := req.Context().Value(phaseKey{}).(string)
	event := Event{Phase: name, Method: req.Method, URL: safeURL(req.URL)}
	t.mu.Lock()
	if t.limiters == nil {
		t.limiters = map[string]*rate.Limiter{}
	}
	key := service(req.URL.Hostname())
	limiter := t.limiters[key]
	if limiter == nil {
		limiter = rate.NewLimiter(1, 1)
		t.limiters[key] = limiter
	}
	t.mu.Unlock()
	defer func() {
		event.ElapsedMS = time.Since(start).Milliseconds()
		t.mu.Lock()
		t.events = append(t.events, event)
		t.mu.Unlock()
	}()
	if err := limiter.Wait(req.Context()); err != nil {
		event.Kind = "deadline_or_cancelled"
		return nil, err
	}
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
		if base == t {
			event.Kind = "configuration_error"
			return nil, errors.New("diagnostic transport needs its base transport")
		}
	}
	response, err := base.RoundTrip(req)
	if err != nil {
		event.Kind = errorKind(err)
		return nil, err
	}
	event.Status, event.Protocol = response.StatusCode, response.Proto
	event.ContentType = safeHeader(response.Header.Get("Content-Type"))
	event.Server = safeHeader(response.Header.Get("Server"))
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBody+1))
	_ = response.Body.Close()
	if err != nil {
		event.Kind = "read_error"
		return nil, errors.New("diagnostic response read failed")
	}
	event.Bytes = len(body)
	if len(body) > maxBody {
		event.Kind = "body_too_large"
		return nil, errors.New("diagnostic response exceeds size limit")
	}
	sum := sha256.Sum256(body)
	event.SHA256 = hex.EncodeToString(sum[:])
	event.Kind = classify(body, response.StatusCode)
	// The observer must preserve the exact bytes for the real adapter's decoder.
	response.Body = io.NopCloser(bytes.NewReader(body))
	return response, nil
}

func classify(body []byte, status int) string {
	if status == 405 {
		return "method_not_allowed"
	}
	if status == 429 {
		return "rate_limited"
	}
	if json.Valid(body) {
		var m map[string]json.RawMessage
		if json.Unmarshal(body, &m) == nil {
			for _, k := range []string{"error", "errors"} {
				v := strings.TrimSpace(string(m[k]))
				if v != "" && v != "null" && v != "false" && v != "0" && v != `""` && v != "[]" {
					return "json_api_error"
				}
			}
		}
		if status >= 400 {
			return "json_http_error"
		}
		return "json"
	}
	lower := bytes.ToLower(body)
	if bytes.Contains(lower, []byte("<h1>site unavailable</h1>")) {
		return "site_unavailable_html"
	}
	if bytes.Contains(lower, []byte("<html")) || bytes.Contains(lower, []byte("<!doctype html")) {
		return "html"
	}
	if bytes.HasPrefix(bytes.TrimSpace(body), []byte("<?xml")) {
		return "xml"
	}
	if status >= 400 {
		return "http_error"
	}
	return "unrecognized_body"
}

func errorKind(err error) string {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "deadline_or_cancelled"
	}
	var timeout interface{ Timeout() bool }
	if errors.As(err, &timeout) && timeout.Timeout() {
		return "timeout"
	}
	return "network_error"
}

func safeHeader(value string) string {
	if len(value) > 160 {
		value = value[:160]
	}
	return strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, value)
}

func safeURL(u *url.URL) string {
	result := url.URL{Scheme: u.Scheme, Host: u.Host, Path: u.Path}
	allowed := map[string]bool{"__call": true, "q": true, "query": true, "fmt": true, "limit": true, "n": true, "p": true, "pids": true, "ctx": true, "api_version": true, "_format": true, "_marker": true, "cc": true, "includeMetaTags": true, "type": true, "method": true}
	values := url.Values{}
	// This token is a public catalog link identifier, not a session token.
	if u.Hostname() == "www.jiosaavn.com" && u.Query().Get("__call") == "webapi.get" && u.Query().Get("type") == "song" {
		allowed["token"] = true
	}
	for key, list := range u.Query() {
		if allowed[key] {
			for _, value := range list {
				values.Add(key, value)
			}
		}
	}
	result.RawQuery = values.Encode()
	return result.String()
}

type Candidate struct {
	Title   string             `json:"title"`
	Artists []canonical.Artist `json:"artists"`
	IDs     map[string]string  `json:"ids"`
}

type Report struct {
	Provider         string          `json:"provider"`
	Version          string          `json:"version"`
	Query            *adapters.Query `json:"query,omitempty"`
	AdapterStatus    string          `json:"adapter_status"`
	AdapterErrorKind string          `json:"adapter_error_kind,omitempty"`
	Operation        string          `json:"operation"`
	Candidates       []Candidate     `json:"candidates"`
	Requests         []Event         `json:"requests"`
	LookupID         string          `json:"lookup_id,omitempty"`
	LookupIDType     string          `json:"lookup_id_type,omitempty"`
	Observation      string          `json:"observation"`
}

type Options struct {
	Routes     bool
	IDType, ID string
}

// Run calls the public adapter itself, independent of bootstrap, credentials,
// resolver matching and caches. Transport must also be wired into the adapter.
func Run(ctx context.Context, adapter adapters.ProviderAdapter, transport *Transport, q adapters.Query, options Options) Report {
	result := Report{Provider: adapter.Name(), Version: adapter.Version(), Query: &q, Candidates: []Candidate{}}
	result.Operation = "search"
	if options.ID != "" {
		result.Operation = "lookup"
		result.Query = nil
		result.LookupID, result.LookupIDType = options.ID, options.IDType
		if result.LookupIDType == "" {
			result.LookupIDType = adapter.Name()
		}
	}
	child, cancel := context.WithTimeout(phase(ctx, "adapter."+result.Operation), 20*time.Second)
	var candidates []canonical.TrackCandidate
	var err error
	if options.ID == "" {
		candidates, err = adapter.Search(child, q)
	} else {
		kind := options.IDType
		if kind == "" {
			kind = adapter.Name()
		}
		var candidate *canonical.TrackCandidate
		candidate, err = adapter.FetchByID(child, kind, options.ID)
		if err == nil && candidate != nil {
			candidates = append(candidates, *candidate)
		}
	}
	cancel()
	result.AdapterStatus = "empty"
	if err != nil {
		result.AdapterStatus = "error"
		switch {
		case errors.Is(err, webdata.ErrUnavailable):
			result.AdapterErrorKind = "unavailable_catalog_page"
		case errors.Is(err, webdata.ErrNoData):
			result.AdapterErrorKind = "unrecognized_catalog_page"
		default:
			var syntax *json.SyntaxError
			var valueType *json.UnmarshalTypeError
			if errors.As(err, &syntax) || errors.As(err, &valueType) {
				result.AdapterErrorKind = "response_decode_error"
			} else {
				result.AdapterErrorKind = "request_or_catalog_error"
			}
		}
	}
	for _, candidate := range candidates {
		if candidate.Track.Title == "" || len(candidate.Track.Artists) == 0 {
			continue
		}
		result.Candidates = append(result.Candidates, Candidate{Title: candidate.Track.Title, Artists: candidate.Track.Artists, IDs: candidate.Track.IDs})
	}
	if len(result.Candidates) > 0 {
		result.AdapterStatus = "metadata_available"
	}
	if options.Routes {
		for _, route := range Routes(adapter.Name(), q) {
			if ctx.Err() != nil {
				break
			}
			child, cancel := context.WithTimeout(phase(ctx, route.Name), 10*time.Second)
			req, _ := http.NewRequestWithContext(child, http.MethodGet, route.URL, nil)
			req.Header.Set("User-Agent", UserAgent)
			req.Header.Set("Accept", route.Accept)
			response, err := (&http.Client{Transport: transport, CheckRedirect: func(next *http.Request, via []*http.Request) error {
				if len(via) >= 3 || next.URL.Scheme != "https" || next.URL.Hostname() != req.URL.Hostname() {
					return errors.New("diagnostic redirect refused")
				}
				return nil
			}}).Do(req)
			if err == nil {
				_ = response.Body.Close()
			}
			cancel()
		}
	}
	transport.mu.Lock()
	result.Requests = append([]Event{}, transport.events...)
	transport.mu.Unlock()
	result.Observation = "insufficient_evidence"
	if result.AdapterStatus == "metadata_available" {
		result.Observation = "adapter_decoded_catalog_identity_not_verified"
		return result
	}
	homepageUnavailable, apiUnavailable := false, false
	for _, event := range result.Requests {
		if event.Kind == "site_unavailable_html" {
			if event.Phase == "route.home" {
				homepageUnavailable = true
			} else {
				apiUnavailable = true
			}
		}
	}
	if homepageUnavailable && apiUnavailable {
		result.Observation = "homepage_and_api_unavailable_origin_unconfirmed"
	}
	return result
}

type Route struct{ Name, URL, Accept string }

func Routes(provider string, q adapters.Query) []Route {
	text := url.QueryEscape(strings.TrimSpace(q.Artist + " " + q.Title))
	switch provider {
	case "jiosaavn":
		return []Route{
			{"route.home", "https://www.jiosaavn.com/", "text/html"},
			{"route.share_lookup", "https://www.jiosaavn.com/api.php?__call=webapi.get&_format=json&_marker=0&ctx=web6dot0&type=song&token=OQsEfQFVUXk", "application/json"},
			{"route.autocomplete", "https://www.jiosaavn.com/api.php?__call=autocomplete.get&_format=json&_marker=0&cc=in&includeMetaTags=1&query=" + text, "application/json"},
			{"route.legacy_search", "https://www.jiosaavn.com/api.php?__call=search.getResults&_format=json&_marker=0&ctx=web6dot0&q=" + text, "application/json"},
		}
	case "deezer":
		return []Route{
			{"route.home", "https://www.deezer.com/", "text/html"},
			{"route.plain_search", "https://api.deezer.com/search?limit=5&q=" + text, "application/json"},
		}
	case "musicbrainz":
		return []Route{
			{"route.home", "https://musicbrainz.org/", "text/html"},
			{"route.plain_search_json", "https://musicbrainz.org/ws/2/recording?query=" + text + "&limit=5&fmt=json", "application/json"},
			{"route.plain_search_xml", "https://musicbrainz.org/ws/2/recording?query=" + text + "&limit=5&fmt=xml", "application/xml"},
		}
	}
	return nil
}
