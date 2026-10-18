package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/config"
	"github.com/ommr/ommr/internal/models/canonical"
)

type extractionFixtureAdapter struct {
	name  string
	track canonical.Track
}

func (a extractionFixtureAdapter) Name() string    { return a.name }
func (a extractionFixtureAdapter) Version() string { return "fixture-v1" }
func (a extractionFixtureAdapter) FetchByID(context.Context, string, string) (*canonical.TrackCandidate, error) {
	return &canonical.TrackCandidate{Provider: a.name, Track: a.track}, nil
}
func (a extractionFixtureAdapter) Search(context.Context, adapters.Query) ([]canonical.TrackCandidate, error) {
	return []canonical.TrackCandidate{{Provider: a.name, Track: a.track}}, nil
}

func TestHandleExtractReportsPerSourceFieldsWithoutFillingMissingCredits(t *testing.T) {
	track := canonical.Track{
		Title:       "Example track",
		Artists:     []canonical.Artist{{Name: "Example artist"}},
		Album:       canonical.Album{Title: "Example album", ReleaseDate: "2024-02-03"},
		Images:      []canonical.Image{{URL: "https://img.example/cover.jpg", Type: "thumbnail"}},
		Credits:     []canonical.Credit{{Name: "Writer", Roles: []string{"Lyricist"}}},
		DurationMS:  123000,
		ReleaseDate: "2024-02-03",
	}
	registry := adapters.NewRegistry()
	registry.Register(extractionFixtureAdapter{name: "fixture", track: track})
	handler := NewHandler(nil, registry, &config.Config{ProviderTimeout: time.Second})
	req := httptest.NewRequest("GET", "/v1/extract?provider=fixture&id=abc&debug=true", nil)
	response := httptest.NewRecorder()
	handler.HandleExtract(response, req)
	if response.Code != 200 {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var got extractionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Sources) != 1 || got.Sources[0].Status != "ok" || len(got.Sources[0].Results) != 1 {
		t.Fatalf("unexpected extraction response: %+v", got)
	}
	result := got.Sources[0].Results[0]
	if result.Year != 2024 || result.ThumbnailURL != "https://img.example/cover.jpg" {
		t.Fatalf("year/artwork not normalized: %+v", result)
	}
	for _, field := range result.PresentFields {
		if field == "lyricist" {
			return
		}
	}
	t.Fatalf("lyricist credit should be reported as present: %+v", result)
}

func TestExtractionLinksAreScopedToTheExtractedProvider(t *testing.T) {
	registry := adapters.NewRegistry()
	registry.Register(extractionFixtureAdapter{name: "tidal", track: canonical.Track{Title: "Song", IDs: map[string]string{"tidal": "123", "spotify": "4cOdK2wGLETKBW3PvgPWqT"}}})
	handler := NewHandler(nil, registry, &config.Config{ProviderTimeout: time.Second})
	response := httptest.NewRecorder()
	handler.HandleExtract(response, httptest.NewRequest("GET", "/v1/extract?provider=tidal&id=123&debug=true", nil))
	var got extractionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	result := got.Sources[0].Results[0]
	if len(result.Links.Platforms) != 1 || result.Links.Platforms["tidal"].URL != "https://tidal.com/track/123" || result.Track.IDs["spotify"] != "" {
		t.Fatal("extraction leaked another provider's identity")
	}
}

func TestDescribeExtractedTrackMarksUnavailableMetadata(t *testing.T) {
	result := describeExtractedTrack(canonical.Track{Title: "Title", Artists: []canonical.Artist{{Name: "Artist"}}})
	if len(result.MissingFields) == 0 {
		t.Fatal("expected incomplete source fields to be reported")
	}
	for _, field := range result.MissingFields {
		if field == "lyricist" {
			return
		}
	}
	t.Fatalf("missing lyricist not reported: %+v", result)
}

func TestExtractionSelectsLargestThumbnail(t *testing.T) {
	got := describeExtractedTrack(canonical.Track{Images: []canonical.Image{{URL: "small", Type: "thumbnail", Width: 120, Height: 90}, {URL: "cover", Type: "cover", Width: 1000, Height: 1000}, {URL: "large", Type: "thumbnail", Width: 480, Height: 360}}})
	if got.ThumbnailURL != "large" {
		t.Fatalf("thumbnail = %s", got.ThumbnailURL)
	}
}

type idKindFixture struct {
	extractionFixtureAdapter
	got *string
}

func (a idKindFixture) FetchByID(ctx context.Context, kind, id string) (*canonical.TrackCandidate, error) {
	*a.got = kind
	return a.extractionFixtureAdapter.FetchByID(ctx, kind, id)
}
func TestJioSaavnExplicitIDKindAcrossGETAndGateway(t *testing.T) {
	for _, gateway := range []bool{false, true} {
		got := ""
		registry := adapters.NewRegistry()
		registry.Register(idKindFixture{extractionFixtureAdapter: extractionFixtureAdapter{name: "jiosaavn", track: canonical.Track{Title: "Song"}}, got: &got})
		handler := NewHandler(nil, registry, &config.Config{ProviderTimeout: time.Second})
		response := httptest.NewRecorder()
		if gateway {
			handler.HandleQuery(response, httptest.NewRequest("POST", "/v1/query", strings.NewReader(`{"operation":"extract","provider":"jiosaavn","id":"PUBLIC42","id_type":"share"}`)))
		} else {
			handler.HandleExtract(response, httptest.NewRequest("GET", "/v1/extract?provider=jiosaavn&id=PUBLIC42&id_type=share", nil))
		}
		if response.Code != 200 || got != "jiosaavn_share" {
			t.Fatalf("gateway=%v status=%d kind=%q body=%s", gateway, response.Code, got, response.Body.String())
		}
		for _, query := range []string{"provider=jiosaavn&id=abc&id_type=wrong", "provider=deezer&id=123&id_type=share", "provider=jiosaavn&id=abc&id_type=share&id_type=native", "artist=A&title=B&id_type=share"} {
			response = httptest.NewRecorder()
			handler.HandleExtract(response, httptest.NewRequest("GET", "/v1/extract?"+query, nil))
			if response.Code != 400 {
				t.Fatalf("invalid identifier kind accepted: %s", query)
			}
		}
	}
}
