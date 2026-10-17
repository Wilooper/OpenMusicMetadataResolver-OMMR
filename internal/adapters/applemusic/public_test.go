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
