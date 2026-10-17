package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ommr/ommr/internal/providerhealth"
)

func TestQueryGatewayUsesConfiguredOperationAndSources(t *testing.T) {
	h, calls := responseFixture(t)
	h.cfg.API.DefaultSources = []string{"deezer"}
	h.cfg.API.DefaultOperation = "resolve"
	router := NewRouter(h, slog.New(slog.NewTextHandler(io.Discard, nil)), 100)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/query", strings.NewReader(`{"artist":"Singer","title":"Song"}`)))
	var result map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &result)
	if rec.Code != 200 || result["track"] == nil || result["provider_status"] != nil || calls.Load() != 1 {
		t.Fatalf("configured JSON query failed: %d %s", rec.Code, rec.Body)
	}
}
func TestEndpointAndDebugPolicyCannotBeBypassedByGateway(t *testing.T) {
	h, _ := responseFixture(t)
	disabled := false
	h.cfg.API.AllowDebug = &disabled
	h.cfg.API.Endpoints = map[string]bool{"search": false}
	router := NewRouter(h, slog.New(slog.NewTextHandler(io.Discard, nil)), 100)
	for _, target := range []struct {
		method, url, body string
		status            int
	}{
		{"GET", "/v1/search?q=Song", "", 404}, {"POST", "/v1/query", `{"operation":"search","q":"Song"}`, 404},
		{"POST", "/v1/query", `{"artist":"Singer","title":"Song","debug":true}`, 200},
	} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(target.method, target.url, strings.NewReader(target.body)))
		if rec.Code != target.status {
			t.Fatalf("wrong policy result: %d %s", rec.Code, rec.Body)
		}
		if strings.Contains(rec.Body.String(), `"provider_status"`) {
			t.Fatal("debug block bypassed")
		}
	}
}
func TestConfiguredDefaultsCanBeLockedAgainstRequestOverrides(t *testing.T) {
	h, _ := responseFixture(t)
	disabled := false
	h.cfg.API.AllowOverrides = &disabled
	h.cfg.API.DefaultOperation = "resolve"
	h.cfg.API.DefaultSources = []string{"deezer"}
	router := NewRouter(h, slog.New(slog.NewTextHandler(io.Discard, nil)), 100)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/query", strings.NewReader(`{"operation":"search","artist":"Singer","title":"Song","sources":["unknown"],"debug":true}`)))
	if rec.Code != 200 || strings.Contains(rec.Body.String(), `"provider_status"`) {
		t.Fatalf("request override bypassed configuration: %s", rec.Body)
	}
}
func TestProviderHealthFiltersSuspendedAdapters(t *testing.T) {
	h, calls := responseFixture(t)
	health := providerhealth.New(1, time.Minute)
	health.Set("deezer", "active", "public", "absent", "")
	lease, _ := health.Begin("deezer")
	health.Finish(context.Background(), "deezer", lease, io.ErrUnexpectedEOF)
	h.registry.SetAvailability(health.Available)
	h.SetProviderHealth(health)
	if active, err := h.registry.ListActive(nil); err != nil || len(active) != 0 {
		t.Fatal("suspended provider still selected")
	}
	rec := httptest.NewRecorder()
	h.HandleProviders(rec, httptest.NewRequest("GET", "/v1/providers", nil))
	if !strings.Contains(rec.Body.String(), `"state":"suspended"`) || calls.Load() != 0 {
		t.Fatal("health state missing or provider contacted")
	}
}
