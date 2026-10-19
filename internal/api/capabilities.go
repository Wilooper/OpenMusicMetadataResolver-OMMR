package api

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/ommr/ommr/internal/artists"
)

type endpointCapability struct {
	Method           string `json:"method"`
	Path             string `json:"path"`
	Enabled          bool   `json:"enabled"`
	RuntimeAvailable bool   `json:"runtime_available"`
}

type providerCapability struct {
	Name                   string   `json:"name"`
	Version                string   `json:"version,omitempty"`
	Enabled                bool     `json:"enabled"`
	TrackAdapterRegistered bool     `json:"track_adapter_registered"`
	TrackRuntimeAvailable  bool     `json:"track_runtime_available"`
	TrackIDField           string   `json:"track_id_field,omitempty"`
	TrackURLInput          bool     `json:"track_url_input"`
	Operations             []string `json:"operations"`
}

// Capability discovery reads runtime state only. It never probes an upstream.
func (h *Handler) HandleCapabilities(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		w.WriteHeader(405)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Use GET"})
		return
	}
	urlProviders := map[string]bool{"spotify": true, "ytmusic": true, "applemusic": true, "deezer": true, "jiosaavn": true, "soundcloud": true, "qobuz": true, "tidal": true, "musicbrainz": true}
	idFields := map[string]string{"spotify": "spotify_id", "ytmusic": "youtube_id", "applemusic": "apple_id", "deezer": "deezer_id", "soundcloud": "soundcloud_id", "jiosaavn": "jiosaavn_id", "musicbrainz": "musicbrainz_id", "qobuz": "qobuz_id", "tidal": "tidal_id", "amazonmusic": "amazonmusic_id", "pandora": "pandora_id", "bugs": "bugs_id", "genie": "genie_id", "melon": "melon_id"}
	names := map[string]bool{}
	for _, name := range []string{"spotify", "ytmusic", "applemusic", "deezer", "jiosaavn", "soundcloud", "musicbrainz", "qobuz", "tidal", "amazonmusic", "pandora", "bugs", "genie", "melon"} {
		names[name] = true
	}
	if h.registry != nil {
		for _, source := range h.registry.List() {
			names[source.Name()] = true
		}
	}
	sorted := []string{}
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)
	profiles := []providerCapability{}
	trackAvailable, artistAvailable := false, false
	youtubeAvailable := h.extractor != nil && h.cfg.Lifecycle.ProviderEnabled("ytmusic")
	if h.health != nil {
		youtubeAvailable = youtubeAvailable && h.health.Available("ytmusic_python")
	}
	for _, name := range sorted {
		entry := providerCapability{Name: name, Enabled: h.cfg.Lifecycle.ProviderEnabled(name), TrackURLInput: urlProviders[name], TrackIDField: idFields[name], Operations: []string{}}
		if h.registry != nil {
			if source, ok := h.registry.Get(name); ok {
				entry.TrackAdapterRegistered = true
				entry.Version = source.Version()
			}
		}
		entry.TrackRuntimeAvailable = entry.Enabled && entry.TrackAdapterRegistered
		if h.registry != nil {
			active, _ := h.registry.ListActive([]string{name})
			entry.TrackRuntimeAvailable = entry.TrackRuntimeAvailable && len(active) > 0
		}
		if h.health != nil {
			entry.TrackRuntimeAvailable = entry.TrackRuntimeAvailable && h.health.Available(name)
		}
		if entry.TrackRuntimeAvailable {
			trackAvailable = true
			for _, op := range []string{"resolve", "links", "extract", "bulk", "search"} {
				if h.cfg.API.Enabled(op) && (op != "bulk" || h.cfg.BulkMaxItems > 0) {
					entry.Operations = append(entry.Operations, op)
				}
			}
		}
		artistReady := entry.Enabled && h.artists != nil && artists.Supported(name)
		if artistReady && h.artists.Available != nil {
			artistReady = h.artists.Available(name)
		}
		if artistReady && name == "ytmusic" {
			artistReady = h.artists.Pool != nil
		}
		if artistReady && h.artists.Health != nil {
			key := name
			if name == "ytmusic" {
				key = "ytmusic_python"
			}
			artistReady = h.artists.Health.Available(key)
		}
		if artistReady {
			artistAvailable = true
			if h.cfg.API.Enabled("artists") {
				entry.Operations = append(entry.Operations, "artists")
			}
		}
		if name == "ytmusic" && youtubeAvailable && h.cfg.API.Enabled("catalog") {
			entry.Operations = append(entry.Operations, "catalog")
		}
		sort.Strings(entry.Operations)
		profiles = append(profiles, entry)
	}
	endpoints := map[string]endpointCapability{}
	for _, op := range []string{"resolve", "links", "extract", "bulk", "search", "artists", "catalog", "fingerprint", "health", "providers", "query", "capabilities"} {
		ready := true
		switch op {
		case "resolve", "links", "extract", "bulk", "search":
			ready = trackAvailable
			if op == "bulk" {
				ready = ready && h.cfg.BulkMaxItems > 0
			}
		case "artists":
			ready = artistAvailable
		case "catalog":
			ready = youtubeAvailable
		case "fingerprint":
			ready = h.fingerprint != nil && h.cfg.AcoustIDKey != ""
		}
		method := "GET"
		if op == "bulk" || op == "query" || op == "fingerprint" {
			method = "POST"
		}
		enabled := h.cfg.API.Enabled(op)
		endpoints[op] = endpointCapability{Method: method, Path: "/v1/" + op, Enabled: enabled, RuntimeAvailable: enabled && ready}
	}
	bulkWorkers := h.cfg.BulkWorkers
	if bulkWorkers <= 0 {
		bulkWorkers = 10
	}
	defaultOp := h.cfg.API.DefaultOperation
	if defaultOp == "" {
		defaultOp = "resolve"
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"capabilities_version": 1,
		"api":                  map[string]any{"default_operation": defaultOp, "allow_debug": h.cfg.API.DebugAllowed(), "allow_overrides": h.cfg.API.OverridesAllowed(), "endpoints": endpoints},
		"providers":            profiles,
		"limits":               map[string]any{"bulk_max_items": h.cfg.BulkMaxItems, "bulk_workers": bulkWorkers, "search_max_limit": 50, "artist_max_limit": 50, "catalog_max_limit": 100, "request_timeout_ms": h.cfg.ProviderTimeout.Milliseconds()},
	})
}
