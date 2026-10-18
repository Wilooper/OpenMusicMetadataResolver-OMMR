package deezer

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/ommr/ommr/internal/adapters"
)

type searchTransport func(*http.Request) (*http.Response, error)

func (f searchTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCapturedTextSearch(t *testing.T) {
	body, err := os.ReadFile("testdata/boyfriend-search-captured.json")
	if err != nil {
		t.Fatal(err)
	}
	p := New()
	p.httpClient.Transport = searchTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("q") != "Karan Aujla Boyfriend" || r.URL.Query().Get("limit") != "5" {
			t.Fatalf("search query: %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(body)))}, nil
	})
	candidates, err := p.Search(context.Background(), adapters.Query{Artist: "Karan Aujla", Title: "Boyfriend"})
	if err != nil || len(candidates) != 1 {
		t.Fatalf("search: %v %v", candidates, err)
	}
	track := candidates[0].Track
	if track.IDs["deezer"] != "3495188951" || track.Title != "Boyfriend" || track.Artists[0].Name != "Karan Aujla" || track.DurationMS != 160000 {
		t.Fatalf("metadata: %+v", track)
	}
}

func TestISRCSearchRemainsExact(t *testing.T) {
	p := New()
	p.httpClient.Transport = searchTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("q") != `isrc:"USQX91300108"` {
			t.Fatalf("ISRC query: %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":[],"total":0}`))}, nil
	})
	_, err := p.Search(context.Background(), adapters.Query{ISRC: "USQX91300108"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCapturedSearchExcludesTributeArtist(t *testing.T) {
	body, err := os.ReadFile("testdata/get-lucky-search-captured.json")
	if err != nil {
		t.Fatal(err)
	}
	p := New()
	p.httpClient.Transport = searchTransport(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Accept-Language") != "en" {
			t.Fatal("missing catalog language header")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(body)))}, nil
	})
	candidates, err := p.Search(context.Background(), adapters.Query{Artist: "Daft Punk", Title: "Get Lucky"})
	if err != nil || len(candidates) != 2 {
		t.Fatalf("expected original radio and album recordings: %+v %v", candidates, err)
	}
	for _, candidate := range candidates {
		if candidate.Track.Artists[0].Name != "Daft Punk" || candidate.Track.IDs["deezer"] == "1819285597" {
			t.Fatalf("tribute accepted: %+v", candidate.Track)
		}
	}
	if candidates[0].Track.DurationMS != 248000 || candidates[1].Track.DurationMS != 367000 {
		t.Fatalf("recording durations: %d, %d", candidates[0].Track.DurationMS, candidates[1].Track.DurationMS)
	}
}
