package resolver

import (
	"context"
	"testing"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/cache/disk"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/ratelimit"
)

type mockProvider struct {
	name string
}

type youtubeSeedProvider struct{}

func (youtubeSeedProvider) Name() string    { return "ytmusic" }
func (youtubeSeedProvider) Version() string { return "test-v1" }
func (youtubeSeedProvider) FetchByID(_ context.Context, _, id string) (*canonical.TrackCandidate, error) {
	return &canonical.TrackCandidate{
		Provider: "ytmusic",
		Track: canonical.Track{
			Title:      "Right Song",
			Artists:    []canonical.Artist{{Name: "Right Artist"}},
			DurationMS: 210000,
			IDs:        map[string]string{"ytmusic": id},
		},
	}, nil
}
func (youtubeSeedProvider) Search(context.Context, adapters.Query) ([]canonical.TrackCandidate, error) {
	return nil, nil
}

type misleadingCrossProvider struct {
	name string
}

type duplicateResultsProvider struct{}

func (duplicateResultsProvider) Name() string    { return "spotify" }
func (duplicateResultsProvider) Version() string { return "test-v1" }
func (duplicateResultsProvider) FetchByID(context.Context, string, string) (*canonical.TrackCandidate, error) {
	return nil, nil
}
func (duplicateResultsProvider) Search(_ context.Context, _ adapters.Query) ([]canonical.TrackCandidate, error) {
	return []canonical.TrackCandidate{
		{Provider: "spotify", Track: canonical.Track{Title: "Same Song", Artists: []canonical.Artist{{Name: "Same Artist"}}, IDs: map[string]string{"spotify": "best-first"}}},
		{Provider: "spotify", Track: canonical.Track{Title: "Same Song", Artists: []canonical.Artist{{Name: "Same Artist"}}, IDs: map[string]string{"spotify": "duplicate-second"}}},
	}, nil
}

func (m misleadingCrossProvider) Name() string    { return m.name }
func (m misleadingCrossProvider) Version() string { return "test-v1" }
func (m misleadingCrossProvider) FetchByID(context.Context, string, string) (*canonical.TrackCandidate, error) {
	return nil, nil
}
func (m misleadingCrossProvider) Search(_ context.Context, q adapters.Query) ([]canonical.TrackCandidate, error) {
	if q.Title == "" {
		return nil, nil
	}
	return []canonical.TrackCandidate{{
		Provider: m.name,
		Track: canonical.Track{
			Title:   "Wrong Song",
			Artists: []canonical.Artist{{Name: "Wrong Artist"}},
			IDs:     map[string]string{m.name: "wrong-" + m.name + "-id"},
		},
	}}, nil
}

func (m *mockProvider) Name() string    { return m.name }
func (m *mockProvider) Version() string { return m.name + "-v1" }
func (m *mockProvider) FetchByID(ctx context.Context, idType string, id string) (*canonical.TrackCandidate, error) {
	return &canonical.TrackCandidate{
		Provider: m.name,
		Track: canonical.Track{
			Title:   "Test Song",
			Artists: []canonical.Artist{{Name: "Test Artist"}},
			IDs:     map[string]string{m.name: id},
		},
		MatchScore: 1.0,
	}, nil
}

func (m *mockProvider) Search(ctx context.Context, query adapters.Query) ([]canonical.TrackCandidate, error) {
	return []canonical.TrackCandidate{
		{
			Provider: m.name,
			Track: canonical.Track{
				Title:   query.Title,
				Artists: []canonical.Artist{{Name: query.Artist}},
				IDs:     map[string]string{m.name: "123"},
			},
			MatchScore: 0.9,
		},
	}, nil
}

func TestResolver(t *testing.T) {
	dir := t.TempDir()
	c, err := disk.NewDiskCache(dir)
	if err != nil {
		t.Fatalf("failed to create disk cache: %v", err)
	}

	reg := adapters.NewRegistry()
	reg.Register(&mockProvider{name: "deezer"})

	limiter := ratelimit.NewProviderLimiter()
	res := New(reg, c, limiter, 1*time.Hour)

	ctx := context.Background()
	req := ResolveRequest{
		Query: adapters.Query{
			Title:  "Test Song",
			Artist: "Test Artist",
		},
	}

	resp, err := res.Resolve(ctx, req)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if resp.Track == nil || resp.Track.Title != "test song" {
		t.Errorf("expected resolved track title 'test song', got %v", resp.Track)
	}

	if len(resp.MetadataSources) == 0 || resp.MetadataSources[0] != "deezer" {
		t.Errorf("expected metadata_sources ['deezer'], got %v", resp.MetadataSources)
	}

	// Second query should hit cache
	respCached, err := res.Resolve(ctx, req)
	if err != nil || respCached.Track == nil {
		t.Fatalf("expected cached track response")
	}
	if len(respCached.ProviderStatus) > 0 && !respCached.ProviderStatus[0].Cached {
		t.Errorf("expected provider status to be marked cached")
	}
}

func TestDirectYouTubeResolutionRejectsCrossProviderWrongSong(t *testing.T) {
	c, err := disk.NewDiskCache(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create disk cache: %v", err)
	}
	reg := adapters.NewRegistry()
	reg.Register(youtubeSeedProvider{})
	reg.Register(misleadingCrossProvider{name: "spotify"})
	reg.Register(misleadingCrossProvider{name: "applemusic"})
	res := New(reg, c, ratelimit.NewProviderLimiter(), time.Hour)

	resp, err := res.Resolve(context.Background(), ResolveRequest{
		Query: adapters.Query{YouTubeID: "source-video-id"},
	})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if resp.Track == nil {
		t.Fatal("expected the directly identified YouTube track")
	}
	if resp.Track.Title != "Right Song" {
		t.Fatalf("expected metadata anchored to the YouTube result, got title %q", resp.Track.Title)
	}
	if got := resp.Track.IDs["ytmusic"]; got != "source-video-id" {
		t.Errorf("expected original YouTube ID to be preserved, got %q", got)
	}
	if got := resp.Track.IDs["spotify"]; got != "" {
		t.Errorf("unverified Spotify ID leaked into result: %q", got)
	}
	if got := resp.Track.IDs["applemusic"]; got != "" {
		t.Errorf("unverified Apple Music ID leaked into result: %q", got)
	}
	for _, status := range resp.ProviderStatus {
		if status.Name == "spotify" || status.Name == "applemusic" {
			if status.Matched || status.RejectionReason != "identity_mismatch" {
				t.Errorf("expected %s to report identity_mismatch, got matched=%t reason=%q", status.Name, status.Matched, status.RejectionReason)
			}
		}
	}
}

func TestResolverKeepsOnlyBestCandidatePerProvider(t *testing.T) {
	c, err := disk.NewDiskCache(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create disk cache: %v", err)
	}
	reg := adapters.NewRegistry()
	reg.Register(duplicateResultsProvider{})
	res := New(reg, c, ratelimit.NewProviderLimiter(), time.Hour)
	resp, err := res.Resolve(context.Background(), ResolveRequest{
		Query: adapters.Query{Title: "Same Song", Artist: "Same Artist"},
	})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if resp.Track == nil {
		t.Fatal("expected a resolved track")
	}
	if got := resp.Track.IDs["spotify"]; got != "best-first" {
		t.Fatalf("expected the best provider result to supply the ID, got %q", got)
	}
}

func TestHashQueryIncludesNormalizedSources(t *testing.T) {
	q := adapters.Query{Title: "test song", Artist: "test artist"}
	allSources := hashQuery(q, nil)
	deezerOnly := hashQuery(q, []string{"deezer"})
	if allSources == deezerOnly {
		t.Fatal("provider selection must be part of the cache key")
	}
	if deezerOnly != hashQuery(q, []string{" Deezer ", "DEEZER"}) {
		t.Fatal("equivalent provider selections should share a cache key")
	}
	if hashQuery(q, []string{"deezer", "spotify"}) != hashQuery(q, []string{"spotify", "deezer"}) {
		t.Fatal("provider selection order should not affect the cache key")
	}
}

// badMockProvider returns a candidate completely unrelated to the query so
// that its match score falls below the acceptance threshold.
type badMockProvider struct {
	name string
}

func (m *badMockProvider) Name() string    { return m.name }
func (m *badMockProvider) Version() string { return m.name + "-v1" }
func (m *badMockProvider) FetchByID(ctx context.Context, idType string, id string) (*canonical.TrackCandidate, error) {
	return nil, nil
}
func (m *badMockProvider) Search(ctx context.Context, query adapters.Query) ([]canonical.TrackCandidate, error) {
	return []canonical.TrackCandidate{
		{
			Provider: m.name,
			Track: canonical.Track{
				Title:   "Totally Unrelated",
				Artists: []canonical.Artist{{Name: "Someone Else"}},
				IDs:     map[string]string{m.name: "WRONG-ID-123"},
			},
			MatchScore: 0.0,
		},
	}, nil
}

// TestSubThresholdCandidatesDoNotPolluteIDs verifies that below-threshold
// candidates (which previously were all force-merged when nothing else
// matched) no longer pollute the resolved track's IDs.
func TestSubThresholdCandidatesDoNotPolluteIDs(t *testing.T) {
	dir := t.TempDir()
	c, err := disk.NewDiskCache(dir)
	if err != nil {
		t.Fatalf("failed to create disk cache: %v", err)
	}

	reg := adapters.NewRegistry()
	reg.Register(&mockProvider{name: "good"})
	reg.Register(&badMockProvider{name: "bad"})

	limiter := ratelimit.NewProviderLimiter()
	res := New(reg, c, limiter, 1*time.Hour)

	ctx := context.Background()
	resp, err := res.Resolve(ctx, ResolveRequest{
		Query: adapters.Query{Title: "Test Song", Artist: "Test Artist"},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp.Track == nil {
		t.Fatalf("expected a resolved track")
	}

	if resp.Track.IDs["good"] != "123" {
		t.Errorf("expected good provider ID to be merged, got %v", resp.Track.IDs)
	}
	if resp.Track.IDs["bad"] != "" {
		t.Errorf("expected bad provider ID to be excluded, got %v", resp.Track.IDs)
	}

	for _, idm := range resp.Track.IdentityMatches {
		if idm.Provider == "bad" {
			t.Errorf("identity matches must not include rejected provider, got %+v", idm)
		}
	}

	for _, st := range resp.ProviderStatus {
		if st.Name == "bad" && st.RejectionReason != "below_threshold" {
			t.Errorf("expected bad provider rejection reason 'below_threshold', got %q", st.RejectionReason)
		}
	}
}

// wrongArtistMockProvider returns a candidate with an exact title match but a
// completely different artist, so its combined score passes the overall
// threshold while the artist gate must still reject it.
type wrongArtistMockProvider struct {
	name string
}

func (m *wrongArtistMockProvider) Name() string    { return m.name }
func (m *wrongArtistMockProvider) Version() string { return m.name + "-v1" }
func (m *wrongArtistMockProvider) FetchByID(ctx context.Context, idType string, id string) (*canonical.TrackCandidate, error) {
	return nil, nil
}
func (m *wrongArtistMockProvider) Search(ctx context.Context, query adapters.Query) ([]canonical.TrackCandidate, error) {
	return []canonical.TrackCandidate{
		{
			Provider: m.name,
			Track: canonical.Track{
				Title:   query.Title,
				Artists: []canonical.Artist{{Name: "Someone Else Entirely"}},
				IDs:     map[string]string{m.name: "WRONG-ARTIST-ID"},
			},
		},
	}, nil
}

// TestWrongArtistCandidateIsGated verifies that a candidate matching the title
// but on a different artist is rejected by the artist gate even though its
// combined match score clears the overall acceptance threshold.
func TestWrongArtistCandidateIsGated(t *testing.T) {
	dir := t.TempDir()
	c, err := disk.NewDiskCache(dir)
	if err != nil {
		t.Fatalf("failed to create disk cache: %v", err)
	}

	reg := adapters.NewRegistry()
	reg.Register(&wrongArtistMockProvider{name: "wrongartist"})

	limiter := ratelimit.NewProviderLimiter()
	res := New(reg, c, limiter, 1*time.Hour)

	resp, err := res.Resolve(context.Background(), ResolveRequest{
		Query: adapters.Query{Title: "Test Song", Artist: "Test Artist"},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp.Track != nil {
		t.Fatalf("expected no track for wrong-artist candidate, got %+v", resp.Track)
	}
	for _, st := range resp.ProviderStatus {
		if st.Name == "wrongartist" {
			if st.Matched {
				t.Errorf("expected wrong-artist provider not matched")
			}
			if st.RejectionReason != "below_threshold" {
				t.Errorf("expected rejection reason 'below_threshold', got %q", st.RejectionReason)
			}
		}
	}
}
