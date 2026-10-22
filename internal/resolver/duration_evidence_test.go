package resolver

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/ratelimit"
)

func timed(provider string, duration int64) canonical.TrackCandidate {
	return canonical.TrackCandidate{Provider: provider, Track: canonical.Track{Title: "Goldberg Variations, BWV 988: Aria", Artists: []canonical.Artist{{Name: "Glenn Gould"}}, DurationMS: duration, IDs: map[string]string{provider: provider + "-recording"}}}
}
func TestMaterialDurationOutlierRequiresIndependentMajority(t *testing.T) {
	rejected := durationOutliers([]canonical.TrackCandidate{timed("applemusic", 112939), timed("bugs", 112000), timed("genie", 113000), timed("ytmusic", 185000), timed("melon", 0)})
	if !rejected["ytmusic"] || len(rejected) != 1 {
		t.Fatalf("outlier not isolated: %+v", rejected)
	}
	for _, rows := range [][]canonical.TrackCandidate{
		{timed("a", 110000), timed("a", 110000), timed("b", 185000)},
		{timed("a", 110000), timed("b", 110000), timed("c", 185000), timed("d", 185000)},
		{timed("a", 110000), timed("b", 110000), timed("c", 115000)},
	} {
		if len(durationOutliers(rows)) != 0 {
			t.Fatal("duplicates, ambiguity or ordinary timing differences became identity evidence")
		}
	}
}

type durationFixture struct {
	candidate        canonical.TrackCandidate
	enrichedDuration int64
}

func (p *durationFixture) Name() string  { return p.candidate.Provider }
func (*durationFixture) Version() string { return "constructed-duration-fixture" }
func (p *durationFixture) Search(context.Context, adapters.Query) ([]canonical.TrackCandidate, error) {
	return []canonical.TrackCandidate{p.candidate}, nil
}
func (p *durationFixture) FetchByID(context.Context, string, string) (*canonical.TrackCandidate, error) {
	c := p.candidate
	return &c, nil
}
func TestRecordingOutlierCannotLeakIDsOrGraph(t *testing.T) {
	registry := adapters.NewRegistry()
	for _, c := range []canonical.TrackCandidate{timed("applemusic", 112939), timed("bugs", 112000), timed("genie", 113000), timed("ytmusic", 185000)} {
		registry.Register(&durationFixture{candidate: c})
	}
	res := New(registry, nil, ratelimit.NewProviderLimiter(), time.Hour)
	r, e := res.Resolve(context.Background(), ResolveRequest{Query: adapters.Query{Title: "Goldberg Variations, BWV 988: Aria", Artist: "Glenn Gould"}})
	if e != nil || r.Track == nil {
		t.Fatal(e)
	}
	if r.Track.IDs["ytmusic"] != "" {
		t.Fatal("different performance ID leaked")
	}
	for _, m := range r.Track.IdentityMatches {
		if m.Provider == "ytmusic" {
			t.Fatal("rejected performance emitted identity match")
		}
	}
	for _, s := range r.ProviderStatus {
		if s.Name == "ytmusic" && (s.Matched || s.RejectionReason != "identity_mismatch") {
			t.Fatal("outlier status misreported")
		}
	}
	// A supplied direct ID continues to anchor its recording; text-majority filtering cannot override it.
	r, e = res.Resolve(context.Background(), ResolveRequest{Query: adapters.Query{YouTubeID: "ytmusic-recording"}, Sources: []string{"applemusic", "bugs", "genie", "ytmusic"}})
	if e != nil || r.Track == nil || r.Track.IDs["ytmusic"] != "ytmusic-recording" {
		t.Fatal("direct identity weakened")
	}
	if len(r.Track.IDs) != 1 {
		t.Fatalf("cross-source duration contradicts direct seed: %+v", r.Track.IDs)
	}
}

func (p *durationFixture) EnrichCandidate(ctx context.Context, c canonical.TrackCandidate) (*canonical.TrackCandidate, error) {
	if p.enrichedDuration <= 0 {
		return nil, nil
	}
	c.Track.DurationMS = p.enrichedDuration
	return &c, nil
}
func TestHydratedTimingRejectsDifferentClassicalPerformance(t *testing.T) {
	registry := adapters.NewRegistry()
	for _, p := range []*durationFixture{
		{candidate: timed("applemusic", 112939)},
		{candidate: timed("bugs", 0), enrichedDuration: 112000},
		{candidate: timed("genie", 0), enrichedDuration: 113000},
		{candidate: timed("ytmusic", 185000)},
	} {
		registry.Register(p)
	}
	res := New(registry, nil, ratelimit.NewProviderLimiter(), time.Hour)
	r, e := res.Resolve(context.Background(), ResolveRequest{Query: adapters.Query{Title: "Goldberg Variations, BWV 988: Aria", Artist: "Glenn Gould"}})
	if e != nil || r.Track == nil {
		t.Fatal(e)
	}
	if r.Track.IDs["ytmusic"] != "" {
		t.Fatal("post-hydration duration outlier leaked")
	}
	if strings.Contains(string(r.Track.Extensions["music_graph"]), "ytmusic-recording") {
		t.Fatal("rejected performance entered graph")
	}
	for _, m := range r.Track.IdentityMatches {
		if m.Provider == "ytmusic" {
			t.Fatal("post-hydration rejection retained identity match")
		}
	}
	for _, s := range r.ProviderStatus {
		if s.Name == "ytmusic" && s.Matched {
			t.Fatal("rejected performance marked matched")
		}
	}
}

func TestExplicitISRCIsNotOverriddenByTimingConsensus(t *testing.T) {
	registry := adapters.NewRegistry()
	for _, c := range []canonical.TrackCandidate{timed("bugs", 112000), timed("genie", 113000), timed("ytmusic", 185000)} {
		c.Track.ISRC = "USABC2300001"
		registry.Register(&durationFixture{candidate: c})
	}
	res := New(registry, nil, ratelimit.NewProviderLimiter(), time.Hour)
	r, e := res.Resolve(context.Background(), ResolveRequest{Query: adapters.Query{ISRC: "USABC2300001"}})
	if e != nil || r.Track == nil || r.Track.IDs["ytmusic"] == "" {
		t.Fatal("duration filtering overrode exact requested ISRC")
	}
}
