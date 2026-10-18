package jiosaavn

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type pageRoundTrip func(*http.Request) (*http.Response, error)

func (f pageRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Constructed structured-data fixture, not a capture of the currently inaccessible page.
const meraStructuredPage = `<script type="application/ld+json">{"@type":"MusicRecording","name":"Mera Mann : Juss &amp; MixSingh","url":"https://www.jiosaavn.com/song/mera-mann/ERIkRAwJf38","byArtist":[{"name":"Juss"},{"name":"Mixsingh"}],"inAlbum":{"name":"Mera Mann"},"duration":"PT2M43S","inLanguage":"Punjabi","datePublished":"2024","copyrightNotice":"© 2024 Speed Records (under exclusive assignment to Times Music)","additionalProperty":{"name":"extra","value":"retain"}}</script>`

func TestPublicPageFirstWithoutAPIOrNativeID(t *testing.T) {
	p := New()
	calls := 0
	p.web.SetTransport(pageRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != "https://www.jiosaavn.com/song/mera-mann/ERIkRAwJf38" {
			t.Fatalf("wrong page: %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(meraStructuredPage))}, nil
	}))
	result, err := p.FetchByID(context.Background(), "jiosaavn_url", "https://www.jiosaavn.com/song/mera-mann/ERIkRAwJf38")
	if err != nil {
		t.Fatal(err)
	}
	track := result.Track
	if calls != 1 || track.DurationMS != 163000 || len(track.Artists) != 2 || track.Album.Title != "Mera Mann" || track.Language != "Punjabi" || track.ReleaseDate != "2024" || len(track.Copyrights) != 1 {
		t.Fatalf("page metadata: calls=%d track=%+v", calls, track)
	}
	if track.IDs["jiosaavn_track_id"] != "" || track.IDs[ProviderName] != "ERIkRAwJf38" {
		t.Fatal("native ID invented")
	}
	if !strings.Contains(string(result.RawResponse), "additionalProperty") {
		t.Fatal("source extras lost")
	}
}

func TestUnavailablePageUsesExactShareAPI(t *testing.T) {
	p := New()
	calls := []string{}
	tr := pageRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls = append(calls, r.URL.Path)
		body := `<html><h1>Site Unavailable</h1></html>`
		if r.URL.Path == "/api.php" {
			if r.URL.Query().Get("__call") != "webapi.get" || r.URL.Query().Get("token") != "ERIkRAwJf38" {
				t.Fatalf("wrong fallback: %s", r.URL)
			}
			body = `{"songs":[{"id":"native42","song":"Mera Mann","primary_artists":"Juss, Mixsingh","perma_url":"https://www.jiosaavn.com/song/mera-mann/ERIkRAwJf38"}]}`
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	p.web.SetTransport(tr)
	p.httpClient.Transport = tr
	result, err := p.FetchByID(context.Background(), "jiosaavn_url", "https://www.jiosaavn.com/song/mera-mann/ERIkRAwJf38")
	if err != nil || len(calls) != 2 || calls[0] != "/song/mera-mann/ERIkRAwJf38" || calls[1] != "/api.php" || result.Track.IDs["jiosaavn_track_id"] != "native42" {
		t.Fatalf("wrong page/API order: %+v %v", calls, err)
	}
}

func TestSongURLRejectsOtherTargets(t *testing.T) {
	for _, raw := range []string{"http://www.jiosaavn.com/song/s/id", "https://foreign.example/song/s/id", "https://www.jiosaavn.com@127.0.0.1/song/s/id", "https://www.jiosaavn.com:443/song/s/id", "https://www.jiosaavn.com/album/s/id", "https://www.jiosaavn.com/song/s/id?cookie=secret", "https://www.jiosaavn.com/song/s/id#secret", "https://www.jiosaavn.com/song/%2e%2e/id", "https://www.jiosaavn.com/song/s/id%2fextra"} {
		if _, _, err := ParseSongURL(raw); err == nil {
			t.Errorf("accepted unsafe target %s", raw)
		}
	}
}
