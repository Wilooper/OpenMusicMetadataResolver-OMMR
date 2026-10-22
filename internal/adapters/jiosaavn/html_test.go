package jiosaavn

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/metadata"
)

func TestHTMLStructuredSongKeepsShareIdentityAndExtras(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<script type="application/ld+json">{"@type":"MusicRecording","url":"https://www.jiosaavn.com/song/song/shareTOKEN","name":"Song","byArtist":{"name":"Singer"},"identifier":"NOT_A_NATIVE_ID","duration":"PT3M19S","genre":"Pop","unusual_extra":"retain"}</script>`))
	}))
	defer server.Close()
	old := SearchURL
	SearchURL = server.URL
	defer func() { SearchURL = old }()
	p := New()
	p.web = nil
	results, err := p.Search(context.Background(), adapters.Query{Artist: "Singer", Title: "Song"})
	if err != nil || len(results) != 1 {
		t.Fatalf("HTML source: %+v %v", results, err)
	}
	result := results[0]
	if result.Track.IDs["jiosaavn"] != "shareTOKEN" || result.Track.IDs["jiosaavn_track_id"] != "" {
		t.Fatalf("fabricated native ID: %+v", result.Track.IDs)
	}
	record := metadata.Record(result)
	raw, _ := json.Marshal(record)
	if record["catalog"] == nil || !json.Valid(raw) {
		t.Fatal("structured extras lost")
	}
}

const v4Song = `{"id":"native42","type":"song","title":"Rock &amp; Roll","year":"2024","language":"english","play_count":1200,"explicit_content":true,"perma_url":"https://www.jiosaavn.com/song/rock-roll/share42","more_info":{"album":"Album &amp; More","album_id":123,"duration":"199","label":"Label","copyright_text":"2024 Label","artistMap":{"primary_artists":[{"id":456,"name":"Artist, With Comma"}],"featured_artists":[{"id":"789","name":"Guest"}],"artists":[{"id":"321","name":"Writer","role":"lyricist"}]},"has_lyrics":"true","encrypted_media_url":"must-not-leak"}}`

func TestV4CatalogJSONAndEmbeddedState(t *testing.T) {
	for _, embedded := range []bool{false, true} {
		t.Run(map[bool]string{false: "json", true: "embedded"}[embedded], func(t *testing.T) {
			payload := `{"total":"1","start":0,"results":[` + v4Song + `]}`
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("api_version") != "4" || r.URL.Query().Get("p") != "0" {
					t.Error("missing v4 search parameters")
				}
				if embedded {
					payload = `<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":` + payload + `}}</script>`
				}
				_, _ = w.Write([]byte(payload))
			}))
			defer server.Close()
			old := SearchURL
			SearchURL = server.URL
			defer func() { SearchURL = old }()
			p := New()
			p.web = nil
			results, err := p.Search(context.Background(), adapters.Query{Artist: "Artist, With Comma", Title: "Rock & Roll"})
			if err != nil || len(results) != 1 {
				t.Fatalf("v4 response: %v %+v", err, results)
			}
			track := results[0].Track
			if track.Title != "Rock & Roll" || track.Album.Title != "Album & More" || track.DurationMS != 199000 || !track.Explicit || track.PlayCount != 1200 {
				t.Fatalf("nested catalog lost: %+v", track)
			}
			if len(track.Artists) != 2 || track.Artists[0].Name != "Artist, With Comma" || track.Artists[0].IDs[ProviderName] != "456" || track.Artists[1].Role != "featured" {
				t.Fatalf("structured artist identity lost: %+v", track.Artists)
			}
			if track.Album.IDs[ProviderName] != "123" || len(track.Credits) != 1 || track.Credits[0].Name != "Writer" || len(track.Copyrights) != 1 {
				t.Fatalf("credits lost: %+v", track)
			}
			raw, _ := json.Marshal(metadata.Record(results[0]))
			if !strings.Contains(string(raw), "has_lyrics") || strings.Contains(string(raw), "must-not-leak") {
				t.Fatalf("extras/sanitization failure: %s", raw)
			}
		})
	}
}

func TestDetailRecordExtrasAndWrongIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"native42":` + v4Song + `}`)) }))
	defer server.Close()
	old := SearchURL
	SearchURL = server.URL
	defer func() { SearchURL = old }()
	p := New()
	candidate, err := p.FetchByID(context.Background(), "jiosaavn", "native42")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(metadata.Record(*candidate))
	if !strings.Contains(string(raw), "has_lyrics") {
		t.Fatalf("detail extras lost: %s", raw)
	}
	if _, err := p.FetchByID(context.Background(), "jiosaavn", "different42"); err == nil {
		t.Fatal("accepted wrong direct ID")
	}
}
