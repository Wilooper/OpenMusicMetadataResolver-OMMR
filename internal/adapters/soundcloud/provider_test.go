package soundcloud

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ommr/ommr/internal/adapters"
)

const sampleSoundCloudOembed = `{
  "version": 1.0,
  "type": "rich",
  "provider_name": "SoundCloud",
  "height": 400,
  "width": 100,
  "title": "Midnight City by M83",
  "thumbnail_url": "https://i1.sndcdn.com/artworks-000123-large.jpg",
  "author_name": "M83",
  "author_url": "https://soundcloud.com/m83"
}`

func TestFetchByID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("url") != "https://soundcloud.com/m83/midnight-city" {
			t.Errorf("unexpected url param: %q", r.URL.Query().Get("url"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleSoundCloudOembed))
	}))
	defer srv.Close()

	p := New()
	p.httpClient = srv.Client()

	// Patch the oembed base URL so the test hits the local server.
	origURL := OembedURL
	OembedURL = srv.URL
	defer func() { OembedURL = origURL }()

	cand, err := p.FetchByID(context.Background(), "soundcloud", "m83/midnight-city")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cand.Track.Title != "Midnight City" {
		t.Errorf("expected title 'Midnight City', got %q", cand.Track.Title)
	}
	if cand.Track.Artists[0].Name != "M83" {
		t.Errorf("expected artist M83, got %q", cand.Track.Artists[0].Name)
	}
	if cand.Track.IDs["soundcloud"] != "m83/midnight-city" {
		t.Errorf("expected soundcloud permalink id, got %q", cand.Track.IDs["soundcloud"])
	}
}

func TestPermalinkHelpers(t *testing.T) {
	tests := []struct {
		in, out string
	}{
		{"https://soundcloud.com/m83/midnight-city", "m83/midnight-city"},
		{"m83/midnight-city", "m83/midnight-city"},
	}
	for _, tt := range tests {
		if got := permalinkOf(normalizeTrackURL(tt.in)); got != tt.out {
			t.Errorf("permalinkOf(normalizeTrackURL(%q)) = %q; want %q", tt.in, got, tt.out)
		}
	}
}

func TestSearchReturnsNilWithoutID(t *testing.T) {
	p := New()
	cands, err := p.Search(context.Background(), adapters.Query{Title: "Midnight City"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cands) != 0 {
		t.Errorf("expected no search results, got %d", len(cands))
	}
}

func TestNativeTrackIDRequiresSoundCloudWidgetAndTrackResource(t *testing.T) {
	for _, tt := range []struct{ html, want string }{
		{`<iframe src="https://w.soundcloud.com/player/?url=https%3A%2F%2Fapi.soundcloud.com%2Ftracks%2F123456&amp;auto_play=false"></iframe>`, "123456"},
		{`<iframe src="https://evil.test/player/?url=https%3A%2F%2Fapi.soundcloud.com%2Ftracks%2F123456"></iframe>`, ""},
		{`<iframe src="https://w.soundcloud.com/player/?url=https%3A%2F%2Fapi.soundcloud.com%2Fplaylists%2F123456"></iframe>`, ""},
		{`<iframe src="https://w.soundcloud.com/player/?url=https%3A%2F%2Fapi.soundcloud.com%2Ftracks%2F123456%3Fsecret_token%3Dsecret"></iframe>`, ""},
	} {
		if got := nativeTrackID(tt.html); got != tt.want {
			t.Errorf("nativeTrackID=%q;want %q", got, tt.want)
		}
	}
}
