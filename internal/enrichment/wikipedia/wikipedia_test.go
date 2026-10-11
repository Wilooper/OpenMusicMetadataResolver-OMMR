package wikipedia

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type fixtureTransport func(*http.Request) (*http.Response, error)

func (f fixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestWikidataLinkProducesAttributedContext(t *testing.T) {
	c := New()
	c.httpClient.Transport = fixtureTransport(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Fatal("credentials sent to context provider")
		}
		body := `{"type":"standard","title":"Song","extract":"Background about the work.","revision":123,"wikibase_item":"Q123"}`
		if r.URL.Host == "www.wikidata.org" {
			if r.URL.Path != "/wiki/Special:EntityData/Q123.json" {
				t.Fatalf("unexpected Wikidata path: %s", r.URL.Path)
			}
			body = `{"entities":{"Q123":{"sitelinks":{"enwiki":{"title":"Song"}}}}}`
		} else if r.URL.Host != "en.wikipedia.org" || r.URL.Path != "/api/rest_v1/page/summary/Song" {
			t.Fatalf("unexpected summary URL: %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})
	got, err := c.FetchFromWikidata(context.Background(), "https://www.wikidata.org/wiki/Q123")
	if err != nil {
		t.Fatal(err)
	}
	if got.WikidataID != "Q123" || got.Revision != "123" || got.Source != "musicbrainz_work_wikidata_link" || got.ArticleURL != "https://en.wikipedia.org/wiki/Song" {
		t.Fatalf("wrong context: %+v", got)
	}
}

func TestWikipediaDisambiguationIsNotSongContext(t *testing.T) {
	c := New()
	c.httpClient.Transport = fixtureTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"type":"disambiguation","title":"Song","extract":"Song may refer to many things"}`)), Header: http.Header{}}, nil
	})
	if _, err := c.Fetch(context.Background(), "https://en.wikipedia.org/wiki/Song"); err == nil {
		t.Fatal("disambiguation accepted")
	}
}

func TestWikipediaEntityMismatchIsRejected(t *testing.T) {
	c := New()
	c.httpClient.Transport = fixtureTransport(func(r *http.Request) (*http.Response, error) {
		body := `{"entities":{"Q123":{"sitelinks":{"enwiki":{"title":"Song"}}}}}`
		if r.URL.Host == "en.wikipedia.org" {
			body = `{"type":"standard","title":"Song","extract":"Different work.","wikibase_item":"Q456"}`
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	if _, err := c.FetchFromWikidata(context.Background(), "https://www.wikidata.org/wiki/Q123"); err == nil {
		t.Fatal("article for different entity accepted")
	}
}

func TestRejectsUnverifiedOrExternalArticleURLs(t *testing.T) {
	for _, u := range []string{"https://en.wikipedia.org.evil.test/wiki/Song", "http://en.wikipedia.org/wiki/Song", "https://en.wikipedia.org/wiki/Song/other"} {
		if _, err := Fetch(context.Background(), u); err == nil {
			t.Fatalf("accepted unsafe URL: %s", u)
		}
	}
}

func TestWikidataRequiresExactEntityHostAndID(t *testing.T) {
	for _, u := range []string{"https://www.wikidata.org.evil.test/wiki/Q1", "http://www.wikidata.org/wiki/Q1", "https://www.wikidata.org/wiki/Q1/other"} {
		if _, err := FetchFromWikidata(context.Background(), u); err == nil {
			t.Fatalf("accepted unsafe URL: %s", u)
		}
	}
}
