package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/ommr/ommr/internal/providerhealth"
)

func (h *Handler) SetProviderHealth(manager *providerhealth.Manager) { h.health = manager }
func (h *Handler) withDefaults(operation string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.cfg.API.Enabled(operation) {
			http.NotFound(w, r)
			return
		}
		clone := r.Clone(r.Context())
		u := *r.URL
		clone.URL = &u
		q := u.Query()
		overrides := h.cfg.API.OverridesAllowed()
		if !overrides || !q.Has("debug") {
			q.Set("debug", strconv.FormatBool(h.cfg.API.Debug))
		}
		if !h.cfg.API.DebugAllowed() {
			q.Set("debug", "false")
		}
		if !overrides || !q.Has("bypass_cache") {
			q.Set("bypass_cache", strconv.FormatBool(h.cfg.API.BypassCache))
		}
		if !overrides || !q.Has("sources") {
			if len(h.cfg.API.DefaultSources) > 0 {
				q.Set("sources", strings.Join(h.cfg.API.DefaultSources, ","))
			} else {
				q.Del("sources")
			}
		}
		if operation == "search" && (!overrides || !q.Has("limit")) {
			limit := h.cfg.API.SearchLimit
			if limit < 1 {
				limit = 10
			}
			q.Set("limit", strconv.Itoa(limit))
		}
		clone.URL.RawQuery = q.Encode()
		next(w, clone)
	}
}
func (h *Handler) HandleProviders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		w.WriteHeader(405)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}
	if h.health == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"providers": []any{}})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"providers": h.health.Snapshot()})
}
func (h *Handler) HandleQuery(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fail := func(message string) {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Use POST with a JSON query"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	var payload map[string]json.RawMessage
	if decoder.Decode(&payload) != nil || payload == nil {
		fail("Invalid JSON query")
		return
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		fail("Request must contain one JSON object")
		return
	}
	operation := h.cfg.API.DefaultOperation
	if operation == "" {
		operation = "resolve"
	}
	if raw, ok := payload["operation"]; ok {
		var requested string
		if json.Unmarshal(raw, &requested) != nil {
			fail("operation must be a string")
			return
		}
		if h.cfg.API.OverridesAllowed() {
			operation = requested
		}
	}
	delete(payload, "operation")
	routes := map[string]http.HandlerFunc{"resolve": h.HandleResolve, "links": h.HandleLinks, "search": h.HandleSearch, "extract": h.HandleExtract, "bulk": h.HandleBulk, "fingerprint": h.HandleFingerprint}
	handler, exists := routes[operation]
	if !exists {
		fail("Unsupported operation")
		return
	}
	if !h.cfg.API.Enabled(operation) {
		http.NotFound(w, r)
		return
	}
	clone := r.Clone(r.Context())
	u := *r.URL
	clone.URL = &u
	clone.URL.Path = "/v1/" + operation
	if operation == "bulk" || operation == "fingerprint" {
		body, _ := json.Marshal(payload)
		clone.Body = io.NopCloser(bytes.NewReader(body))
		clone.ContentLength = int64(len(body))
		h.withDefaults(operation, handler)(w, clone)
		return
	}
	if raw, ok := payload["query"]; ok {
		var query map[string]json.RawMessage
		if json.Unmarshal(raw, &query) != nil || query == nil {
			fail("query must be an object")
			return
		}
		delete(payload, "query")
		for key, value := range query {
			if _, duplicate := payload[key]; duplicate {
				fail("Duplicate query field")
				return
			}
			payload[key] = value
		}
	}
	params := url.Values{}
	allowed := map[string]bool{"spotify_id": true, "youtube_id": true, "deezer_id": true, "apple_id": true, "soundcloud_id": true, "qobuz_id": true, "tidal_id": true, "amazonmusic_id": true, "amazon_music_id": true, "pandora_id": true, "isrc": true, "artist": true, "title": true, "album": true, "sources": true, "debug": true, "bypass_cache": true, "q": true, "page": true, "limit": true, "provider": true, "id": true}
	for key, raw := range payload {
		if !allowed[key] {
			fail("Unknown query field: " + key)
			return
		}
		if key == "sources" {
			var sources []string
			if json.Unmarshal(raw, &sources) == nil {
				params.Set(key, strings.Join(sources, ","))
				continue
			}
		}
		var value any
		if json.Unmarshal(raw, &value) != nil {
			fail("Invalid query field")
			return
		}
		switch value.(type) {
		case string, bool, float64:
			params.Set(key, fmt.Sprint(value))
		default:
			fail("Query fields must be scalar values")
			return
		}
	}
	clone.Method = http.MethodGet
	clone.Body = nil
	clone.ContentLength = 0
	clone.URL.RawQuery = params.Encode()
	h.withDefaults(operation, handler)(w, clone)
}
