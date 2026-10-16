package bootstrap

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/providerhealth"
)

type fixtureSource struct {
	name  string
	calls *atomic.Int64
}

func (f fixtureSource) Name() string    { return f.name }
func (f fixtureSource) Version() string { return "fixture" }
func (f fixtureSource) FetchByID(context.Context, string, string) (*canonical.TrackCandidate, error) {
	f.calls.Add(1)
	return nil, nil
}
func (f fixtureSource) Search(context.Context, adapters.Query) ([]canonical.TrackCandidate, error) {
	f.calls.Add(1)
	return nil, nil
}
func TestStartupValidationFallbackAndRequiredCredentialGates(t *testing.T) {
	var probes atomic.Int64
	calls := &atomic.Int64{}
	source := func(name string) adapters.ProviderAdapter { return fixtureSource{name, calls} }
	reg := adapters.NewRegistry()
	health := providerhealth.New(2, time.Minute)
	plans := []Plan{
		{Name: "spotify", Enabled: true, Configured: true, Public: source("spotify"), Candidates: []Candidate{{source("spotify"), "session", func(context.Context) error { probes.Add(1); return errors.New("invalid") }}}},
		{Name: "tidal", Enabled: true, Configured: false},
		{Name: "qobuz", Enabled: true, Configured: true, Candidates: []Candidate{{source("qobuz"), "keyed", func(context.Context) error { probes.Add(1); return nil }}}},
		{Name: "disabled", Enabled: false, Configured: true, Candidates: []Candidate{{source("disabled"), "keyed", func(context.Context) error { t.Error("disabled source was probed"); return nil }}}},
	}
	Activate(context.Background(), plans, 2, health, reg)
	if probes.Load() != 2 {
		t.Fatal("configured credentials were not probed")
	}
	if len(reg.List()) != 2 {
		t.Fatal("unavailable source was loaded")
	}
	if _, ok := reg.Get("tidal"); ok {
		t.Fatal("missing required credentials loaded")
	}
	statuses := health.Snapshot()
	for _, status := range statuses {
		if status.Name == "spotify" && (status.Mode != "public" || status.Credentials != "rejected") {
			t.Fatal("invalid session did not fall back")
		}
	}
	if active, err := reg.ListActive([]string{"tidal"}); err != nil || len(active) != 0 {
		t.Fatal("disabled known source should be skipped")
	}
	if calls.Load() != 0 {
		t.Fatal("source metadata fetched outside validation")
	}
}
