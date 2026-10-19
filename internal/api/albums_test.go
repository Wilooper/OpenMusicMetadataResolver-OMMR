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

	"github.com/ommr/ommr/internal/albums"
	"github.com/ommr/ommr/internal/cache/disk"
)

func TestAlbumsGETGatewayLimitsAndPolicy(t *testing.T) {
	h, _ := responseFixture(t)
	backend, e := disk.NewDiskCache(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer backend.Close()
	s := albums.New(nil, backend)
	var calls atomic.Int32
	s.HTTP.Transport = artistTransport(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		body := `{"results":[{"wrapperType":"collection","collectionId":123,"collectionName":"Album","trackCount":2},{"wrapperType":"track","trackId":11,"collectionId":123,"trackName":"First","discNumber":1,"trackNumber":1},{"wrapperType":"track","trackId":12,"collectionId":123,"trackName":"Second","discNumber":1,"trackNumber":2}]}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	h.SetAlbums(s)
	h.cfg.API.DefaultSources = []string{"spotify"}
	router := NewRouter(h, slog.New(slog.NewTextHandler(io.Discard, nil)), 1000)
	call := func(method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		return w
	}
	get := call("GET", "/v1/albums?provider=applemusic&id=123", "")
	post := call("POST", "/v1/query", `{"operation":"albums","provider":"applemusic","id":"123"}`)
	var a, b map[string]any
	_ = json.Unmarshal(get.Body.Bytes(), &a)
	_ = json.Unmarshal(post.Body.Bytes(), &b)
	if get.Code != 200 || post.Code != 200 || !reflect.DeepEqual(a, b) || a["source"] != nil || calls.Load() != 1 {
		t.Fatalf("GET/POST/cache: %s %s %d", get.Body.String(), post.Body.String(), calls.Load())
	}
	debug := call("GET", "/v1/albums?provider=applemusic&id=123&debug=true", "")
	if debug.Code != 200 || !strings.Contains(debug.Body.String(), `"cached":true`) {
		t.Fatal(debug.Body.String())
	}
	limit := call("POST", "/v1/query", `{"operation":"albums","query":{"provider":"applemusic","id":"123","limit":1}}`)
	if limit.Code != 200 || !strings.Contains(limit.Body.String(), `"complete":false,"reason":"limit"`) {
		t.Fatal(limit.Body.String())
	}
	for _, path := range []string{"/v1/albums?provider=applemusic&id=123&id=456", "/v1/albums?provider=applemusic&id=123&limit=0", "/v1/albums?provider=applemusic&id=123&url=https://foreign.invalid", "/v1/albums?provider=musicbrainz&id=123"} {
		w := call("GET", path, "")
		if w.Code != 400 {
			t.Fatalf("invalid input: %s %d", path, w.Code)
		}
	}
	disabled := false
	h.cfg.API.AllowDebug = &disabled
	w := call("GET", "/v1/albums?provider=applemusic&id=123&debug=true", "")
	var public map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &public)
	if w.Code != 200 || public["source"] != nil {
		t.Fatal("disabled debug ignored")
	}
	h.cfg.API.Endpoints = map[string]bool{"albums": false}
	if call("GET", "/v1/albums?provider=applemusic&id=123", "").Code != 404 || call("POST", "/v1/query", `{"operation":"albums","provider":"applemusic","id":"123"}`).Code != 404 {
		t.Fatal("endpoint policy bypassed")
	}
	h.cfg.API.Endpoints = nil
	h.cfg.API.DefaultOperation = "albums"
	if call("POST", "/v1/query", `{"provider":"applemusic","id":"123"}`).Code != 200 {
		t.Fatal("default operation unsupported")
	}
	h.cfg.Lifecycle.Enabled = map[string]bool{"applemusic": false}
	if call("GET", "/v1/albums?provider=applemusic&id=123", "").Code != 503 {
		t.Fatal("provider policy bypassed")
	}
}
func TestAlbumCapabilitiesAndUnavailable(t *testing.T) {
	h, _ := responseFixture(t)
	h.SetAlbums(albums.New(nil, nil))
	h.albums.Available = func(p string) bool { return p == "deezer" }
	w := httptest.NewRecorder()
	h.HandleCapabilities(w, httptest.NewRequest("GET", "/v1/capabilities", nil))
	var response struct {
		API struct {
			Endpoints map[string]endpointCapability `json:"endpoints"`
		} `json:"api"`
		Providers []providerCapability `json:"providers"`
	}
	if json.Unmarshal(w.Body.Bytes(), &response) != nil || !response.API.Endpoints["albums"].RuntimeAvailable {
		t.Fatal(w.Body.String())
	}
	for _, p := range response.Providers {
		if strings.Contains(strings.Join(p.Operations, ","), "albums") != (p.Name == "deezer") {
			t.Fatalf("incorrect album readiness: %+v", p)
		}
	}
	h.SetAlbums(nil)
	w = httptest.NewRecorder()
	h.HandleAlbums(w, httptest.NewRequest("GET", "/v1/albums?provider=deezer&id=123", nil))
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
}
