package jiosaavn

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/provider"
)

const sampleJioSaavnSearch = `{
  "total": 1,
  "start": 1,
  "results": [
    {
      "id": "6uEI9gj0",
      "type": "",
      "song": "Get Lucky (feat. Pharrell Williams and Nile Rodgers)",
      "album": "Random Access Memories",
      "albumid": "1140957",
      "year": "2013",
      "music": "Pharrell Williams, Nile Rodgers, Thomas Bangalter, Guy-Manuel De Homem-Christo",
      "primary_artists": "Daft Punk, Pharrell Williams, Nile Rodgers",
      "featured_artists": "Pharrell Williams, Nile Rodgers",
      "singers": "Daft Punk",
      "starring": "",
      "image": "https://c.saavncdn.com/087/album-150x150.jpg",
      "label": "Columbia",
      "language": "english",
      "play_count": "696888",
      "perma_url": "https://www.jiosaavn.com/song/get-lucky-feat.-pharrell-williams-and-nile-rodgers/Rh0ueE1XXQM",
      "album_url": "https://www.jiosaavn.com/album/random-access-memories/1140957",
      "duration": "367",
      "release_date": "2013-04-18",
      "explicit_content": "0",
      "copyright_text": "© 2013 Columbia Records"
    }
  ]
}`

func TestSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("__call") != "search.getResults" {
			t.Errorf("unexpected __call: %q", r.URL.Query().Get("__call"))
		}
		if r.URL.Query().Get("q") != "Daft Punk Get Lucky" {
			t.Errorf("unexpected query: %q", r.URL.Query().Get("q"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleJioSaavnSearch))
	}))
	defer srv.Close()

	origURL := SearchURL
	SearchURL = srv.URL
	defer func() { SearchURL = origURL }()

	p := New()
	p.httpClient = srv.Client()

	cands, err := p.Search(context.Background(), adapters.Query{Artist: "Daft Punk", Title: "Get Lucky"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cands) != 1 {
		t.Fatalf("expected 1 result, got %d", len(cands))
	}

	tr := cands[0].Track
	if tr.Title != "Get Lucky (feat. Pharrell Williams and Nile Rodgers)" {
		t.Errorf("unexpected title %q", tr.Title)
	}
	if tr.Artists[0].Name != "Daft Punk" {
		t.Errorf("expected first artist Daft Punk, got %q", tr.Artists[0].Name)
	}
	if tr.IDs["jiosaavn"] != "Rh0ueE1XXQM" {
		t.Errorf("expected perma_url-derived id, got %q", tr.IDs["jiosaavn"])
	}
	if tr.DurationMS != 367000 {
		t.Errorf("expected duration 367000, got %d", tr.DurationMS)
	}
	if tr.PlayCount != 696888 {
		t.Errorf("expected play count 696888, got %d", tr.PlayCount)
	}
	if tr.Label != "Columbia" {
		t.Errorf("expected label Columbia, got %q", tr.Label)
	}
	if tr.Album.Title != "Random Access Memories" {
		t.Errorf("expected album RAM, got %q", tr.Album.Title)
	}
	if len(tr.Images) == 0 || tr.Images[0].Width != 500 {
		t.Errorf("expected upscaled 500px image, got %+v", tr.Images)
	}
}

func TestFetchByID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("__call") != "song.getDetails" {
			t.Errorf("unexpected __call: %q", r.URL.Query().Get("__call"))
		}
		var search provider.JioSaavnSearchResponse
		_ = json.Unmarshal([]byte(sampleJioSaavnSearch), &search)
		payload := map[string]provider.JioSaavnSongResult{search.Results[0].ID: search.Results[0]}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	origURL := SearchURL
	SearchURL = srv.URL
	defer func() { SearchURL = origURL }()

	p := New()
	p.httpClient = srv.Client()

	cand, err := p.FetchByID(context.Background(), "jiosaavn", "6uEI9gj0")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cand.MatchScore != 1.0 {
		t.Errorf("expected match score 1.0, got %f", cand.MatchScore)
	}
	if cand.Track.Title != "Get Lucky (feat. Pharrell Williams and Nile Rodgers)" {
		t.Errorf("unexpected title %q", cand.Track.Title)
	}
}

func TestJioSaavnID(t *testing.T) {
	r := provider.JioSaavnSongResult{
		ID:       "6uEI9gj0",
		PermaURL: "https://www.jiosaavn.com/song/get-lucky/Rh0ueE1XXQM",
	}
	if got := jioSaavnID(r); got != "Rh0ueE1XXQM" {
		t.Errorf("expected Rh0ueE1XXQM, got %q", got)
	}
	r.PermaURL = ""
	if got := jioSaavnID(r); got != "6uEI9gj0" {
		t.Errorf("expected fallback id 6uEI9gj0, got %q", got)
	}
}

func TestUpscaleImage(t *testing.T) {
	in := "https://c.saavncdn.com/087/album-150x150.jpg"
	got := upscaleImage(in, 500)
	want := "https://c.saavncdn.com/087/album-500x500.jpg"
	if got != want {
		t.Errorf("upscaleImage(%q) = %q; want %q", in, got, want)
	}
}
