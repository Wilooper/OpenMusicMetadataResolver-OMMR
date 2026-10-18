package musicbrainz

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/ommr/ommr/internal/adapters"
)

type htmlTransport func(*http.Request) (*http.Response, error)

func (f htmlTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestSearchHTMLRowsBecomeScopedRecordingCandidates(t *testing.T) {
	p := New()
	p.web = nil
	p.httpClient = &http.Client{Transport: htmlTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`<table><tr><td><a href="/recording/12345678-1234-1234-1234-123456789abc">Song</a></td><td><a href="/artist/22345678-1234-1234-1234-123456789abc">Singer</a></td></tr></table>`))}, nil
	})}
	result, err := p.Search(context.Background(), adapters.Query{Artist: "Singer", Title: "Song"})
	if err != nil || len(result) != 1 || result[0].Track.IDs["musicbrainz"] != "12345678-1234-1234-1234-123456789abc" {
		t.Fatalf("HTML recording: %+v %v", result, err)
	}
	if _, err = p.FetchByID(context.Background(), "musicbrainz", "12345678-1234-1234-1234-123456789abc"); err == nil {
		t.Fatal("search row reused as direct detail")
	}
}
