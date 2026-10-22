package bootstrap

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ommr/ommr/internal/config"
)

func TestRequiredCatalogBootProbeRejectsHTTP200Errors(t *testing.T) {
	apiError := false
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Query().Get("app_id") != "test-app-id" {
			t.Error("configured key not used")
		}
		if apiError {
			io.WriteString(w, `{"status":"error","code":401}`)
		} else {
			io.WriteString(w, `{"tracks":{"items":[{"id":123,"title":"Probe Song","performer":{"id":1,"name":"Probe Artist"}}]}}`)
		}
	}))
	defer server.Close()
	enabled := map[string]bool{}
	for _, name := range []string{"spotify", "applemusic", "ytmusic", "musicbrainz", "deezer", "soundcloud", "jiosaavn", "tidal", "amazonmusic", "pandora"} {
		enabled[name] = false
	}
	cfg := &config.Config{QobuzAppID: "test-app-id", QobuzBaseURL: server.URL, CacheTTL: time.Hour, Lifecycle: config.LifecycleConfig{FailureThreshold: 2, Cooldown: time.Minute, ProbeTimeout: time.Second, StartupTimeout: 2 * time.Second, ProbeWorkers: 2, Enabled: enabled}}
	runtime, err := Build(context.Background(), cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := runtime.Registry.Get("qobuz"); !ok {
		t.Fatal("working required source did not load")
	}
	apiError = true
	runtime, err = Build(context.Background(), cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := runtime.Registry.Get("qobuz"); ok {
		t.Fatal("HTTP 200 API error counted as validated access")
	}
	if requests != 2 {
		t.Fatalf("expected two boot probes, got %d", requests)
	}
}

func TestSponsorBlockLifecycleRegistrationAndFlags(t *testing.T) {
	for _, mode := range []string{"enabled", "source_disabled", "enrichment_disabled"} {
		t.Run(mode, func(t *testing.T) {
			enabled := map[string]bool{}
			for _, name := range []string{"spotify", "applemusic", "ytmusic", "musicbrainz", "deezer", "soundcloud", "jiosaavn", "qobuz", "tidal", "amazonmusic", "pandora", "bugs", "genie", "melon", "coverartarchive", "musicbrainz_release"} {
				enabled[name] = false
			}
			enabled["sponsorblock"] = mode != "source_disabled"
			cfg := &config.Config{EnableEnrichment: mode != "enrichment_disabled", Lifecycle: config.LifecycleConfig{FailureThreshold: 2, Cooldown: time.Minute, ProbeTimeout: time.Second, StartupTimeout: time.Second, ProbeWorkers: 2, Enabled: enabled}}
			runtime, err := Build(context.Background(), cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, s := range runtime.Health.Snapshot() {
				if s.Name == "sponsorblock" {
					found = true
					if (s.State == "active") != (mode == "enabled") {
						t.Fatalf("wrong source state: %+v", s)
					}
				}
			}
			if !found {
				t.Fatal("missing source lifecycle")
			}
			if _, ok := runtime.Registry.Get("sponsorblock"); ok {
				t.Fatal("annotations became recording identity adapter")
			}
		})
	}
}
