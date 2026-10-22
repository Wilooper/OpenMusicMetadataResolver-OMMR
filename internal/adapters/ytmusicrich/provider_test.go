package ytmusicrich

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/ommr/ommr/internal/models/canonical"
)

type fakeCaller struct{ wrong bool }

func (f fakeCaller) Call(ctx context.Context, kind, id string, limit int) (json.RawMessage, error) {
	if kind == "album" {
		return nil, errors.New("upstream unavailable")
	}
	if kind == "song" {
		if f.wrong {
			return json.RawMessage(`{"id":"abcdefghijk","song":{"videoDetails":{"videoId":"wrongrecord"}}}`), nil
		}
		return json.RawMessage(`{"id":"abcdefghijk","song":{"videoDetails":{"videoId":"abcdefghijk"}}}`), nil
	}
	raw, _ := json.Marshal(map[string]any{"id": id, "kind": "artist", "metadata": map[string]string{"biography": "Real source context"}})
	return raw, nil
}
func TestPartialContextSurvivesWithoutChangingRecordingIdentity(t *testing.T) {
	original := canonical.TrackCandidate{Provider: "ytmusic", Track: canonical.Track{Title: "Song", IDs: map[string]string{"ytmusic": "abcdefghijk"}, Album: canonical.Album{IDs: map[string]string{"ytmusic": "MPREalbum"}}, Artists: []canonical.Artist{{Name: "Singer", IDs: map[string]string{"ytmusic": "UCartist"}}}}}
	p := Provider{Pool: fakeCaller{}, Profiles: true}
	rich, err := p.EnrichCandidate(context.Background(), original)
	if err != nil {
		t.Fatal(err)
	}
	if rich.Track.IDs["ytmusic"] != "abcdefghijk" || string(rich.Track.Extensions["_enrichment_partial"]) != "true" || len(rich.Track.Extensions["youtube_catalog"]) == 0 {
		t.Fatalf("partial metadata lost: %+v", rich)
	}
	if original.Track.Extensions != nil {
		t.Fatal("input mutated")
	}
	p = Provider{Pool: fakeCaller{wrong: true}}
	if _, err = p.EnrichCandidate(context.Background(), original); err == nil {
		t.Fatal("wrong song attached")
	}
}
func TestSongContextRejectsAdjacentQueueRecords(t *testing.T) {
	raw := json.RawMessage(`{"watch":{"tracks":[{"videoId":"abcdefghijk"},{"videoId":"wrongrecord"}]}}`)
	if sameSong(raw, "abcdefghijk") {
		t.Fatal("adjacent song treated as target metadata")
	}
}
