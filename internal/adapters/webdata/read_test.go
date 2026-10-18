package webdata

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const songID = "12345678-1234-1234-1234-123456789abc"
const artistID = "22345678-1234-1234-1234-123456789abc"

func TestNativeJSONAndEmbeddedStateRecovery(t *testing.T) {
	for _, test := range []struct{ provider, mode, id, body string }{
		{"deezer", "search", "", `<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"search":{"data":[{"id":123,"title":"Song","artist":{"id":7,"name":"Singer"},"new_field":"retain"}]}}}}</script>`},
		{"deezer", "track", "123", `<script>window.__DZR_APP_STATE__ = {"track":{"id":123,"title":"Song","artist":{"name":"Singer"}}}; doNotExecute();</script>`},
		{"jiosaavn", "track", "nativeID", `<script type="application/json">{"song":{"id":"nativeID","song":"Song","primary_artists":"Singer"}}</script>`},
		{"musicbrainz", "search", "", `<script type="application/json">{"recordings":[{"id":"` + songID + `","title":"Song","artist-credit":[{"artist":{"name":"Singer"}}]}]}</script>`},
		{"deezer", "search", "", `<html><body><pre>{&quot;data&quot;:[{&quot;id&quot;:123,&quot;title&quot;:&quot;Song&quot;,&quot;artist&quot;:{&quot;name&quot;:&quot;Singer&quot;}}]}</pre></body></html>`},
	} {
		t.Run(test.provider+test.mode, func(t *testing.T) {
			body, err := DecodeHTML([]byte(test.body), Request{test.provider, test.mode, test.id})
			if err != nil || !json.Valid(body) {
				t.Fatalf("recover: %s %v", body, err)
			}
			if test.provider == "jiosaavn" && !strings.Contains(string(body), `"nativeID":`) {
				t.Fatal("detail was not keyed by its own native ID")
			}
		})
	}
	raw := `{"data":[]}`
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(raw))}
	body, err := Read(resp, Request{Provider: "deezer", Mode: "search"})
	if err != nil || string(body) != raw {
		t.Fatal("native empty API response altered")
	}
}
func TestStructuredMusicMetadataUsesOwnedIDs(t *testing.T) {
	for _, test := range []struct{ provider, url, id string }{
		{"deezer", "https://www.deezer.com/track/123", "123"},
		{"jiosaavn", "https://www.jiosaavn.com/song/song/shareTOKEN", "shareTOKEN"},
		{"musicbrainz", "https://musicbrainz.org/recording/" + songID, songID},
	} {
		t.Run(test.provider, func(t *testing.T) {
			page := `<script type="application/ld+json">{"@graph":[{"@type":"MusicRecording","url":"` + test.url + `","name":"Song & Artist","byArtist":[{"name":"Singer"}],"duration":"PT3M19.25S","datePublished":"2020-01-02","inLanguage":"hi","extra_field":"keep"}]}</script>`
			body, err := DecodeHTML([]byte(page), Request{test.provider, "search", ""})
			if err != nil || !strings.Contains(string(body), "extra_field") {
				t.Fatalf("structured extras lost: %s %v", body, err)
			}
			body, err = DecodeHTML([]byte(page), Request{test.provider, "track", test.id})
			if err != nil {
				t.Fatal(err)
			}
			if test.provider == "musicbrainz" && !strings.Contains(string(body), `"length":199250`) {
				t.Fatal("fractional duration lost")
			}
			if _, err = DecodeHTML([]byte(page), Request{test.provider, "track", "999"}); err == nil {
				t.Fatal("another recording used as direct match")
			}
			wrong := strings.ReplaceAll(page, test.url, "https://attacker.test/track/123")
			if _, err = DecodeHTML([]byte(wrong), Request{test.provider, "search", ""}); err == nil {
				t.Fatal("foreign provider ID accepted")
			}
		})
	}
}
func TestMusicBrainzVisibleRowsStayScoped(t *testing.T) {
	page := `<table><tr><td><a href="/recording/` + songID + `">Song</a></td><td><a href="/artist/` + artistID + `">Singer</a></td><td><a href="/recording/33345678-1234-1234-1234-123456789abc">Related song</a></td></tr></table>`
	body, err := DecodeHTML([]byte(page), Request{Provider: "musicbrainz", Mode: "search"})
	if err != nil || strings.Contains(string(body), "33345678") {
		t.Fatalf("related entity selected: %s %v", body, err)
	}
	if _, err = DecodeHTML([]byte(page), Request{Provider: "musicbrainz", Mode: "track", ID: songID}); err == nil {
		t.Fatal("list row was used as exact detail")
	}
}
func TestErrorPagesAmbiguityAndSizeLimits(t *testing.T) {
	for _, page := range []string{
		`<html><title>Site Unavailable</title><h1>Site Unavailable</h1></html>`,
		`<script type="application/json">{"data":[]}</script>`,
		`<script>fetch('https://attacker.test');</script>`,
		`<meta property="og:title" content="Site Unavailable - Singer">`,
		`<script type="application/json">{"recommendations":{"data":[{"id":123,"title":"Song","artist":{"name":"Singer"}}]}}</script>`,
		`<script type="application/json">{"data":[{"id":123,"title":"Song","artist":{"name":"Singer"}}]}</script><script type="application/json">{"data":[{"id":456,"title":"Other","artist":{"name":"Singer"}}]}</script>`,
		strings.Repeat("x", maxBody+1),
	} {
		if _, err := DecodeHTML([]byte(page), Request{Provider: "deezer", Mode: "search"}); err == nil {
			t.Fatal("invalid HTML reported usable")
		}
	}
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(`{"error":{"message":"SECRET"}}`))}
	if _, err := Read(resp, Request{Provider: "deezer", Mode: "search"}); err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("API error not sanitized: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestPublicFallbackCooldownAndHostBoundary(t *testing.T) {
	f := NewFallback("www.deezer.com")
	calls := 0
	f.SetTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`<html><h1>Site Unavailable</h1></html>`))}, nil
	}))
	req := Request{Provider: "deezer", Mode: "search"}
	for range 2 {
		if _, err := f.Try(context.Background(), "https://www.deezer.com/search/song/track", req); err == nil {
			t.Fatal("empty page recovered")
		}
	}
	if calls != 1 {
		t.Fatal("failed public path retried without cooldown")
	}
	if _, err := f.Try(context.Background(), "https://attacker.test/search", req); err == nil {
		t.Fatal("caller-selected host accepted")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	f.retryAt = time.Time{}
	f.SetWait(func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() })
	if _, err := f.Try(ctx, "https://www.deezer.com/search/song/track", req); err == nil || calls != 1 {
		t.Fatal("rate/cancellation boundary bypassed")
	}
}

func TestPreformattedValuesAndSchemaVariantsPreserveData(t *testing.T) {
	body, err := DecodeHTML([]byte(`<body><pre>{&quot;data&quot;:[{&quot;id&quot;:123,&quot;title&quot;:&quot;Song  With Spaces&quot;,&quot;artist&quot;:{&quot;name&quot;:&quot;Singer&quot;}}]}</pre></body>`), Request{Provider: "deezer", Mode: "search"})
	if err != nil || !strings.Contains(string(body), "Song  With Spaces") {
		t.Fatalf("preformatted value changed: %s %v", body, err)
	}
	page := `<script type="application/ld+json">{"@type":["MusicRecording"],"url":"https://www.deezer.com/track/123","name":"Song","byArtist":"Singer","image":{"@type":"ImageObject","url":"https://images.example/cover.jpg"}}</script>`
	body, err = DecodeHTML([]byte(page), Request{Provider: "deezer", Mode: "search"})
	if err != nil || !strings.Contains(string(body), "cover.jpg") {
		t.Fatalf("structured variants: %s %v", body, err)
	}
	native := `{"error":false,"data":[]}`
	out, err := Read(&http.Response{Body: io.NopCloser(strings.NewReader(native))}, Request{Provider: "deezer", Mode: "search"})
	if err != nil || string(out) != native {
		t.Fatal("false API error flag treated as failure")
	}
}
