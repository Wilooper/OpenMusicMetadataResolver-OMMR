package providerhealth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/canonical"
)

type noDetailProvider struct{ calls int }

func (*noDetailProvider) Name() string    { return "test" }
func (*noDetailProvider) Version() string { return "test" }
func (*noDetailProvider) Search(context.Context, adapters.Query) ([]canonical.TrackCandidate, error) {
	return nil, nil
}
func (*noDetailProvider) FetchByID(context.Context, string, string) (*canonical.TrackCandidate, error) {
	return nil, nil
}
func (p *noDetailProvider) EnrichCandidate(context.Context, canonical.TrackCandidate) (*canonical.TrackCandidate, error) {
	p.calls++
	return nil, nil
}

func TestNoOpDetailDoesNotResetFailureEvidence(t *testing.T) {
	manager := New(3, time.Minute)
	manager.Set("test", "active", "public", "absent", "")
	lease, _ := manager.Begin("test")
	manager.Finish(context.Background(), "test", lease, errors.New("HTTP 500"))
	source := &noDetailProvider{}
	wrapped := Wrap(source, manager)
	detail, ok := wrapped.(adapters.CandidateEnricher)
	if !ok {
		t.Fatal("detail capability hidden")
	}
	if _, err := detail.EnrichCandidate(context.Background(), canonical.TrackCandidate{}); err != nil {
		t.Fatal(err)
	}
	if manager.Snapshot()[0].ConsecutiveFailures != 1 || source.calls != 1 {
		t.Fatal("no-op detail reset health")
	}
}
