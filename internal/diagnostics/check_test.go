package diagnostics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/adapters/jiosaavn"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestObserverPreservesBytesAndHidesSecrets(t *testing.T) {
	body := []byte(`{"results":[{"id":"native42","song":"Song"}]}`)
	transport := &Transport{Base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Proto: "HTTP/2.0", Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader(body))}, nil
	})}
	req, _ := http.NewRequest("GET", "https://user:password@www.jiosaavn.com/api.php?__call=search.getResults&q=Song&access_token=private&cookie=private#private", nil)
	req.Header.Set("Authorization", "private")
	response, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if !bytes.Equal(body, got) {
		t.Fatal("observer changed adapter input")
	}
	raw, _ := json.Marshal(transport.events)
	if strings.Contains(string(raw), "private") || strings.Contains(string(raw), "password") || strings.Contains(string(raw), "user:") {
		t.Fatal("secrets leaked")
	}
	if transport.events[0].Kind != "json" || len(transport.events[0].SHA256) != 64 {
		t.Fatalf("missing evidence: %+v", transport.events)
	}
}

func TestStatusAndBodyAreIndependent(t *testing.T) {
	cases := []struct {
		body   string
		status int
		kind   string
	}{
		{`<html><h1>Site Unavailable</h1></html>`, 200, "site_unavailable_html"},
		{`{"error":{"message":"bad request"}}`, 200, "json_api_error"},
		{`{"error":null,"results":[]}`, 200, "json"},
		{`{}`, 405, "method_not_allowed"},
		{`{}`, 429, "rate_limited"},
		{`<?xml version="1.0"?><metadata/>`, 200, "xml"},
	}
	for _, c := range cases {
		if got := classify([]byte(c.body), c.status); got != c.kind {
			t.Errorf("got %s want %s", got, c.kind)
		}
	}
}

func TestCancellationAndOversizedResponse(t *testing.T) {
	called := false
	transport := &Transport{Base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		called = true
		return nil, errors.New("private proxy data")
	})}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://musicbrainz.org/", nil)
	if _, err := transport.RoundTrip(req); err == nil || called {
		t.Fatal("cancelled request reached network")
	}
	if transport.events[0].Kind != "deadline_or_cancelled" {
		t.Fatal("cancellation misclassified")
	}
	transport = &Transport{Base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", maxBody+1)))}, nil
	})}
	req, _ = http.NewRequest("GET", "https://musicbrainz.org/", nil)
	if _, err := transport.RoundTrip(req); err == nil || transport.events[0].Kind != "body_too_large" {
		t.Fatal("body limit not enforced")
	}
}

func TestRealAdapterOutcomeNotHTTP200DefinesSuccess(t *testing.T) {
	for _, body := range []string{`<html><h1>Site Unavailable</h1></html>`, `{"results":[]}`, `{"results":[{"id":"native42","song":"Song","primary_artists":"Singer"}]}`} {
		tr := &Transport{Base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
		})}
		original := http.DefaultTransport
		http.DefaultTransport = tr
		report := Run(context.Background(), jiosaavn.New(), tr, adapters.Query{Artist: "Singer", Title: "Song"}, Options{})
		http.DefaultTransport = original
		if strings.HasPrefix(body, "<") && report.AdapterStatus == "metadata_available" {
			t.Fatal("empty unavailable page counted as catalog")
		}
		if body == `{"results":[]}` && report.AdapterStatus != "empty" {
			t.Fatal("JSON without candidates counted as data")
		}
		if strings.Contains(body, "native42") && report.AdapterStatus != "metadata_available" {
			t.Fatal("valid candidate was not decoded")
		}
		if len(report.Requests) < 1 {
			t.Fatal("adapter HTTP call not observed")
		}
	}
	for _, provider := range []string{"jiosaavn", "deezer", "musicbrainz"} {
		for _, route := range Routes(provider, adapters.Query{Artist: "A&B", Title: "a?b"}) {
			u, err := url.Parse(route.URL)
			if err != nil || u.Scheme != "https" || u.User != nil {
				t.Fatalf("unsafe fixed route: %+v", route)
			}
		}
	}
}
