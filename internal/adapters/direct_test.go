package adapters

import (
	"github.com/ommr/ommr/internal/models/canonical"
	"testing"
)

func TestJioSaavnNativeAndShareOwnership(t *testing.T) {
	c := canonical.TrackCandidate{Provider: "jiosaavn", Track: canonical.Track{IDs: map[string]string{"jiosaavn": "ERIkRAwJf38", "jiosaavn_track_id": "azOux9HL"}}}
	for _, q := range []Query{{JioSaavnID: "azOux9HL"}, {JioSaavnID: "ERIkRAwJf38"}, {JioSaavnURL: "https://www.jiosaavn.com/song/mera-mann/ERIkRAwJf38"}} {
		if !DirectIDMatch(q, c) {
			t.Fatalf("lost exact JioSaavn identity: %+v", q)
		}
	}
	if DirectIDMatch(Query{JioSaavnURL: "https://www.jiosaavn.com/song/other/azOux9HL"}, c) {
		t.Fatal("share token matched native-id alias")
	}
	c.Track.IDs = map[string]string{"jiosaavn": "azOux9HL", "jiosaavn_track_id": "otherID8"}
	if DirectIDMatch(Query{JioSaavnID: "azOux9HL"}, c) {
		t.Fatal("contradictory native ID accepted")
	}
	c.Provider = "other"
	if DirectIDMatch(Query{JioSaavnID: "azOux9HL"}, c) {
		t.Fatal("foreign provider supplied JioSaavn seed")
	}
}
