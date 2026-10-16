package providerhealth

import (
	"context"
	"errors"
	"net/url"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/enrichment/catalog"
	"github.com/ommr/ommr/internal/models/canonical"
)

type adapter struct {
	adapters.ProviderAdapter
	manager *Manager
}

func Wrap(source adapters.ProviderAdapter, m *Manager) adapters.ProviderAdapter {
	wrapped := &adapter{source, m}
	if richer, ok := source.(adapters.MetadataEnricher); ok {
		return &enrichAdapter{wrapped, richer}
	}
	return wrapped
}
func (a *adapter) FetchByID(ctx context.Context, kind, id string) (*canonical.TrackCandidate, error) {
	lease, err := a.manager.Begin(a.Name())
	if err != nil {
		return nil, err
	}
	result, err := a.ProviderAdapter.FetchByID(ctx, kind, id)
	a.manager.Finish(ctx, a.Name(), lease, err)
	return result, sanitize(err)
}
func (a *adapter) Search(ctx context.Context, q adapters.Query) ([]canonical.TrackCandidate, error) {
	lease, err := a.manager.Begin(a.Name())
	if err != nil {
		return nil, err
	}
	result, err := a.ProviderAdapter.Search(ctx, q)
	a.manager.Finish(ctx, a.Name(), lease, err)
	return result, sanitize(err)
}
func (a *adapter) RateLimitKey() string {
	if source, ok := a.ProviderAdapter.(interface{ RateLimitKey() string }); ok {
		return source.RateLimitKey()
	}
	return a.Name()
}

type enrichAdapter struct {
	*adapter
	richer adapters.MetadataEnricher
}

func (a *enrichAdapter) Enrich(ctx context.Context, track canonical.Track) (*canonical.Track, error) {
	lease, err := a.manager.Begin(a.Name())
	if err != nil {
		return nil, err
	}
	result, err := a.richer.Enrich(ctx, track)
	a.manager.Finish(ctx, a.Name(), lease, err)
	return result, sanitize(err)
}

type enrichment struct {
	catalog.Enricher
	manager *Manager
}

func WrapEnricher(source catalog.Enricher, m *Manager) catalog.Enricher {
	return &enrichment{source, m}
}
func (e *enrichment) Enrich(ctx context.Context, track canonical.Track) (*catalog.Result, error) {
	lease, err := e.manager.Begin(e.Name())
	if err != nil {
		return nil, err
	}
	result, err := e.Enricher.Enrich(ctx, track)
	if result == nil && err == nil {
		e.manager.Skip(e.Name(), lease)
	} else {
		e.manager.Finish(ctx, e.Name(), lease, err)
	}
	return result, sanitize(err)
}

func sanitize(err error) error {
	if err == nil {
		return nil
	}
	var requestError *url.Error
	if errors.As(err, &requestError) {
		return errors.New("provider network request failed")
	}
	return err
}
