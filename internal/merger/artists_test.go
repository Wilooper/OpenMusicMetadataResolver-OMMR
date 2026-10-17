package merger

import (
	"encoding/json"
	"testing"

	"github.com/ommr/ommr/internal/models/canonical"
)

func artistFixture() []canonical.TrackCandidate {
	return []canonical.TrackCandidate{
		{Provider: "applemusic", Track: canonical.Track{Title: "Boyfriend", Artists: []canonical.Artist{{Name: "Karan Aujla & Ikky", IDs: map[string]string{"applemusic": "karan-only"}}}}},
		{Provider: "ytmusic", Track: canonical.Track{Title: "Boyfriend", Artists: []canonical.Artist{{Name: "Karan Aujla", IDs: map[string]string{"ytmusic": "karan"}}, {Name: "Ikky", IDs: map[string]string{"ytmusic": "ikky"}}}}},
		{Provider: "spotify", Track: canonical.Track{Title: "Boyfriend", Artists: []canonical.Artist{{Name: "Karan Aujla", IDs: map[string]string{"spotify": "karan-spotify"}}, {Name: "Ikky", IDs: map[string]string{"spotify": "ikky-spotify"}}}}},
	}
}
func TestEvidenceBasedArtistStructureAndOwnedIDs(t *testing.T) {
	candidates := artistFixture()
	before, _ := json.Marshal(candidates)
	got := New().MergeCandidates(candidates)
	if len(got.Artists) != 2 || got.Artists[0].Name != "Karan Aujla" || got.Artists[1].Name != "Ikky" {
		t.Fatalf("artists: %+v", got.Artists)
	}
	for _, a := range got.Artists {
		if a.IDs["applemusic"] != "" {
			t.Fatal("combined-display ID assigned to an individual")
		}
		if a.IDs["spotify"] == "" || a.IDs["ytmusic"] == "" {
			t.Fatal("verified individual IDs lost")
		}
	}
	if len(got.FieldSources["artists"]) != 3 {
		t.Fatalf("equivalent display lost vote: %v", got.FieldSources)
	}
	after, _ := json.Marshal(candidates)
	if string(before) != string(after) {
		t.Fatal("mutated source metadata")
	}
	candidates[0], candidates[2] = candidates[2], candidates[0]
	swapped, _ := json.Marshal(New().MergeCandidates(candidates))
	original, _ := json.Marshal(got)
	if string(original) != string(swapped) {
		t.Fatal("arrival order changes artists")
	}
}
func TestArtistNamesAreNotSplitWithoutSeparateEvidence(t *testing.T) {
	candidates := artistFixture()[:1]
	candidates[0].Track.Artists[0].Name = "Earth, Wind & Fire"
	if got := New().MergeCandidates(candidates); len(got.Artists) != 1 {
		t.Fatal("band split from punctuation")
	}
}
func TestArtistTrustOverridePreservesChosenRepresentation(t *testing.T) {
	got := NewWithPolicy(Policy{Mode: "consensus", Priorities: map[string][]string{"artists": {"applemusic"}}}).MergeCandidates(artistFixture())
	if len(got.Artists) != 1 || got.Artists[0].Name != "Karan Aujla & Ikky" {
		t.Fatalf("priority ignored: %+v", got.Artists)
	}
}
