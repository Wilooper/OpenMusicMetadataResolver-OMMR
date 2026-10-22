package resolver

import (
	"context"
	"testing"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/ratelimit"
)

func TestVersionQualifiers(t *testing.T) {
	for _, tc := range []struct {
		target, candidate string
		want              bool
	}{
		{"Tum Hi Ho", "Tum Hi Ho (Remix)", false},
		{"Song (Acoustic)", "Song", false},
		{"Song - Live", "Song (Live at Wembley)", true},
		{"Song", "Song (Official Audio)", true},
		{"Live Forever", "Live Forever", true},
		{"Song (Remix)", "Song (Acoustic)", false},
	} {
		if got := compatibleVersion(tc.target, tc.candidate); got != tc.want {
			t.Errorf("%q / %q: %v", tc.target, tc.candidate, got)
		}
	}
}

func TestUnrequestedRemixCannotLeakRecordingID(t *testing.T) {
	candidate := canonical.TrackCandidate{Provider: "deezer", Track: canonical.Track{Title: "Tum Hi Ho (Remix)", Artists: []canonical.Artist{{Name: "Arijit Singh"}}, IDs: map[string]string{"deezer": "remix"}}}
	fixture := &detailFixture{original: candidate, rich: candidate}
	registry := adapters.NewRegistry()
	registry.Register(fixture)
	resolver := New(registry, nil, ratelimit.NewProviderLimiter(), time.Hour)
	result, err := resolver.Resolve(context.Background(), ResolveRequest{Query: adapters.Query{Title: "Tum Hi Ho", Artist: "Arijit Singh"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Track != nil || fixture.calls != 0 {
		t.Fatalf("unrequested version accepted: %+v", result.Track)
	}
}

func TestUnrequestedReimaginedCannotLeakRecordingID(t *testing.T) {
	// Captured public metadata: original Genie 81785038 is 316s; Apple's
	// 1750709920 and YouTube X0uOx8nj67M identify a ~380s Reimagined version.
	for _, title := range []string{"Experience (Reimagined)", "Experience - Re-imagined"} {
		candidate := canonical.TrackCandidate{Provider: "deezer", Track: canonical.Track{Title: title, Artists: []canonical.Artist{{Name: "Ludovico Einaudi"}}, DurationMS: 380082, IDs: map[string]string{"deezer": "reimagined"}}}
		fixture := &detailFixture{original: candidate, rich: candidate}
		registry := adapters.NewRegistry()
		registry.Register(fixture)
		resolver := New(registry, nil, ratelimit.NewProviderLimiter(), time.Hour)
		result, err := resolver.Resolve(context.Background(), ResolveRequest{Query: adapters.Query{Title: "Experience", Artist: "Ludovico Einaudi"}})
		if err != nil {
			t.Fatal(err)
		}
		if result.Track != nil || fixture.calls != 0 {
			t.Fatal("unrequested reimagined recording accepted")
		}
	}
	if !compatibleVersion("Experience (Reimagined)", "Experience (Reimagined)") {
		t.Fatal("explicitly requested version rejected")
	}
	if compatibleVersion("Experience (Reimagined)", "Experience") {
		t.Fatal("original accepted for requested reimagined version")
	}
}
