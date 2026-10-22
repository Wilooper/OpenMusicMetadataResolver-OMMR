package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ommr/ommr/internal/artists"
	"github.com/ommr/ommr/internal/providerhealth"
)

func TestCapabilitiesReflectConfigurationAndRuntime(t *testing.T) {
	h, _ := linkHandler(t, "deezer", false)
	disabled := false
	h.cfg.API.AllowDebug = &disabled
	h.cfg.API.Endpoints = map[string]bool{"search": false}
	h.cfg.SpotifyCookie = "PRIVATE_COOKIE"
	h.cfg.SpotifyClientSecret = "PRIVATE_SECRET"
	h.cfg.Lifecycle.Enabled = map[string]bool{"ytmusic": false}
	h.artists = artists.New(nil, nil)
	health := providerhealth.New(1, time.Hour)
	health.Set("deezer", "active", "public", "absent", "")
	lease, _ := health.Begin("deezer")
	health.Finish(context.Background(), "deezer", lease, errors.New("unavailable"))
	h.health = health
	h.artists.Health = health
	w := httptest.NewRecorder()
	h.HandleCapabilities(w, httptest.NewRequest("GET", "/v1/capabilities", nil))
	if w.Code != 200 || strings.Contains(w.Body.String(), "PRIVATE_") || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("unsafe capabilities: %d %s", w.Code, w.Body.String())
	}
	var result struct {
		API struct {
			AllowDebug bool                          `json:"allow_debug"`
			Endpoints  map[string]endpointCapability `json:"endpoints"`
		} `json:"api"`
		Providers []providerCapability `json:"providers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.API.AllowDebug || result.API.Endpoints["search"].Enabled || result.API.Endpoints["catalog"].RuntimeAvailable {
		t.Fatal("capabilities ignored endpoint/config/worker state")
	}
	for _, source := range result.Providers {
		if source.Name == "deezer" && (source.TrackRuntimeAvailable || len(source.Operations) > 0 || !source.TrackAdapterRegistered) {
			t.Fatalf("suspended source advertised: %+v", source)
		}
		if source.Name == "ytmusic" && (source.Enabled || len(source.Operations) > 0) {
			t.Fatalf("disabled source advertised: %+v", source)
		}
		if source.Name == "jiosaavn" && (source.TrackAdapterRegistered || strings.Join(source.Operations, ",") != "artists") {
			t.Fatalf("artist service availability conflated with track adapter: %+v", source)
		}
	}
}

func TestCapabilitiesRouteGatewayAndPolicy(t *testing.T) {
	h, _ := linkHandler(t, "deezer", false)
	router := NewRouter(h, slog.New(slog.NewTextHandler(io.Discard, nil)), 100)
	for _, method := range []string{"GET", "POST"} {
		path, body := "/v1/capabilities", ""
		if method == "POST" {
			path, body = "/v1/query", `{"operation":"capabilities"}`
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"capabilities_version":1`) {
			t.Fatalf("%s: %d %s", method, w.Code, w.Body.String())
		}
	}
	h.cfg.API.Endpoints = map[string]bool{"capabilities": false}
	for _, tc := range []struct{ method, path, body string }{{"GET", "/v1/capabilities", ""}, {"POST", "/v1/query", `{"operation":"capabilities"}`}} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
		if w.Code != 404 {
			t.Fatal("disabled capability endpoint bypassed")
		}
	}
	h.cfg.API.Endpoints = nil
	h.cfg.API.DefaultOperation = "capabilities"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("POST", "/v1/query", strings.NewReader(`{}`)))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"capabilities_version":1`) {
		t.Fatal("default capability operation unavailable")
	}
}
