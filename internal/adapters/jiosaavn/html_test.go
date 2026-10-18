package jiosaavn

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
