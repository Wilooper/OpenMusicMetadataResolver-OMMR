package merger

import (
	"testing"

	"github.com/ommr/ommr/internal/models/canonical"
)

func placementCandidate(provider, album string, disc, track int) canonical.TrackCandidate {
	return canonical.TrackCandidate{Provider: provider, MatchScore: .98, Track: canonical.Track{
		Title: "Song", IDs: map[string]string{provider: provider + "-track"},
		Album:      canonical.Album{Title: album, IDs: map[string]string{provider: provider + "-album"}},
		DiscNumber: disc, TrackNumber: track,
	}}
}

func TestAlbumPlacementCannotMixDifferentEditions(t *testing.T) {
	a := placementCandidate("applemusic", "Album", 2, 4)
	s := placementCandidate("spotify", "Compilation", 1, 19)
	d := placementCandidate("deezer", "Album", 1, 7)
	m := NewWithPolicy(Policy{Mode: "consensus", Priorities: map[string][]string{
		"album": {"applemusic"}, "disc_number": {"deezer"}, "track_number": {"spotify"},
	}})
	for _, candidates := range [][]canonical.TrackCandidate{{s, a, d}, {d, a, s}} {
		got := m.MergeCandidates(candidates)
		if got.Album.Title != "Album" || got.TrackNumber != 4 || got.DiscNumber != 2 || got.FieldSources["disc_number"][0] != "applemusic" {
			t.Fatalf("mixed placement: %+v", got)
		}
	}
	a.Track.DiscNumber = 0
	got := m.MergeCandidates([]canonical.TrackCandidate{a, s, d})
	if got.DiscNumber != 0 || got.TrackNumber != 4 {
		t.Fatal("unknown disc borrowed from other edition")
	}
}

func TestDirectProviderAnchorsAlbumAndPosition(t *testing.T) {
	candidates := []canonical.TrackCandidate{placementCandidate("spotify", "Original", 1, 3), placementCandidate("deezer", "Original", 1, 3), placementCandidate("applemusic", "Deluxe", 2, 4)}
	m := New()
	for _, provider := range []string{"applemusic", "spotify", ""} {
		got := m.MergeCandidatesAnchored(candidates, provider)
		if provider == "applemusic" {
			if got.Album.Title != "Deluxe" || got.DiscNumber != 2 || got.TrackNumber != 4 {
				t.Fatalf("direct edition lost: %+v", got)
			}
		} else if got.Album.Title != "Original" || got.TrackNumber != 3 {
			t.Fatalf("anchor mutated shared policy: %+v", got)
		}
	}
}

func TestSelectedEditionDoesNotBorrowCatalogTotals(t *testing.T) {
	a := placementCandidate("applemusic", "Album", 2, 4)
	s := placementCandidate("spotify", "Album", 1, 7)
	s.Track.Album.TotalTracks = 30
	s.Track.Album.UPC = "other-edition"
	got := NewWithPolicy(Policy{Mode: "priority", Priorities: map[string][]string{"album": {"applemusic"}}}).MergeCandidates([]canonical.TrackCandidate{a, s})
	if got.Album.TotalTracks != 0 || got.Album.UPC != "" {
		t.Fatal("same title conflated edition catalog data")
	}
}

func TestIndependentPositionPolicyIsRejected(t *testing.T) {
	for _, field := range []string{"track_number", "disc_number"} {
		if _, err := ParsePolicy(`{"priorities":{"` + field + `":["spotify"]}}`); err == nil {
			t.Fatal("position override can detach selected edition")
		}
	}
}
