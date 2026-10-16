package adapters

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Registry manages thread-safe registration and retrieval of ProviderAdapters.
type Registry struct {
	mu           sync.RWMutex
	adapters     map[string]ProviderAdapter
	availability func(string) bool
	known        map[string]bool
}

// NewRegistry initializes an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		adapters: make(map[string]ProviderAdapter),
		known:    map[string]bool{},
	}
}

// Register registers a ProviderAdapter implementation.
func (r *Registry) Register(adapter ProviderAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[strings.ToLower(adapter.Name())] = adapter
	r.known[strings.ToLower(adapter.Name())] = true
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
		if r.availability == nil || r.availability(a.Name()) {
			list = append(list, a)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name() < list[j].Name() })
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
	seen := map[string]bool{}
	for _, name := range names {
		cleanName := strings.TrimSpace(strings.ToLower(name))
		if cleanName == "" || seen[cleanName] {
			continue
		}
		adapter, ok := r.adapters[cleanName]
		if !ok {
			if r.known[cleanName] {
				continue
			}
			return nil, fmt.Errorf("provider %q is not registered", name)
		}
		if r.availability == nil || r.availability(adapter.Name()) {
			active = append(active, adapter)
		}
		seen[cleanName] = true
	}
	sort.Slice(active, func(i, j int) bool { return active[i].Name() < active[j].Name() })
	return active, nil
}

func (r *Registry) SetAvailability(check func(string) bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.availability = check
}

func (r *Registry) MarkKnown(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.known[strings.ToLower(name)] = true
}
