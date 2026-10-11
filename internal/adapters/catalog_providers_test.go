package adapters_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/adapters/amazonmusic"
	"github.com/ommr/ommr/internal/adapters/pandora"
	"github.com/ommr/ommr/internal/adapters/qobuz"
	"github.com/ommr/ommr/internal/adapters/tidal"
)

func TestNewCatalogProvidersNormalizeSameFixtureMetadata(t *testing.T) {
	const payload = `{"data":{"id":"fixture-id","title":"Example track","duration":212,"isrc":"USAAA2400001","releaseDate":"2024-02-03","artists":[{"id":"artist-1","name":"Example artist"}],"album":{"title":"Example album","releaseDate":"2024-02-03","images":[{"url":"https://img.example/cover.jpg","width":600,"height":600}]}}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
		}
		if strings.HasPrefix(r.URL.Path, "/api.json/") && r.URL.Query().Get("app_id") != "issued-app" {
			t.Errorf("Qobuz app ID missing: %s", r.URL)
		}
		if strings.Contains(r.URL.Path, "/v2/") && r.Header.Get("Authorization") != "Bearer approved-token" {
			t.Errorf("OAuth token missing for %s", r.URL.Path)
		}
		if strings.HasPrefix(r.URL.Path, "/v2/") && r.URL.Path != "/v2/tracks/tidal-id" && r.Header.Get("x-api-key") != "security-profile" {
			t.Errorf("Amazon security profile missing")
		}
		if r.URL.Path == "/graphql" && r.Method != http.MethodPost {
			t.Errorf("Pandora GraphQL method = %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		fixture := payload
		fixtureID := ""
		switch {
		case r.URL.Path == "/api.json/0.2/track/get":
			fixtureID = r.URL.Query().Get("track_id")
		case strings.HasPrefix(r.URL.Path, "/v2/tracks/"):
			fixtureID = strings.TrimPrefix(r.URL.Path, "/v2/tracks/")
		case r.URL.Path == "/graphql":
			fixture = strings.Replace(fixture, `"id":"fixture-id",`, "", 1)
		}
		if fixtureID != "" {
			fixture = strings.Replace(fixture, "fixture-id", fixtureID, 1)
		}
		fmt.Fprint(w, fixture)
	}))
	defer server.Close()

	providers := []struct {
		name, id string
		adapter  adapters.ProviderAdapter
	}{
		{"qobuz", "qobuz-id", qobuz.NewWithConfig(server.URL, "issued-app", "")},
		{"tidal", "tidal-id", tidal.NewWithConfig(server.URL, "approved-token")},
		{"amazonmusic", "amazon-id", amazonmusic.NewWithConfig(server.URL, "approved-token", "security-profile")},
		{"pandora", "pandora-id", pandora.NewWithConfig(server.URL, "approved-token")},
	}
	for _, tc := range providers {
		t.Run(tc.name, func(t *testing.T) {
			candidate, err := tc.adapter.FetchByID(context.Background(), tc.name, tc.id)
			if err != nil {
				t.Fatal(err)
			}
			track := candidate.Track
			if track.Title != "Example track" || len(track.Artists) != 1 || track.Artists[0].Name != "Example artist" || track.Album.Title != "Example album" || track.ISRC != "USAAA2400001" || track.ReleaseDate != "2024-02-03" || len(track.Images) == 0 {
				t.Fatalf("incomplete normalized metadata: %+v", track)
			}
			if len(track.Credits) != 0 {
				t.Fatalf("fixture has no credits; adapter invented credits: %+v", track.Credits)
			}
		})
	}
}

func TestCatalogProvidersFailClosedWithoutCredentials(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++ }))
	defer server.Close()
	providers := []struct {
		name    string
		adapter adapters.ProviderAdapter
	}{
		{"qobuz", qobuz.NewWithConfig(server.URL, "", "")},
		{"tidal", tidal.NewWithConfig(server.URL, "")},
		{"amazonmusic", amazonmusic.NewWithConfig(server.URL, "", "")},
		{"pandora", pandora.NewWithConfig(server.URL, "")},
	}
	for _, tc := range providers {
		if _, err := tc.adapter.FetchByID(context.Background(), tc.name, "id"); err == nil {
			t.Errorf("%s should reject requests without required credentials", tc.name)
		}
	}
	if requests != 0 {
		t.Fatalf("credential-free adapters made %d network requests", requests)
	}
}

func TestCatalogProviderRejectsResponseForAnotherID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":"different-track","title":"Wrong song","artist":{"name":"Wrong artist"}}`)
	}))
	defer server.Close()
	provider := qobuz.NewWithConfig(server.URL, "issued-app", "")
	if _, err := provider.FetchByID(context.Background(), "qobuz", "requested-track"); err == nil {
		t.Fatal("provider returned a different track ID without rejecting it")
	}
}
