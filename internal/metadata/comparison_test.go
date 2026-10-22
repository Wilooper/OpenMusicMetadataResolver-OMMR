package metadata

import (
	"encoding/json"
	"testing"

	"github.com/ommr/ommr/internal/models/canonical"
)

func TestComparisonExposesDifferencesWithoutInventingUnknownVotes(t *testing.T) {
	candidates := []canonical.TrackCandidate{
		{Provider: "applemusic", Track: canonical.Track{DurationMS: 160796, Explicit: false, IDs: map[string]string{"applemusic": "1"}}, RawResponse: []byte(`{"trackId":1,"trackExplicitness":"notExplicit"}`)},
		{Provider: "ytmusic", Track: canonical.Track{DurationMS: 161000, Explicit: false}},
	}
	track := canonical.Track{DurationMS: 160796, FieldSources: map[string][]string{"duration_ms": {"applemusic"}}}
	Compare(&track, candidates)
	var result map[string]struct {
		Selected  any            `json:"selected"`
		Values    map[string]any `json:"values"`
		Different bool           `json:"different"`
	}
	if json.Unmarshal(track.Extensions["metadata_comparison"], &result) != nil {
		t.Fatal("comparison absent")
	}
	if !result["duration_ms"].Different || len(result["duration_ms"].Values) != 2 || result["duration_ms"].Selected != float64(160796) {
		t.Fatalf("duration comparison: %+v", result)
	}
	if _, ok := result["explicit"]; ok {
		t.Fatal("unknown false created another vote")
	}
}
