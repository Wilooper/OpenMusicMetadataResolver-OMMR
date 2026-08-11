package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/adapters/applemusic"
	"github.com/ommr/ommr/internal/adapters/deezer"
	"github.com/ommr/ommr/internal/adapters/musicbrainz"
	"github.com/ommr/ommr/internal/adapters/spotify"
	"github.com/ommr/ommr/internal/adapters/ytmusic"
	"github.com/ommr/ommr/internal/api"
	"github.com/ommr/ommr/internal/cache/disk"
	"github.com/ommr/ommr/internal/config"
	"github.com/ommr/ommr/internal/ratelimit"
	"github.com/ommr/ommr/internal/resolver"
)

func setupTestServer(t *testing.T) *httptest.Server {
	dir, err := os.MkdirTemp("", "ommr-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	diskCache, err := disk.NewDiskCache(dir)
	if err != nil {
		t.Fatalf("failed to create disk cache: %v", err)
	}

	reg := adapters.NewRegistry()
	reg.Register(musicbrainz.New())
	reg.Register(deezer.New())
	reg.Register(applemusic.New())
	reg.Register(ytmusic.New())
	reg.Register(spotify.New())

	limiter := ratelimit.NewProviderLimiter()
	res := resolver.New(reg, diskCache, limiter, 1*time.Hour)

	cfg := &config.Config{
		Port:            8080,
		CacheProvider:   "disk",
		CacheDir:        dir,
		CacheTTL:        1 * time.Hour,
		RateLimitRPS:    100,
		ProviderTimeout: 5 * time.Second,
		BulkMaxItems:    100,
		BulkWorkers:     5,
	}

	h := api.NewHandler(res, reg, cfg)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	router := api.NewRouter(h, logger, 100)

	return httptest.NewServer(router)
}

func TestHealthEndpoint(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/health")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var hResp api.HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&hResp); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}

	if hResp.Status != "healthy" {
		t.Errorf("expected status 'healthy', got %q", hResp.Status)
	}
}

func TestResolveEndpointValidation(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/resolve")
	if err != nil {
		t.Fatalf("resolve request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400 Bad Request for empty query, got %d", resp.StatusCode)
	}
}

func TestBulkEndpointValidation(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	emptyPayload := []byte(`{"queries":[]}`)
	resp, err := http.Post(ts.URL+"/v1/bulk", "application/json", bytes.NewBuffer(emptyPayload))
	if err != nil {
		t.Fatalf("bulk request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400 for empty queries array, got %d", resp.StatusCode)
	}
}

func TestSearchEndpointPagination(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/search?q=believer&page=1&limit=2")
	if err != nil {
		t.Fatalf("search request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 OK for search, got %d", resp.StatusCode)
	}

	var sResp api.SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sResp); err != nil {
		t.Fatalf("failed to decode search response: %v", err)
	}

	if sResp.Page != 1 || sResp.Limit != 2 {
		t.Errorf("expected page 1, limit 2; got page %d, limit %d", sResp.Page, sResp.Limit)
	}
}
