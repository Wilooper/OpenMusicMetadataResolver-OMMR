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

func TestMergerExtendedFields(t *testing.T) {
	m := New()

	candSpotify := canonical.TrackCandidate{
		Provider:   "spotify",
		MatchScore: 0.98,
		Track: canonical.Track{
			Title:       "Test Song",
			PreviewURL:  "https://p.scdn.co/preview",
			TrackNumber: 3,
			DiscNumber:  1,
			Label:       "Label A",
			Barcode:     "UPC-123",
			Copyrights:  []canonical.Copyright{{Type: "C", Text: "(C) 2024 Label A"}},
			Album: canonical.Album{
				Title: "Album A", TotalTracks: 10,
				Label: "Label A", UPC: "UPC-123",
				Copyrights: []canonical.Copyright{{Type: "P", Text: "(P) 2024 Label A"}},
			},
			IDs: map[string]string{"spotify": "s1"},
		},
	}

	candDeezer := canonical.TrackCandidate{
		Provider:   "deezer",
		MatchScore: 0.90,
		Track: canonical.Track{
			Title:     "Test Song",
			ISWC:      "T-123",
			ISRC:      "USABC123",
			PlayCount: 99999,
			IDs:       map[string]string{"deezer": "d1"},
		},
	}

	merged := m.MergeCandidates([]canonical.TrackCandidate{candSpotify, candDeezer})
	if merged == nil {
		t.Fatalf("expected non-nil merged track")
	}
	if merged.PreviewURL != "https://p.scdn.co/preview" {
		t.Errorf("expected preview url from spotify, got %q", merged.PreviewURL)
	}
	if merged.TrackNumber != 3 || merged.DiscNumber != 1 {
		t.Errorf("expected track/disc 3/1, got %d/%d", merged.TrackNumber, merged.DiscNumber)
	}
	if merged.Label != "Label A" {
		t.Errorf("expected label 'Label A', got %q", merged.Label)
	}
	if merged.ISWC != "T-123" {
		t.Errorf("expected ISWC 'T-123', got %q", merged.ISWC)
	}
	if merged.PlayCount != 99999 {
		t.Errorf("expected play count 99999, got %d", merged.PlayCount)
	}
	if merged.Album.TotalTracks != 10 || merged.Album.Label != "Label A" || merged.Album.UPC != "UPC-123" {
		t.Errorf("unexpected merged album enrichment: %+v", merged.Album)
	}
	if len(merged.Copyrights) == 0 {
		t.Errorf("expected merged copyrights")
	}
	if merged.Barcode != "UPC-123" {
		t.Errorf("expected barcode 'UPC-123', got %q", merged.Barcode)
	}
}
