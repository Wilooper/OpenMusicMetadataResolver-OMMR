package catalog

import (
	"context"
	"encoding/json"
	"github.com/ommr/ommr/internal/cache/disk"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ommr/ommr/internal/models/canonical"
)

const releaseID = "76df3287-6cda-33eb-8e9a-044b5e15ffdd"
const recordingID = "12345678-1234-1234-1234-123456789abc"

func fixtureTrack() canonical.Track {
	return canonical.Track{Title: "Song", Artists: []canonical.Artist{{Name: "Singer"}}, DurationMS: 180000, Album: canonical.Album{Title: "Album", IDs: map[string]string{"musicbrainz": releaseID}}, IDs: map[string]string{"musicbrainz": recordingID}}
}
func TestCoverArtPreservesImageMetadataAndChecksReleaseIdentity(t *testing.T) {
	mismatch := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := releaseID
		if mismatch {
			id = recordingID
		}
		json.NewEncoder(w).Encode(map[string]any{"release": "https://musicbrainz.org/release/" + id, "images": []any{map[string]any{"image": "https://images.example/front.jpg", "front": true, "approved": true, "edit": 42, "types": []string{"Front"}, "thumbnails": map[string]string{"500": "https://images.example/500.jpg"}}}})
	}))
	defer srv.Close()
	source := NewCoverArt(NewClient(nil))
	source.base = srv.URL
	result, err := source.Enrich(context.Background(), fixtureTrack())
	if err != nil || len(result.Images) != 2 {
		t.Fatalf("cover art failed: %+v %v", result, err)
	}
	data, _ := json.Marshal(result.Data)
	if !strings.Contains(string(data), `"edit":42`) {
		t.Fatal("extra artwork fields discarded")
	}
	mismatch = true
	if _, err = source.Enrich(context.Background(), fixtureTrack()); err == nil {
		t.Fatal("wrong release accepted")
	}
}
func TestLastFMRequiresExactIdentityAndPreservesTagsWikiAndStatistics(t *testing.T) {
	wrong := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") != "test-key" || r.URL.Query().Get("autocorrect") != "0" {
			t.Error("incorrect Last.fm request")
		}
		artist := "Singer"
		if wrong {
			artist = "Other"
		}
		json.NewEncoder(w).Encode(map[string]any{"track": map[string]any{"name": "Song", "artist": map[string]string{"name": artist}, "mbid": recordingID, "listeners": "123", "toptags": map[string]any{"tag": []string{"pop"}}, "wiki": map[string]string{"summary": "Context"}}})
	}))
	defer srv.Close()
	source := NewLastFM(NewClient(nil), "test-key")
	source.base = srv.URL
	result, err := source.Enrich(context.Background(), fixtureTrack())
	if err != nil || result == nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(result.Data)
	if !strings.Contains(string(data), "listeners") || !strings.Contains(string(data), "Context") {
		t.Fatal("rich Last.fm fields lost")
	}
	wrong = true
	if _, err = source.Enrich(context.Background(), fixtureTrack()); err == nil {
		t.Fatal("wrong artist accepted")
	}
}
func TestDiscogsPreservesMatchingEditionWithoutCreatingRecordingIDs(t *testing.T) {
	wrong := false
	calls := 0
	client := NewClient(func(context.Context, string) error { calls++; return nil })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Discogs token=test-token" {
			t.Error("missing Discogs header")
		}
		if strings.Contains(r.URL.Path, "search") {
			json.NewEncoder(w).Encode(map[string]any{"results": []any{map[string]int{"id": 123}}})
			return
		}
		title := "Song"
		if wrong {
			title = "Other"
		}
		json.NewEncoder(w).Encode(map[string]any{"id": 123, "title": "Album", "artists": []any{map[string]string{"name": "Singer"}}, "styles": []string{"Synth-pop"}, "companies": []string{"Company"}, "tracklist": []any{map[string]string{"title": title, "duration": "3:00", "position": "A1"}}})
	}))
	defer srv.Close()
	source := NewDiscogs(client, "test-token")
	source.base = srv.URL
	result, err := source.Enrich(context.Background(), fixtureTrack())
	if err != nil || result == nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(result.Data)
	if !strings.Contains(string(data), "Synth-pop") || !strings.Contains(string(data), "release_track_context") {
		t.Fatal("edition context lost")
	}
	if calls != 2 {
		t.Fatal("each upstream request must be rate limited")
	}
	wrong = true
	result, err = source.Enrich(context.Background(), fixtureTrack())
	if err != nil || result != nil {
		t.Fatal("unmatched release track accepted")
	}
}
func TestMusicBrainzReleaseMustContainAcceptedRecording(t *testing.T) {
	wrong := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := recordingID
		if wrong {
			id = "other"
		}
		json.NewEncoder(w).Encode(map[string]any{"id": releaseID, "title": "Album", "barcode": "123", "media": []any{map[string]any{"tracks": []any{map[string]any{"recording": map[string]string{"id": id}}}}}})
	}))
	defer srv.Close()
	source := NewMusicBrainzRelease(NewClient(nil))
	source.base = srv.URL
	if result, err := source.Enrich(context.Background(), fixtureTrack()); err != nil || result == nil {
		t.Fatal(err)
	}
	wrong = true
	if _, err := source.Enrich(context.Background(), fixtureTrack()); err == nil {
		t.Fatal("unrelated edition accepted")
	}
}
func TestAcoustIDUsesPostAndReturnsAllCandidates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Error("fingerprint must use POST")
		}
		_ = r.ParseForm()
		if r.Form.Get("client") != "test-key" || r.Form.Get("fingerprint") != "fp" || r.Form.Get("duration") != "180" {
			t.Error("wrong fingerprint parameters")
		}
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "results": []any{map[string]any{"id": "a", "score": 0.9}, map[string]any{"id": "b", "score": 0.8}}})
	}))
	defer srv.Close()
	source := NewAcoustID(NewClient(nil), "test-key")
	source.base = srv.URL
	result, err := source.Lookup(context.Background(), "fp", 180)
	if err != nil || len(result["results"].([]any)) != 2 {
		t.Fatal("ambiguous candidates were hidden")
	}
	if _, err = source.Lookup(context.Background(), "fp", 0); err == nil {
		t.Fatal("invalid duration accepted")
	}
}
func TestClientNeverLeaksCredentialURLsInErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://elsewhere.invalid/?api_key=private", 302)
	}))
	defer srv.Close()
	var out any
	err := NewClient(nil).Request(context.Background(), "lastfm", "GET", srv.URL+"?api_key=private", "", nil, &out)
	if err == nil || strings.Contains(err.Error(), "private") {
		t.Fatal("credential URL leaked or redirect followed")
	}
}

type fixtureTransport func(*http.Request) (*http.Response, error)

func (f fixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestArtworkRedirectFollowsOnlyArchiveHTTPS(t *testing.T) {
	calls := 0
	client := NewClient(nil)
	client.HTTP.Transport = fixtureTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return &http.Response{StatusCode: 307, Header: http.Header{"Location": []string{"https://archive.org/download/mbid-" + releaseID + "/index.json"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
		}
		if r.URL.Host != "archive.org" || r.Header.Get("Authorization") != "" {
			t.Error("invalid artwork redirect")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"release":"https://musicbrainz.org/release/` + releaseID + `","images":[]}`)), Request: r}, nil
	})
	source := NewCoverArt(client)
	if result, err := source.Enrich(context.Background(), fixtureTrack()); err != nil || result == nil || calls != 2 {
		t.Fatalf("artwork redirect failed %v", err)
	}
}
func TestCatalogCacheReusesJSONAndDoesNotStoreErrors(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("error") == "1" {
			io.WriteString(w, `{"error":10}`)
			return
		}
		io.WriteString(w, `{"id":123,"extra":"retained"}`)
	}))
	defer srv.Close()
	cache, err := disk.NewDiskCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer cache.Close()
	waits := 0
	client := NewClient(func(context.Context, string) error { waits++; return nil })
	client.Cache = cache
	for range 2 {
		var out map[string]any
		if err := client.Request(context.Background(), "discogs", "GET", srv.URL, "token", nil, &out); err != nil || out["extra"] != "retained" {
			t.Fatal(err)
		}
	}
	if calls != 1 || waits != 1 {
		t.Fatal("cached request repeated upstream work")
	}
	for range 2 {
		var out map[string]any
		if err := client.Request(context.Background(), "discogs", "GET", srv.URL+"?error=1", "token", nil, &out); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 3 {
		t.Fatal("application-level error was cached")
	}
	var out any
	if err := client.Request(context.Background(), "discogs", "GET", srv.URL, "other-token", nil, &out); err != nil {
		t.Fatal(err)
	}
	if calls != 4 {
		t.Fatal("credential scopes share raw cache")
	}
}
