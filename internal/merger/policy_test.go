package merger

import (
	"encoding/json"
	"testing"

	"github.com/ommr/ommr/internal/models/canonical"
)

func TestConsensusDeterminismAndExplicitPriority(t *testing.T) {
	makeCandidate := func(name, title string, duration int64) canonical.TrackCandidate {
		return canonical.TrackCandidate{Provider: name, MatchScore: 0.95, Track: canonical.Track{Title: title, DurationMS: duration, Album: canonical.Album{Title: "Album", IDs: map[string]string{name: name + "-album"}}}}
	}
	candidates := []canonical.TrackCandidate{makeCandidate("spotify", "Minority", 90000), makeCandidate("deezer", "Song", 100000), makeCandidate("applemusic", "Song", 100000)}
	original, _ := json.Marshal(New().MergeCandidates(candidates))
	candidates[0], candidates[2] = candidates[2], candidates[0]
	reversed := New().MergeCandidates(candidates)
	data, _ := json.Marshal(reversed)
	if string(original) != string(data) {
		t.Fatal("merge depends on provider arrival order")
	}
	if reversed.Title != "Song" || reversed.DurationMS != 100000 || len(reversed.Album.IDs) != 3 {
		t.Fatalf("consensus/album IDs failed: %+v", reversed)
	}
	policy, err := ParsePolicy(`{"mode":"consensus","priorities":{"title":["spotify","deezer"]}}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := NewWithPolicy(policy).MergeCandidates(candidates); got.Title != "Minority" || got.DurationMS != 100000 {
		t.Fatal("field trust override failed")
	}
}
func TestUnknownExplicitDoesNotVoteAgainstKnownEvidence(t *testing.T) {
	candidates := []canonical.TrackCandidate{{Provider: "spotify", Track: canonical.Track{Title: "Song"}}, {Provider: "deezer", Track: canonical.Track{Title: "Song", Explicit: true}}}
	got := New().MergeCandidates(candidates)
	if !got.Explicit || string(got.Extensions["explicit_status"]) != `"explicit"` {
		t.Fatal("unknown false overrode explicit evidence")
	}
}
func TestPolicyRejectsUnsupportedModesAndFields(t *testing.T) {
	for _, raw := range []string{`{"mode":"random"}`, `{"priorities":{"bogus":["spotify"]}}`, `{"priorities":{"title":["spotify","Spotify"]}}`} {
		if _, err := ParsePolicy(raw); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}

func TestConfidenceUsesSelectedISRCInsteadOfRejectedConsensus(t *testing.T) {
	candidates := []canonical.TrackCandidate{
		{Provider: "spotify", MatchScore: 0.95, Track: canonical.Track{Title: "Song", ISRC: "USAAA2500001"}},
		{Provider: "applemusic", MatchScore: 0.95, Track: canonical.Track{Title: "Song", ISRC: "USAAA2500002"}},
		{Provider: "deezer", MatchScore: 0.95, Track: canonical.Track{Title: "Song", ISRC: "USAAA2500002"}},
	}
	merged := NewWithPolicy(Policy{Mode: "consensus", Priorities: map[string][]string{"isrc": {"spotify"}}}).MergeCandidates(candidates)
	if merged.ISRC != "USAAA2500001" || len(merged.FieldSources["isrc"]) != 1 || merged.Confidence.Level == canonical.ConfidenceVerified {
		t.Fatalf("unselected ISRC created verified confidence: %+v", merged)
	}
}
