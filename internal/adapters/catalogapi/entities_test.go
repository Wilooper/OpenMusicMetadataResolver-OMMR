package catalogapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/metadata"
	"github.com/ommr/ommr/internal/models/canonical"
)

const jsonAPISearch = `{"data":[{"type":"tracks","id":"9007199254740993"}],"included":[
 {"type":"albums","id":"unrelated","attributes":{"title":"Wrong album"}},
 {"type":"tracks","id":"9007199254740993","attributes":{"title":"Boyfriend","duration":"PT2M40.796S","isrc":"USAAA2500001","explicit":false,"mediaTags":["HIRES_LOSSLESS"]},"relationships":{"artists":{"data":[{"type":"artists","id":"same"}]},"albums":{"data":[{"type":"albums","id":"album"}]}}},
 {"type":"albums","id":"same","attributes":{"title":"ID collision album"}},
 {"type":"artists","id":"same","attributes":{"name":"Karan Aujla"}},
 {"type":"albums","id":"album","attributes":{"title":"P-POP CULTURE","releaseDate":"2025-08-22"},"relationships":{"coverArt":{"data":{"type":"artworks","id":"cover"}}}},
 {"type":"artworks","id":"cover","attributes":{"files":[{"href":"https://img.example/cover.jpg","width":600,"height":600}]}},
 {"type":"tracks","id":"unrelated-track","attributes":{"title":"Wrong recording","artists":[{"name":"Other"}]}}
]}`

func TestJSONAPISearchAssociatesOnlyReferencedResources(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(jsonAPISearch)) }))
	defer server.Close()
	p := New(Config{Name: "tidal", BaseURL: server.URL, Search: func(adapters.Query) (string, string, []byte, error) { return "GET", "/search", nil, nil }})
	got, err := p.Search(context.Background(), adapters.Query{Title: "Boyfriend", Artist: "Karan Aujla"})
	if err != nil || len(got) != 1 {
		t.Fatalf("results: %+v %v", got, err)
	}
	tr := got[0].Track
	if tr.IDs["tidal"] != "9007199254740993" || tr.DurationMS != 160796 || tr.Artists[0].Name != "Karan Aujla" || tr.Album.IDs["tidal"] != "album" || len(tr.Images) != 1 {
		t.Fatalf("incomplete: %+v", tr)
	}
	record, _ := json.Marshal(metadata.Record(got[0]))
	if strings.Contains(string(record), "Wrong recording") || strings.Contains(string(record), "ID collision album") || !strings.Contains(string(record), "HIRES_LOSSLESS") {
		t.Fatalf("entity projection: %s", record)
	}
	if explicit, known := metadata.Explicit(got[0]); !known || explicit {
		t.Fatal("known clean rating lost")
	}
}
func TestSearchDoesNotTreatNestedAlbumsAsTracks(t *testing.T) {
	var payload any
	_ = json.Unmarshal([]byte(`{"data":[],"included":[{"type":"albums","id":"1","attributes":{"title":"Not a track"},"artists":[{"name":"Artist"}]}]}`), &payload)
	if got := findTrackItems(payload); len(got) != 0 {
		t.Fatalf("unrelated included entities surfaced: %+v", got)
	}
}
func TestDurationParsingPreservesPrecisionAndRejectsBadInput(t *testing.T) {
	for _, tc := range []struct {
		value        any
		milliseconds bool
		want         int64
	}{
		{"PT2M40.796S", false, 160796}, {160.796, false, 160796}, {json.Number("160.796"), false, 160796}, {160796.0, true, 160796}, {"PT", false, 0}, {"P3D", false, 0}, {"NaN", false, 0}, {-5.0, false, 0}, {1e30, false, 0},
	} {
		if got := durationMilliseconds(tc.value, tc.milliseconds); got != tc.want {
			t.Errorf("%v: %d != %d", tc.value, got, tc.want)
		}
	}
}
func TestAmazonParentalRatingRemainsSeparateFromAlbumRating(t *testing.T) {
	item := map[string]any{"id": "track", "title": "Song", "artists": []any{map[string]any{"name": "Artist"}}, "parentalSettings": map[string]any{"hasExplicitLanguage": false}, "album": map[string]any{"explicit": true}}
	track := normalize(item, "amazonmusic")
	raw, _ := json.Marshal(item)
	if explicit, known := metadata.Explicit(canonical.TrackCandidate{Provider: "amazonmusic", Track: track, RawResponse: raw}); !known || explicit {
		t.Fatal("album explicit rating contaminated song")
	}
}
