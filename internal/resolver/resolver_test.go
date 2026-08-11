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

func (m *mockProvider) Name() string    { return m.name }
func (m *mockProvider) Version() string { return m.name + "-v1" }
func (m *mockProvider) FetchByID(ctx context.Context, idType string, id string) (*canonical.TrackCandidate, error) {
	return &canonical.TrackCandidate{
		Provider: m.name,
		Track: canonical.Track{
			Title:  "Test Song",
			Artists: []canonical.Artist{{Name: "Test Artist"}},
			IDs:    map[string]string{m.name: id},
		},
		MatchScore: 1.0,
	}, nil
}

func (m *mockProvider) Search(ctx context.Context, query adapters.Query) ([]canonical.TrackCandidate, error) {
	return []canonical.TrackCandidate{
		{
			Provider: m.name,
			Track: canonical.Track{
				Title:  query.Title,
				Artists: []canonical.Artist{{Name: query.Artist}},
				IDs:    map[string]string{m.name: "123"},
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
