package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/cache/disk"
	"github.com/ommr/ommr/internal/config"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/ratelimit"
	"github.com/ommr/ommr/internal/resolver"
)

type unavailableLinkAdapter struct{ extractionFixtureAdapter }

func (a unavailableLinkAdapter) Search(_ context.Context, q adapters.Query) ([]canonical.TrackCandidate, error) {
	if q.Title == "" {
		return nil, nil
	}
	return nil, fmt.Errorf("secondary catalog unavailable")
}

func TestSecondaryLinkSearchFailureIsReported(t *testing.T) {
	registry := adapters.NewRegistry()
	registry.Register(extractionFixtureAdapter{name: "ytmusic", track: canonical.Track{Title: "Song", Artists: []canonical.Artist{{Name: "Singer"}}, IDs: map[string]string{"ytmusic": "x-KOXck57lc"}}})
	registry.Register(unavailableLinkAdapter{extractionFixtureAdapter{name: "applemusic"}})
	handler := NewHandler(resolver.New(registry, nil, ratelimit.NewProviderLimiter(), time.Hour), registry, &config.Config{ProviderTimeout: time.Second})
	recorder := httptest.NewRecorder()
	handler.HandleLinks(recorder, httptest.NewRequest("GET", "/v1/links?youtube_id=x-KOXck57lc", nil))
	var got linksResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	for _, status := range got.ProviderStatus {
		if status.Name == "applemusic" {
			if status.Success || status.Error != "secondary catalog unavailable" || status.RejectionReason != "provider_error" {
				t.Fatalf("cross-search failure hidden: %+v", status)
			}
			return
		}
	}
	t.Fatal("missing provider status")
}

func TestLinksEndpointReturnsOnlyAcceptedOwnProviderIdentities(t *testing.T) {
	registry := adapters.NewRegistry()
	registry.Register(extractionFixtureAdapter{name: "ytmusic", track: canonical.Track{Title: "Song", Artists: []canonical.Artist{{Name: "Singer"}}, ISRC: "USABC2300001", IDs: map[string]string{"ytmusic": "x-KOXck57lc", "tidal": "987", "tidal_url": "https://tidal.com/track/987"}}})
	registry.Register(extractionFixtureAdapter{name: "spotify", track: canonical.Track{Title: "Song", Artists: []canonical.Artist{{Name: "Singer"}}, ISRC: "USABC2300001", IDs: map[string]string{"spotify": "4cOdK2wGLETKBW3PvgPWqT"}}})
	registry.Register(extractionFixtureAdapter{name: "applemusic", track: canonical.Track{Title: "Unrelated", Artists: []canonical.Artist{{Name: "Other"}}, ISRC: "CONFLICT", IDs: map[string]string{"applemusic": "999"}}})
	cache, err := disk.NewDiskCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer cache.Close()
	res := resolver.New(registry, cache, ratelimit.NewProviderLimiter(), time.Hour)
	handler := NewHandler(res, registry, &config.Config{ProviderTimeout: time.Second})
	router := NewRouter(handler, slog.Default(), 50)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest("GET", "/v1/links?youtube_id=x-KOXck57lc", nil))
	if recorder.Code != 200 {
		t.Fatalf("links failed: %s", recorder.Body)
	}
	var got linksResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Links.OMMRID == "" || got.Links.ISRC != "USABC2300001" || got.Links.Platforms["ytmusic"].URL != "https://music.youtube.com/watch?v=x-KOXck57lc" || got.Links.Platforms["spotify"].ID == "" {
		t.Fatalf("missing accepted links: %+v", got)
	}
	for _, rejected := range []string{"applemusic", "tidal"} {
		if _, ok := got.Links.Platforms[rejected]; ok {
			t.Fatalf("unaccepted %s link leaked", rejected)
		}
	}
	if len(got.ProviderStatus) != 3 {
		t.Fatal("missing provider diagnostics")
	}
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest("GET", "/v1/resolve?youtube_id=x-KOXck57lc", nil))
	var resolved resolver.ResolveResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resolved); err != nil {
		t.Fatal(err)
	}
	if resolved.Track == nil || len(resolved.Track.Extensions["platform_links"]) == 0 || !resolved.ProviderStatus[0].Cached {
		t.Fatal("link extension missing from cached resolution")
	}
	if len(resolved.Track.IdentityMatches) != 2 {
		t.Fatalf("foreign IDs or helpers treated as identity evidence: %+v", resolved.Track.IdentityMatches)
	}
}

func TestLinksEndpointValidationAndEmptyResults(t *testing.T) {
	registry := adapters.NewRegistry()
	registry.Register(extractionFixtureAdapter{name: "spotify", track: canonical.Track{Title: "Other", Artists: []canonical.Artist{{Name: "Other"}}, ISRC: "CONFLICT", IDs: map[string]string{"spotify": "4cOdK2wGLETKBW3PvgPWqT"}}})
	handler := NewHandler(resolver.New(registry, nil, ratelimit.NewProviderLimiter(), time.Hour), registry, &config.Config{ProviderTimeout: time.Second})
	for _, tt := range []struct {
		method, target string
		code           int
	}{{"POST", "/v1/links?title=Song&artist=Singer", 405}, {"GET", "/v1/links", 400}, {"GET", "/v1/links?isrc=USABC2300001", 200}} {
		recorder := httptest.NewRecorder()
		handler.HandleLinks(recorder, httptest.NewRequest(tt.method, tt.target, nil))
		if recorder.Code != tt.code {
			t.Fatalf("%s %s = %d", tt.method, tt.target, recorder.Code)
		}
		if tt.code == 200 {
			var got linksResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if len(got.Links.Platforms) != 0 || got.Links.OMMRID != "" || got.Links.ISRC != "" {
				t.Fatal("no-match response invented identity")
			}
		}
	}
}
