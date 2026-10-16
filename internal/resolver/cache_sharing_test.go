package resolver

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/cache/disk"
	"github.com/ommr/ommr/internal/merger"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/ratelimit"
)

type blockingProvider struct {
	calls   atomic.Int64
	started chan struct{}
	release chan struct{}
	empty   bool
}

func (*blockingProvider) Name() string    { return "deezer" }
func (*blockingProvider) Version() string { return "fixture-v1" }
func (p *blockingProvider) FetchByID(context.Context, string, string) (*canonical.TrackCandidate, error) {
	return nil, nil
}
func (p *blockingProvider) Search(ctx context.Context, _ adapters.Query) ([]canonical.TrackCandidate, error) {
	if p.calls.Add(1) == 1 && p.started != nil {
		close(p.started)
	}
	if p.release != nil {
		select {
		case <-p.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if p.empty {
		return nil, nil
	}
	return []canonical.TrackCandidate{{Provider: "deezer", Track: canonical.Track{Title: "Song", Artists: []canonical.Artist{{Name: "Singer"}}, IDs: map[string]string{"deezer": "123"}}}}, nil
}
func sharedResolver(t *testing.T, p *blockingProvider) *Resolver {
	t.Helper()
	reg := adapters.NewRegistry()
	reg.Register(p)
	cache, err := disk.NewDiskCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cache.Close() })
	return New(reg, cache, ratelimit.NewProviderLimiter(), time.Hour)
}
func TestConcurrentMissesUseOneProviderCallAndIndependentResponses(t *testing.T) {
	p := &blockingProvider{started: make(chan struct{}), release: make(chan struct{})}
	res := sharedResolver(t, p)
	req := ResolveRequest{Query: adapters.Query{Artist: "Singer", Title: "Song"}}
	results := make([]*ResolveResponse, 20)
	errors := make([]error, 20)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func() { defer wg.Done(); results[i], errors[i] = res.Resolve(context.Background(), req) }()
	}
	<-p.started
	close(p.release)
	wg.Wait()
	for _, err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	if p.calls.Load() != 1 {
		t.Fatalf("duplicate provider calls: %d", p.calls.Load())
	}
	results[0].Track.IDs["deezer"] = "changed"
	for _, result := range results[1:] {
		if result.Track.IDs["deezer"] != "123" {
			t.Fatal("waiters share mutable response maps")
		}
	}
}
func TestCancelingFirstWaiterDoesNotCancelSharedWork(t *testing.T) {
	p := &blockingProvider{started: make(chan struct{}), release: make(chan struct{})}
	res := sharedResolver(t, p)
	req := ResolveRequest{Query: adapters.Query{Artist: "Singer", Title: "Song"}}
	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan error, 1)
	go func() { _, err := res.Resolve(ctx, req); first <- err }()
	<-p.started
	cancel()
	if err := <-first; err != context.Canceled {
		t.Fatal(err)
	}
	close(p.release)
	got, err := res.Resolve(context.Background(), req)
	if err != nil || got.Track == nil || p.calls.Load() != 1 {
		t.Fatalf("shared work canceled: %+v %v %d", got, err, p.calls.Load())
	}
}
func TestHealthyNoResultsAreCached(t *testing.T) {
	p := &blockingProvider{empty: true}
	res := sharedResolver(t, p)
	req := ResolveRequest{Query: adapters.Query{Artist: "Singer", Title: "Missing"}}
	for range 2 {
		response, err := res.Resolve(context.Background(), req)
		if err != nil || response.Track != nil {
			t.Fatal("wrong empty response")
		}
	}
	if p.calls.Load() != 1 {
		t.Fatal("negative result not cached")
	}
}
func TestCacheNamespaceIncludesMergePolicy(t *testing.T) {
	p := &blockingProvider{}
	res := sharedResolver(t, p)
	req := ResolveRequest{Query: adapters.Query{Artist: "Singer", Title: "Song"}}
	if _, err := res.Resolve(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	res.Configure(merger.Policy{Mode: "priority"})
	if _, err := res.Resolve(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if p.calls.Load() != 2 {
		t.Fatal("changed merge policy reused old cache")
	}
}
