package merger

import (
	"testing"

	"github.com/ommr/ommr/internal/models/canonical"
)

func TestMergerFieldPriorities(t *testing.T) {
	m := New()

	candSpotify := canonical.TrackCandidate{
		Provider:   "spotify",
		MatchScore: 0.98,
		Track: canonical.Track{
			Title:  "Get Lucky (Spotify Title)",
			Genres: []string{"Disco"},
			IDs:    map[string]string{"spotify": "spot123"},
			Images: []canonical.Image{
				{URL: "spot_high", Width: 640, Height: 640},
				{URL: "spot_low", Width: 100, Height: 100},
			},
		},
	}

	candMB := canonical.TrackCandidate{
		Provider:   "musicbrainz",
		MatchScore: 0.95,
		Track: canonical.Track{
			Title: "Get Lucky (MB Title)",
			Credits: []canonical.Credit{
				{Name: "Guy-Manuel", Roles: []string{"Composer", "Producer"}},
			},
			IDs:    map[string]string{"musicbrainz": "mb123"},
			Images: []canonical.Image{{URL: "mb_mid", Width: 300, Height: 300}},
		},
	}

	candidates := []canonical.TrackCandidate{candSpotify, candMB}
	merged := m.MergeCandidates(candidates)

	if merged == nil {
		t.Fatalf("expected non-nil merged track")
	}

	// Title priority
	if merged.Title != "Get Lucky (Spotify Title)" {
		t.Errorf("expected Spotify title, got %q", merged.Title)
	}

	// FieldSources map[string][]string
	if len(merged.FieldSources["title"]) == 0 || merged.FieldSources["title"][0] != "spotify" {
		t.Errorf("expected field_sources['title'] = ['spotify'], got %v", merged.FieldSources["title"])
	}

	// Credits multi-role
	if len(merged.Credits) == 0 || len(merged.Credits[0].Roles) != 2 {
		t.Errorf("expected 2 roles for credit, got %v", merged.Credits)
	}

	// Images ascending resolution sort (lowest to highest)
	if len(merged.Images) != 3 {
		t.Fatalf("expected 3 images, got %d", len(merged.Images))
	}
	if merged.Images[0].URL != "spot_low" || merged.Images[2].URL != "spot_high" {
		t.Errorf("expected images sorted ascending by resolution area, got %v", merged.Images)
	}

	// CanonicalID and Completeness
	if merged.CanonicalID == "" {
		t.Errorf("expected populated canonical_id")
	}
	if merged.Completeness <= 0.0 {
		t.Errorf("expected positive completeness score, got %f", merged.Completeness)
	}
}
