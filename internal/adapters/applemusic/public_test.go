package applemusic

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/ommr/ommr/internal/adapters"
)

type publicTransport func(*http.Request) (*http.Response, error)

func (f publicTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPublicSearchAndLookupUseConfiguredStorefront(t *testing.T) {
	p := NewWithCatalog("", "in")
	p.httpClient = &http.Client{Transport: publicTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("country") != "in" {
			t.Errorf("storefront ignored: %s", r.URL)
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("keyless request contains authorization")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"results":[{"wrapperType":"track","kind":"song","trackId":1829591986,"trackName":"Boyfriend","artistName":"Karan Aujla & Ikky"}]}`)), Header: http.Header{}}, nil
	})}
	got, err := p.Search(context.Background(), adapters.Query{Artist: "Karan Aujla", Title: "Boyfriend"})
	if err != nil || len(got) != 1 {
		t.Fatalf("search: %v", err)
	}
	if _, err := p.FetchByID(context.Background(), "applemusic", "1829591986"); err != nil {
		t.Fatal(err)
	}
}

func TestSongCountsCannotBecomeAlbumWideTotals(t *testing.T) {
	// Public P-POP CULTURE reports 5/6 on its song rows, but 11 on the collection.
	// DAMN. reports 14 songs on a row versus 15 collection items including bonus media.
	for _, fixture := range []string{
		`{"results":[{"wrapperType":"track","kind":"song","trackId":1829591986,"artistId":914892130,"trackName":"Boyfriend","artistName":"Karan Aujla & Ikky","collectionId":1829591921,"collectionName":"P-POP CULTURE","discNumber":1,"discCount":2,"trackCount":5}]}`,
		`{"results":[{"wrapperType":"track","kind":"song","trackId":1829591997,"artistId":914892130,"trackName":"P-POP CULTURE","artistName":"Karan Aujla & Ikky","collectionId":1829591921,"collectionName":"P-POP CULTURE","discNumber":2,"discCount":2,"trackCount":6}]}`,
		`{"results":[{"wrapperType":"track","kind":"song","trackId":1440882165,"artistId":368183298,"trackName":"HUMBLE.","artistName":"Kendrick Lamar","collectionId":1440881722,"collectionName":"DAMN.","discNumber":1,"discCount":1,"trackCount":14}]}`,
	} {
		p := New()
		p.httpClient = &http.Client{Transport: publicTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(fixture))}, nil
		})}
		got, err := p.Search(context.Background(), adapters.Query{Title: "Song"})
		if err != nil || len(got) != 1 {
			t.Fatal(err)
		}
		if got[0].Track.Album.TotalTracks != 0 {
			t.Fatal("song-row count mislabeled as whole-album total")
		}
		if !strings.Contains(string(got[0].RawResponse), `"trackCount"`) {
			t.Fatal("source count discarded")
		}
	}
}
