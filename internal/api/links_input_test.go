package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/cache/disk"
	"github.com/ommr/ommr/internal/config"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/resolver"
)

type linkInputAdapter struct {
	name  string
	calls *atomic.Int64
	wrong bool
}

func (a linkInputAdapter) Name() string    { return a.name }
func (a linkInputAdapter) Version() string { return "fixture" }
func (a linkInputAdapter) Search(context.Context, adapters.Query) ([]canonical.TrackCandidate, error) {
	return nil, nil
}
func (a linkInputAdapter) FetchByID(_ context.Context, kind, id string) (*canonical.TrackCandidate, error) {
	a.calls.Add(1)
	ids := map[string]string{a.name: id}
	if a.name == "jiosaavn" {
		ids = map[string]string{"jiosaavn": "ERIkRAwJf38", "jiosaavn_track_id": "azOux9HL"}
	}
	if a.wrong {
		ids = map[string]string{a.name: "wrong"}
	}
	return &canonical.TrackCandidate{Provider: a.name, Track: canonical.Track{Title: "Song", Artists: []canonical.Artist{{Name: "Singer"}}, IDs: ids, DurationMS: 163000}}, nil
}

func linkHandler(t *testing.T, name string, wrong bool) (*Handler, *atomic.Int64) {
	t.Helper()
	calls := &atomic.Int64{}
	reg := adapters.NewRegistry()
	reg.Register(linkInputAdapter{name, calls, wrong})
	backend, err := disk.NewDiskCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	res := resolver.New(reg, backend, nil, time.Hour)
	return NewHandler(res, reg, &config.Config{ProviderTimeout: time.Second, BulkMaxItems: 10, BulkWorkers: 2}), calls
}

func TestShareLinksGETGatewayBulkAndCache(t *testing.T) {
	h, calls := linkHandler(t, "deezer", false)
	for _, tc := range []struct{ method, target, body string }{
		{"GET", "/v1/resolve?url=" + url.QueryEscape("https://www.deezer.com/track/123?utm_source=share"), ""},
		{"POST", "/v1/query", `{"url":"https://www.deezer.com/track/123","operation":"resolve"}`},
		{"POST", "/v1/bulk", `{"queries":[{"url":"https://www.deezer.com/track/123"},{"url":"https://foreign.example/track/123"}]}`},
	} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(tc.method, tc.target, strings.NewReader(tc.body))
		if tc.target == "/v1/query" {
			h.HandleQuery(w, r)
		} else if tc.target == "/v1/bulk" {
			h.HandleBulk(w, r)
		} else {
			h.HandleResolve(w, r)
		}
		if w.Code != 200 {
			t.Fatalf("%s %d %s", tc.target, w.Code, w.Body.String())
		}
		var data map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &data)
		if tc.target == "/v1/bulk" {
			if data["successful"] != float64(1) || data["failed"] != float64(1) {
				t.Fatal("bulk link isolation failed")
			}
		} else {
			track, ok := data["track"].(map[string]any)
			if !ok || track["title"] != "Song" {
				t.Fatalf("missing direct track %s", w.Body.String())
			}
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("tracking variants didn't share cache: %d", calls.Load())
	}
}

func TestInvalidLinksStopBeforeProviderCalls(t *testing.T) {
	h, calls := linkHandler(t, "deezer", false)
	for _, query := range []string{
		"url=" + url.QueryEscape("https://foreign.example/track/123"),
		"url=" + url.QueryEscape("https://www.deezer.com/track/123") + "&spotify_id=other",
		"url=" + url.QueryEscape("https://www.deezer.com/track/123") + "&url=" + url.QueryEscape("https://www.deezer.com/track/456"),
		"youtube_id=" + url.QueryEscape("https://evil.example/watch?v=dQw4w9WgXcQ"),
	} {
		w := httptest.NewRecorder()
		h.HandleResolve(w, httptest.NewRequest("GET", "/v1/resolve?"+query, nil))
		if w.Code != 400 {
			t.Fatalf("accepted invalid link: %d %s", w.Code, w.Body.String())
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid URL reached provider")
	}
}

func TestJioSaavnAndMusicBrainzDirectSeeds(t *testing.T) {
	for _, tc := range []struct{ name, query string }{
		{"jiosaavn", "url=" + url.QueryEscape("https://www.jiosaavn.com/song/mera-mann/ERIkRAwJf38")},
		{"jiosaavn", "jiosaavn_id=azOux9HL"},
		{"musicbrainz", "url=" + url.QueryEscape("https://musicbrainz.org/recording/047ea202-b98d-46ae-97f7-0180a20ee5cf")},
	} {
		for _, wrong := range []bool{false, true} {
			h, _ := linkHandler(t, tc.name, wrong)
			w := httptest.NewRecorder()
			h.HandleResolve(w, httptest.NewRequest("GET", "/v1/resolve?"+tc.query+"&debug=true", nil))
			if w.Code != 200 {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
			var data map[string]any
			_ = json.Unmarshal(w.Body.Bytes(), &data)
			if (data["track"] != nil) == wrong {
				t.Fatalf("wrong direct seed decision: wrong=%v %s", wrong, w.Body.String())
			}
		}
	}
}
