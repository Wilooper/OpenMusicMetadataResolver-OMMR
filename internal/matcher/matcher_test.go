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

func TestMatcherAlbumComparisonUsesQueryAlbum(t *testing.T) {
	m := New()

	cand := canonical.TrackCandidate{
		Provider: "applemusic",
		Track: canonical.Track{
			Title:   "Get Lucky",
			Artists: []canonical.Artist{{Name: "Daft Punk"}},
			Album: canonical.Album{
				Title: "Random Access Memories",
			},
		},
	}

	// Without album context the match must not be deflated by album similarity.
	qNoAlbum := adapters.Query{Title: "Get Lucky", Artist: "Daft Punk"}
	scoreNoAlbum, bd := m.ScoreCandidate(cand, qNoAlbum)
	if scoreNoAlbum < 0.95 {
		t.Errorf("expected score >= 0.95 without album context, got %f", scoreNoAlbum)
	}
	if bd.Album != nil {
		t.Errorf("expected nil album breakdown without album context, got %v", *bd.Album)
	}

	// Matching album context should reward the candidate.
	qAlbumMatch := adapters.Query{Title: "Get Lucky", Artist: "Daft Punk", Album: "Random Access Memories"}
	scoreMatch, _ := m.ScoreCandidate(cand, qAlbumMatch)
	if scoreMatch < 0.95 {
		t.Errorf("expected score >= 0.95 for matching album, got %f", scoreMatch)
	}

	// Mismatched album context should penalize the candidate.
	qAlbumMismatch := adapters.Query{Title: "Get Lucky", Artist: "Daft Punk", Album: "Some Other Album"}
	scoreMismatch, bdMismatch := m.ScoreCandidate(cand, qAlbumMismatch)
	if bdMismatch.Album == nil || *bdMismatch.Album > 0.5 {
		t.Errorf("expected low album breakdown for mismatch, got %v", bdMismatch.Album)
	}
	if scoreMismatch > scoreMatch {
		t.Errorf("expected mismatched album score (%f) to be below matched album score (%f)", scoreMismatch, scoreMatch)
	}
}

func TestMatcherDirectIDMatch(t *testing.T) {
	m := New()

	cand := canonical.TrackCandidate{
		Provider: "ytmusic",
		Track: canonical.Track{
			Title:   "Anything At All",
			Artists: []canonical.Artist{{Name: "Not Matching"}},
			IDs:     map[string]string{"ytmusic": "dQw4w9WgXcQ"},
		},
	}

	// A direct YouTube ID lookup must score a full 1.0 despite weak
	// title/artist similarity.
	q := adapters.Query{YouTubeID: "dQw4w9WgXcQ"}
	score, _ := m.ScoreCandidate(cand, q)
	if score != 1.0 {
		t.Errorf("expected score 1.0 for direct ID match, got %f", score)
	}
}
