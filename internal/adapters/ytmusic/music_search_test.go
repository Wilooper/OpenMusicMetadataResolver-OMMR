package ytmusic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/metadata"
)

func TestMusicSearchPrefersCatalogSongs(t *testing.T) {
	fixture, err := os.ReadFile("testdata/music_search.json")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		client := payload["context"].(map[string]any)["client"].(map[string]any)
		if client["clientName"] != "WEB_REMIX" || payload["params"] == "" {
			t.Errorf("unfiltered music search: %v", payload)
		}
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Error("public search sent credentials")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()
	old := musicSearchURL
	musicSearchURL = srv.URL
	defer func() { musicSearchURL = old }()
	p := New()
	p.innerTube.httpClient = srv.Client()
	candidates, err := p.Search(context.Background(), adapters.Query{Artist: "Karan Aujla", Title: "Boyfriend"})
	if err != nil || len(candidates) == 0 {
		t.Fatalf("search: %v, %d candidates", err, len(candidates))
	}
	tr := candidates[0].Track
	if tr.Title != "Boyfriend" || tr.IDs[ProviderName] != "tIjUfo30r4M" || tr.Album.Title != "P-POP CULTURE" || tr.DurationMS != 161000 {
		t.Fatalf("unexpected song: %+v", tr)
	}
	if len(tr.Artists) != 2 || tr.Artists[0].Name != "Karan Aujla" || tr.Artists[1].Name != "Ikky" {
		t.Fatalf("artists: %+v", tr.Artists)
	}
	if calls != 1 {
		t.Fatalf("unnecessary video fallback: %d calls", calls)
	}
	record := metadata.Record(candidates[0])
	raw, _ := json.Marshal(record["catalog"])
	if !strings.Contains(string(raw), "436M plays") || strings.Contains(string(raw), "trackingParams") {
		t.Fatalf("catalog extras: %s", raw)
	}
}

func TestMusicSearchFallsBackOnUnavailableResponse(t *testing.T) {
	for _, body := range []string{`<html>Site Unavailable</html>`, `{"contents":{}}`, `{"error":{"code":403}}`} {
		t.Run(body, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/music" {
					_, _ = w.Write([]byte(body))
					return
				}
				_, _ = w.Write([]byte(sampleSearchResponse))
			}))
			defer srv.Close()
			oldMusic, oldVideo := musicSearchURL, innerTubeBaseURL
			musicSearchURL, innerTubeBaseURL = srv.URL+"/music", srv.URL
			defer func() { musicSearchURL, innerTubeBaseURL = oldMusic, oldVideo }()
			p := New()
			p.innerTube.httpClient = srv.Client()
			got, err := p.Search(context.Background(), adapters.Query{Artist: "Talwiinder", Title: "Tu"})
			if err != nil || len(got) != 1 || got[0].Track.IDs[ProviderName] != "FVNSACXFAy0" {
				t.Fatalf("fallback: %+v, %v", got, err)
			}
		})
	}
}

func TestMusicSearchRejectsNonSongRows(t *testing.T) {
	fixture, _ := os.ReadFile("testdata/music_search.json")
	var response any
	_ = json.Unmarshal(fixture, &response)
	var find func(any) map[string]any
	find = func(v any) map[string]any {
		switch n := v.(type) {
		case map[string]any:
			if row, ok := n["musicResponsiveListItemRenderer"].(map[string]any); ok {
				return row
			}
			for _, child := range n {
				if row := find(child); row != nil {
					return row
				}
			}
		case []any:
			for _, child := range n {
				if row := find(child); row != nil {
					return row
				}
			}
		}
		return nil
	}
	row := find(response)
	raw, _ := json.Marshal(row)
	raw = []byte(strings.ReplaceAll(string(raw), "MUSIC_VIDEO_TYPE_ATV", "MUSIC_VIDEO_TYPE_OMV"))
	_ = json.Unmarshal(raw, &row)
	if _, ok := parseMusicItem(row); ok {
		t.Fatal("music video accepted as catalog song")
	}
}
