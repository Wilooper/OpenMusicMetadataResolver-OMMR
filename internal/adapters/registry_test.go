package adapters

import (
	"context"
	"testing"

	"github.com/ommr/ommr/internal/models/canonical"
)

type mockAdapter struct {
	name string
}

func (m *mockAdapter) Name() string    { return m.name }
func (m *mockAdapter) Version() string { return m.name + "-v1" }
func (m *mockAdapter) FetchByID(ctx context.Context, idType string, id string) (*canonical.TrackCandidate, error) {
	return nil, nil
}
func (m *mockAdapter) Search(ctx context.Context, query Query) ([]canonical.TrackCandidate, error) {
	return nil, nil
}

func TestRegistry(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockAdapter{name: "deezer"})
	reg.Register(&mockAdapter{name: "musicbrainz"})

	a, found := reg.Get("deezer")
	if !found || a.Name() != "deezer" {
		t.Errorf("expected deezer adapter, got found=%v", found)
	}

	active, err := reg.ListActive([]string{"deezer", "musicbrainz"})
	if err != nil || len(active) != 2 {
		t.Errorf("expected 2 active adapters, got len=%d, err=%v", len(active), err)
	}

	_, err = reg.ListActive([]string{"unknown_provider"})
	if err == nil {
		t.Errorf("expected error for unregistered provider, got nil")
	}
}
