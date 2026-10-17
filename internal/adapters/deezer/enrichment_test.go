package deezer

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

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
