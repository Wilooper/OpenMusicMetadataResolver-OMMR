package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ommr/ommr/internal/artists"
	"github.com/ommr/ommr/internal/cache/disk"
)

type artistTransport func(*http.Request) (*http.Response, error)

func (f artistTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestArtistsGETPOSTAndEndpointPolicy(t *testing.T) {
	h, _ := responseFixture(t)
	backend, err := disk.NewDiskCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	service := artists.New(nil, backend)
	var calls atomic.Int32
	service.HTTP.Transport = artistTransport(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		body := `{"id":123,"name":"Artist","nb_fan":0}`
		if strings.HasSuffix(r.URL.Path, "/top") {
			body = `{"data":[{"id":12,"title":"Song","artist":{"name":"Artist"}}]}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	h.SetArtists(service)
	h.cfg.API.DefaultSources = []string{"spotify"}
	router := NewRouter(h, slog.New(slog.NewTextHandler(io.Discard, nil)), 100)
	get := httptest.NewRecorder()
	router.ServeHTTP(get, httptest.NewRequest("GET", "/v1/artists?provider=deezer&id=123", nil))
	if get.Code != 200 {
		t.Fatal(get.Body.String())
	}
	var production map[string]any
	if json.Unmarshal(get.Body.Bytes(), &production) != nil {
		t.Fatal("invalid response")
	}
	if production["sources"] != nil {
		t.Fatal("production diagnostics leaked")
	}
	post := httptest.NewRecorder()
	router.ServeHTTP(post, httptest.NewRequest("POST", "/v1/query", strings.NewReader(`{"operation":"artists","provider":"deezer","id":"123"}`)))
	var gateway map[string]any
	_ = json.Unmarshal(post.Body.Bytes(), &gateway)
	if post.Code != 200 || !reflect.DeepEqual(production, gateway) || calls.Load() != 2 {
		t.Fatalf("GET/POST parity or cache failed: %s %s calls=%d", get.Body.String(), post.Body.String(), calls.Load())
	}
	debug := httptest.NewRecorder()
	router.ServeHTTP(debug, httptest.NewRequest("GET", "/v1/artists?provider=deezer&id=123&debug=true", nil))
	var detailed map[string]any
	_ = json.Unmarshal(debug.Body.Bytes(), &detailed)
	if detailed["sources"] == nil || calls.Load() != 2 {
		t.Fatal("debug/cache failed")
	}
	for _, target := range []string{"/v1/artists?provider=deezer&id=123&id=456", "/v1/artists?artist=Artist&sources=spotify", "/v1/artists?provider=deezer&id=https://evil.test", "/v1/artists?provider=deezer&id=123&url=https://evil.test", "/v1/artists?artist=Artist&limit=51"} {
		r := httptest.NewRecorder()
		router.ServeHTTP(r, httptest.NewRequest("GET", target, nil))
		if r.Code != 400 {
			t.Fatalf("accepted invalid target: %s %d", target, r.Code)
		}
	}
	if calls.Load() != 2 {
		t.Fatal("invalid request made source calls")
	}
	h.cfg.API.Endpoints = map[string]bool{"artists": false}
	for _, method := range []string{"GET", "POST"} {
		target, body := "/v1/artists?provider=deezer&id=123", ""
		if method == "POST" {
			target = "/v1/query"
			body = `{"operation":"artists","provider":"deezer","id":"123"}`
		}
		r := httptest.NewRecorder()
		router.ServeHTTP(r, httptest.NewRequest(method, target, strings.NewReader(body)))
		if r.Code != 404 {
			t.Fatalf("disabled endpoint bypassed: %s %d", method, r.Code)
		}
	}
}
