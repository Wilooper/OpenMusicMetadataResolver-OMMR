package resolver

import (
	"context"
	"testing"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/ratelimit"
)

func TestArtistEvidenceRejectsLookalikesAndPreservesCollaborations(t *testing.T) {
	for _, tc := range []struct {
		expected string
		names    []string
		want     bool
	}{
		{"Daft Punk", []string{"Funk Punk"}, false},
		{"Karan Aujla", []string{"Karan Aujla & Ikky"}, true},
		{"Karan Aujla & Ikky", []string{"Karan Aujla", "Ikky"}, true},
		{"Pharrell Williams", []string{"Daft Punk", "Pharrell Williams", "Nile Rodgers"}, true},
		{"The Beatles", []string{"Beatles"}, true},
		{"Singer", nil, false},
	} {
		artists := []canonical.Artist{}
		for _, name := range tc.names {
			artists = append(artists, canonical.Artist{Name: name})
		}
		if got := hasArtistEvidence(tc.expected, artists); got != tc.want {
			t.Errorf("%s vs %v: %v", tc.expected, tc.names, got)
		}
	}
}
func TestSameTitleDifferentArtistCannotLeakRecordingID(t *testing.T) {
	original := canonical.TrackCandidate{Provider: "deezer", Track: canonical.Track{Title: "Get Lucky", Artists: []canonical.Artist{{Name: "Funk Punk"}}, DurationMS: 250000, IDs: map[string]string{"deezer": "wrong"}}}
	fixture := &detailFixture{original: original, rich: original}
	registry := adapters.NewRegistry()
	registry.Register(fixture)
	resolver := New(registry, nil, ratelimit.NewProviderLimiter(), time.Hour)
	result, err := resolver.Resolve(context.Background(), ResolveRequest{Query: adapters.Query{Title: "Get Lucky", Artist: "Daft Punk"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Track != nil || fixture.calls != 0 || result.ProviderStatus[0].Matched {
		t.Fatalf("wrong-artist result escaped gate: %+v", result)
	}
}
