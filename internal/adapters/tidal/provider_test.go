package tidal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ommr/ommr/internal/adapters"
)

func TestSearchRequestsTrackRelationshipAndPermittedIncludes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/searchResults/Karan Aujla Boyfriend/relationships/tracks" {
			t.Errorf("search endpoint: %s", r.URL.Path)
		}
		if r.URL.Query().Get("include") != "tracks.artists,tracks.albums.coverArt" || r.Header.Get("Authorization") != "Bearer fixture" {
			t.Errorf("missing includes/auth: %s", r.URL)
		}
		_, _ = w.Write([]byte(`{"data":[{"type":"tracks","id":"1"}],"included":[{"type":"tracks","id":"1","attributes":{"title":"Boyfriend","duration":"PT2M41S"},"relationships":{"artists":{"data":[{"type":"artists","id":"a"}]}}},{"type":"artists","id":"a","attributes":{"name":"Karan Aujla"}}]}`))
	}))
	defer server.Close()
	got, err := NewWithConfig(server.URL, "fixture").Search(context.Background(), adapters.Query{Artist: "Karan Aujla", Title: "Boyfriend"})
	if err != nil || len(got) != 1 || got[0].Track.Artists[0].Name != "Karan Aujla" || got[0].Track.DurationMS != 161000 {
		t.Fatalf("search: %+v %v", got, err)
	}
}
