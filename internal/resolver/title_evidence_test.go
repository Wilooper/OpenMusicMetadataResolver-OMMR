package resolver

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/ratelimit"
)

type titleFixture struct{ candidate canonical.TrackCandidate }

func (p *titleFixture) Name() string    { return p.candidate.Provider }
func (p *titleFixture) Version() string { return "captured-fixture" }
func (p *titleFixture) Search(context.Context, adapters.Query) ([]canonical.TrackCandidate, error) {
	return []canonical.TrackCandidate{p.candidate}, nil
}
func (p *titleFixture) FetchByID(context.Context, string, string) (*canonical.TrackCandidate, error) {
	return &p.candidate, nil
}

func TestSameArtistAlbumDifferentTitleIsRejected(t *testing.T) {
	data, err := os.ReadFile("testdata/same-album-wrong-title-captured.json")
	if err != nil {
		t.Fatal(err)
	}
	var candidate canonical.TrackCandidate
	if err := json.Unmarshal(data, &candidate); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		q    adapters.Query
		want bool
	}{
		{"wrong song same album", adapters.Query{Title: "FOLK MAFIA", Artist: "Gur Sidhu", Album: "Known Unknowns"}, false},
		{"requested actual song", adapters.Query{Title: "ON MY MIND", Artist: "Gur Sidhu", Album: "Known Unknowns"}, true},
		{"exact ID remains authoritative", adapters.Query{AppleID: "6815521808", Title: "FOLK MAFIA", Artist: "Gur Sidhu"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registry := adapters.NewRegistry()
			registry.Register(&titleFixture{candidate: candidate})
			res := New(registry, nil, ratelimit.NewProviderLimiter(), time.Hour)
			result, err := res.Resolve(context.Background(), ResolveRequest{Query: tc.q})
			if err != nil {
				t.Fatal(err)
			}
			if (result.Track != nil) != tc.want {
				t.Fatalf("accepted=%v want=%v", result.Track != nil, tc.want)
			}
			if !tc.want && result.ProviderStatus[0].Matched {
				t.Fatal("rejected song marked matched")
			}
		})
	}
}

func TestFeaturedTitleRequiresStructuredArtistEvidence(t *testing.T) {
	score := 0.60
	for _, tc := range []struct {
		name   string
		artist string
		want   bool
	}{
		{"reported featured artist", "Jasmeen Akhtar", true},
		{"different featured artist", "Real Boss", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := canonical.TrackCandidate{Track: canonical.Track{Title: "HYPE", Artists: []canonical.Artist{{Name: "Gur Sidhu"}, {Name: tc.artist, Role: "featured"}}, MatchBreakdown: canonical.MatchBreakdown{Title: &score}}}
			if hasTitleEvidence("HYPE (feat. Jasmeen Akhtar)", candidate) != tc.want {
				t.Fatal("featured identity mismatch")
			}
		})
	}
}
