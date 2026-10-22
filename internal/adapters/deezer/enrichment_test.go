package deezer

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/canonical"
)

type detailTransport func(*http.Request) (*http.Response, error)

func (f detailTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDetailLookupRejectsAnotherRecordingID(t *testing.T) {
	p := New()
	p.httpClient = &http.Client{Transport: detailTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"id":999,"title":"Other song","artist":{"name":"Singer"}}`)), Header: http.Header{}}, nil
	})}
	if _, err := p.FetchByID(context.Background(), "deezer", "123"); err == nil {
		t.Fatal("wrong recording ID accepted")
	}
}
func TestSearchCandidateRequestsDetailsButDirectRecordSkipsDuplicate(t *testing.T) {
	p := New()
	calls := 0
	p.httpClient = &http.Client{Transport: detailTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"id":123,"title":"Song","isrc":"USAAA2500001","duration":160,"artist":{"id":1,"name":"Singer"},"album":{"id":3,"title":"Album"}}`)), Header: http.Header{}}, nil
	})}
	candidate := canonical.TrackCandidate{Provider: "deezer", Track: canonical.Track{IDs: map[string]string{"deezer": "123"}}, RawResponse: []byte(`{"data":[{"id":123}]}`)}
	got, err := p.EnrichCandidate(context.Background(), candidate)
	if err != nil || got == nil || got.Track.ISRC != "USAAA2500001" {
		t.Fatalf("details: %+v %v", got, err)
	}
	if next, err := p.EnrichCandidate(context.Background(), *got); next != nil || err != nil || calls != 1 {
		t.Fatal("direct result duplicated detail request")
	}
}

func TestSearchAndDetailRecoverEmbeddedHTMLAsNormalCandidates(t *testing.T) {
	p := New()
	p.web = nil
	body := `<script type="application/json">{"data":[{"id":123,"title":"Song","artist":{"name":"Singer"},"extra_field":"retain"}]}</script>`
	p.httpClient = &http.Client{Transport: detailTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	results, err := p.Search(context.Background(), adapters.Query{Title: "Song", Artist: "Singer"})
	if err != nil || len(results) != 1 || results[0].Track.IDs["deezer"] != "123" {
		t.Fatalf("HTML search: %+v %v", results, err)
	}
	body = `<script type="application/ld+json">{"@type":"MusicRecording","url":"https://www.deezer.com/track/123","name":"Song","byArtist":[{"name":"Singer"},{"name":"Collaborator"}],"duration":"PT3M19S","extra_field":"retain"}</script>`
	result, err := p.FetchByID(context.Background(), "deezer", "123")
	if err != nil || result.Track.DurationMS != 199000 {
		t.Fatalf("HTML detail: %+v %v", result, err)
	}
	if len(result.Track.Artists) != 2 || len(result.Track.Artists[0].IDs) != 0 || len(result.Track.Artists[1].IDs) != 0 || len(result.Track.Album.IDs) != 0 {
		t.Fatal("missing artist/album IDs became fabricated zeros")
	}
	if _, err = p.FetchByID(context.Background(), "deezer", "456"); err == nil {
		t.Fatal("HTML detail returned another ID")
	}
}

func TestUnavailableAPIUsesOnePublicPageFallback(t *testing.T) {
	p := New()
	p.httpClient = &http.Client{Transport: detailTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`<html><h1>Site Unavailable</h1></html>`))}, nil
	})}
	calls := 0
	p.web.SetTransport(detailTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "www.deezer.com" {
			t.Fatal("wrong fallback host")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`<script type="application/ld+json">{"@type":"MusicRecording","url":"https://www.deezer.com/track/123","name":"Song","byArtist":{"name":"Singer"}}</script>`))}, nil
	}))
	results, err := p.Search(context.Background(), adapters.Query{Artist: "Singer", Title: "Song"})
	if err != nil || calls != 1 || len(results) != 1 || results[0].Track.IDs["deezer"] != "123" {
		t.Fatalf("public recovery: %+v %v calls=%d", results, err, calls)
	}
}
