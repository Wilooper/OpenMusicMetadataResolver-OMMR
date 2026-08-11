package adapters

import (
	"fmt"
	"strings"
	"sync"
)

// Registry manages thread-safe registration and retrieval of ProviderAdapters.
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]ProviderAdapter
}

// NewRegistry initializes an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		adapters: make(map[string]ProviderAdapter),
	}
}

// Register registers a ProviderAdapter implementation.
func (r *Registry) Register(adapter ProviderAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[strings.ToLower(adapter.Name())] = adapter
}

// Get retrieves a registered ProviderAdapter by name.
func (r *Registry) Get(name string) (ProviderAdapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.adapters[strings.ToLower(name)]
	return a, ok
}

// List returns a slice of all registered ProviderAdapters.
func (r *Registry) List() []ProviderAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]ProviderAdapter, 0, len(r.adapters))
	for _, a := range r.adapters {
		list = append(list, a)
	}
	return list
}

// ListActive filters registered adapters by requested provider names (comma-separated or slice).
func (r *Registry) ListActive(names []string) ([]ProviderAdapter, error) {
	if len(names) == 0 {
		return r.List(), nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	active := make([]ProviderAdapter, 0, len(names))
	for _, name := range names {
		cleanName := strings.TrimSpace(strings.ToLower(name))
		if cleanName == "" {
			continue
		}
		adapter, ok := r.adapters[cleanName]
		if !ok {
			return nil, fmt.Errorf("provider %q is not registered", name)
		}
		active = append(active, adapter)
	}
	return active, nil
}
