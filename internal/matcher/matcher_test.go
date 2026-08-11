package matcher

import (
	"testing"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/canonical"
)

func TestMatcherScoring(t *testing.T) {
	m := New()

	cand := canonical.TrackCandidate{
		Provider: "deezer",
		Track: canonical.Track{
			Title: "Get Lucky",
			Artists: []canonical.Artist{
				{Name: "Daft Punk"},
			},
			ISRC: "USGBR1300001",
		},
	}

	// Exact ISRC match
	qISRC := adapters.Query{ISRC: "USGBR1300001"}
	score, _ := m.ScoreCandidate(cand, qISRC)
	if score != 1.0 {
		t.Errorf("expected score 1.0 for ISRC match, got %f", score)
	}

	// Title + Artist fuzzy match
	qFuzzy := adapters.Query{Title: "Get Lucky (Official Video)", Artist: "Daft Punk"}
	scoreFuzzy, bd := m.ScoreCandidate(cand, qFuzzy)
	if scoreFuzzy < 0.90 {
		t.Errorf("expected score >= 0.90 for fuzzy title match, got %f", scoreFuzzy)
	}
	if bd.Title == nil || *bd.Title < 0.90 {
		t.Errorf("expected title breakdown score >= 0.90, got %v", bd.Title)
	}
}
