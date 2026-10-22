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

type detailFixture struct {
	original canonical.TrackCandidate
	rich     canonical.TrackCandidate
	fail     bool
	calls    int
}

func (p *detailFixture) Name() string    { return "deezer" }
func (p *detailFixture) Version() string { return "fixture" }
func (p *detailFixture) FetchByID(context.Context, string, string) (*canonical.TrackCandidate, error) {
	return nil, nil
}
func (p *detailFixture) Search(context.Context, adapters.Query) ([]canonical.TrackCandidate, error) {
	return []canonical.TrackCandidate{p.original}, nil
}
func (p *detailFixture) EnrichCandidate(context.Context, canonical.TrackCandidate) (*canonical.TrackCandidate, error) {
	p.calls++
	if p.fail {
		return nil, fmt.Errorf("unavailable")
	}
	return &p.rich, nil
}

func TestDetailHydrationAddsFactsWithoutReplacingIdentity(t *testing.T) {
	for _, mode := range []string{"ok", "wrong_id", "wrong_title", "wrong_artist", "wrong_isrc", "wrong_duration", "wrong_album", "unavailable", "unknown_explicit"} {
		t.Run(mode, func(t *testing.T) {
			original := canonical.TrackCandidate{Provider: "deezer", Track: canonical.Track{Title: "Song", Artists: []canonical.Artist{{Name: "Singer"}}, ISRC: "USAAA2500001", DurationMS: 160000, PreviewURL: "https://preview.example", IDs: map[string]string{"deezer": "123"}, Album: canonical.Album{Title: "Album", IDs: map[string]string{"deezer": "album"}}}, RawResponse: []byte(`{"data":[{"id":123,"search_only_field":"retain","explicit_lyrics":false}]}`)}
			encoded, _ := json.Marshal(original)
			var rich canonical.TrackCandidate
			_ = json.Unmarshal(encoded, &rich)
			rich.Track.Label = "Known label"
			rich.Track.Credits = []canonical.Credit{{Name: "Composer", Roles: []string{"Composer"}}}
			rich.Track.PreviewURL = ""
			rich.Track.IDs["spotify"] = "forged-id"
			rich.RawResponse = []byte(`{"id":123,"detail_only_field":"retain","explicit_lyrics":false}`)
			fixture := &detailFixture{original: original, rich: rich}
			switch mode {
			case "wrong_id":
				fixture.rich.Track.IDs["deezer"] = "other"
			case "wrong_title":
				fixture.rich.Track.Title = "Other Song"
			case "wrong_artist":
				fixture.rich.Track.Artists[0].Name = "Other Singer"
			case "wrong_isrc":
				fixture.rich.Track.ISRC = "OTHER"
			case "wrong_duration":
				fixture.rich.Track.DurationMS = 200000
			case "wrong_album":
				fixture.rich.Track.Album.IDs["deezer"] = "other-album"
			case "unavailable":
				fixture.fail = true
			case "unknown_explicit":
				fixture.original.Track.Explicit = true
				fixture.rich.RawResponse = []byte(`{"id":123}`)
			}
			reg := adapters.NewRegistry()
			reg.Register(fixture)
			res := New(reg, nil, ratelimit.NewProviderLimiter(), time.Hour)
			got, err := res.Resolve(context.Background(), ResolveRequest{Query: adapters.Query{Title: "Song", Artist: "Singer"}})
			if err != nil || got.Track == nil {
				t.Fatalf("resolve: %v", err)
			}
			accepted := mode == "ok" || mode == "unknown_explicit"
			if (got.Track.Label == "Known label") != accepted {
				t.Fatalf("metadata accepted incorrectly: %+v", got.Track)
			}
			if got.Track.IDs["deezer"] != "123" || got.Track.IDs["spotify"] != "" || got.Track.Title != "Song" {
				t.Fatalf("identity replaced: %+v", got.Track)
			}
			if got.Track.PreviewURL != "https://preview.example" {
				t.Fatal("search-only normalized field lost")
			}
			if mode == "ok" {
				var extra map[string]any
				_ = json.Unmarshal(got.Track.Extensions["provider_data"], &extra)
				record := extra["deezer"].(map[string]any)["catalog"].(map[string]any)
				if record["search_only_field"] != "retain" || record["detail_only_field"] != "retain" {
					t.Fatal("extra fields lost during hydration")
				}
			}
			if mode == "unknown_explicit" && !got.Track.Explicit {
				t.Fatal("unknown detail flag erased known explicit evidence")
			}
			if !accepted && got.ProviderStatus[0].EnrichmentStatus == "ok" {
				t.Fatal("failed hydration marked successful")
			}
		})
	}
}

type hydrationTTLCache struct{ ttl time.Duration }

func (*hydrationTTLCache) Get(context.Context, string) ([]byte, bool, error) { return nil, false, nil }
func (c *hydrationTTLCache) Set(_ context.Context, _ string, _ []byte, ttl time.Duration) error {
	c.ttl = ttl
	return nil
}
func (*hydrationTTLCache) Delete(context.Context, string) error { return nil }
func (*hydrationTTLCache) Close() error                         { return nil }

func TestFailedDetailDoesNotFreezeSparseResultForFullCacheTTL(t *testing.T) {
	candidate := canonical.TrackCandidate{Provider: "deezer", Track: canonical.Track{Title: "Song", Artists: []canonical.Artist{{Name: "Singer"}}, IDs: map[string]string{"deezer": "123"}}}
	provider := &detailFixture{original: candidate, fail: true}
	registry := adapters.NewRegistry()
	registry.Register(provider)
	cache := &hydrationTTLCache{}
	resolver := New(registry, cache, ratelimit.NewProviderLimiter(), time.Hour)
	result, err := resolver.Resolve(context.Background(), ResolveRequest{Query: adapters.Query{Title: "Song", Artist: "Singer"}})
	if err != nil || result.Track == nil || cache.ttl != 30*time.Second {
		t.Fatalf("sparse cache TTL: %v %v", cache.ttl, err)
	}
}

func TestPartialDetailMetadataIsKeptAndRetriedSoon(t *testing.T) {
	c := canonical.TrackCandidate{Provider: "deezer", Track: canonical.Track{Title: "Song", Artists: []canonical.Artist{{Name: "Singer"}}, IDs: map[string]string{"deezer": "123"}}}
	rich := c
	rich.Track.Extensions = map[string]json.RawMessage{"_enrichment_partial": json.RawMessage(`true`), "extra_metadata": json.RawMessage(`{"biography":"Context"}`)}
	source := &detailFixture{original: c, rich: rich}
	registry := adapters.NewRegistry()
	registry.Register(source)
	backend := &hydrationTTLCache{}
	resolver := New(registry, backend, ratelimit.NewProviderLimiter(), time.Hour)
	result, err := resolver.Resolve(context.Background(), ResolveRequest{Query: adapters.Query{Artist: "Singer", Title: "Song"}})
	if err != nil || result.Track == nil || result.ProviderStatus[0].EnrichmentStatus != "partial" || backend.ttl != 30*time.Second {
		t.Fatalf("partial result: %+v %v %v", result, err, backend.ttl)
	}
}
