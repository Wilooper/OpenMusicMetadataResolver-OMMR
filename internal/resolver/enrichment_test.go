package resolver

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/ratelimit"
)

type enrichingFixture struct {
	rich  canonical.Track
	calls int
	fail  bool
}

func (p *enrichingFixture) Name() string    { return "musicbrainz" }
func (p *enrichingFixture) Version() string { return "fixture" }
func (p *enrichingFixture) FetchByID(context.Context, string, string) (*canonical.TrackCandidate, error) {
	return nil, nil
}
func (p *enrichingFixture) Search(context.Context, adapters.Query) ([]canonical.TrackCandidate, error) {
	return []canonical.TrackCandidate{{Provider: p.Name(), Track: canonical.Track{Title: "Song", Artists: []canonical.Artist{{Name: "Singer"}}, ISRC: "USABC2300001", IDs: map[string]string{"musicbrainz": "recording"}}}}, nil
}
func (p *enrichingFixture) Enrich(context.Context, canonical.Track) (*canonical.Track, error) {
	p.calls++
	if p.fail {
		return nil, fmt.Errorf("lookup unavailable")
	}
	return &p.rich, nil
}

func TestResolutionEnrichesOnlyAcceptedRecording(t *testing.T) {
	for _, mode := range []string{"ok", "wrong_title", "wrong_id", "wrong_isrc", "unavailable"} {
		t.Run(mode, func(t *testing.T) {
			fixture := &enrichingFixture{rich: canonical.Track{Title: "Song", Artists: []canonical.Artist{{Name: "Singer"}}, ISRC: "USABC2300001", ISWC: "T-123", IDs: map[string]string{"musicbrainz": "recording"}, Credits: []canonical.Credit{{Name: "Poet", Roles: []string{"Lyricist"}}}, Extensions: map[string]json.RawMessage{"musicbrainz_work_id": json.RawMessage(`"work"`)}}}
			status := "identity_mismatch"
			switch mode {
			case "ok":
				status = "ok"
			case "wrong_title":
				fixture.rich.Title = "Other Song"
			case "wrong_id":
				fixture.rich.IDs["musicbrainz"] = "other"
			case "wrong_isrc":
				fixture.rich.ISRC = "OTHER"
			case "unavailable":
				fixture.fail = true
				status = "unavailable"
			}
			registry := adapters.NewRegistry()
			registry.Register(fixture)
			res := New(registry, nil, ratelimit.NewProviderLimiter(), time.Hour)
			response, err := res.Resolve(context.Background(), ResolveRequest{Query: adapters.Query{Artist: "Singer", Title: "Song"}})
			if err != nil || response.Track == nil {
				t.Fatalf("resolve failed: %v", err)
			}
			if fixture.calls != 1 || response.ProviderStatus[0].EnrichmentStatus != status {
				t.Fatalf("enrichment diagnostics: %+v", response.ProviderStatus)
			}
			if response.Track.Title != "Song" || response.Track.IDs["musicbrainz"] != "recording" {
				t.Fatal("enrichment changed accepted identity")
			}
			if (len(response.Track.Credits) > 0) != (mode == "ok") {
				t.Fatalf("unexpected credits: %+v", response.Track.Credits)
			}
		})
	}
}

func TestRejectedCandidateNeverTriggersEnrichment(t *testing.T) {
	fixture := &enrichingFixture{}
	registry := adapters.NewRegistry()
	registry.Register(fixture)
	res := New(registry, nil, ratelimit.NewProviderLimiter(), time.Hour)
	_, err := res.Resolve(context.Background(), ResolveRequest{Query: adapters.Query{Artist: "Unrelated Artist", Title: "Unrelated Title", ISRC: "OTHER"}})
	if err != nil {
		t.Fatal(err)
	}
	if fixture.calls != 0 {
		t.Fatal("rejected recording triggered enrichment")
	}
}
