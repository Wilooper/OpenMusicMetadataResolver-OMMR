package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/ommr/ommr/internal/artists"
)

func (h *Handler) SetArtists(service *artists.Service) { h.artists = service }

func (h *Handler) HandleArtists(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fail := func(status int, message string) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
	}
	if r.Method != http.MethodGet {
		fail(405, "Use GET")
		return
	}
	params := r.URL.Query()
	allowed := map[string]bool{"artist": true, "provider": true, "id": true, "sources": true, "limit": true, "debug": true, "bypass_cache": true}
	for key, values := range params {
		if !allowed[key] || len(values) != 1 {
			fail(400, "unknown or repeated artist query field")
			return
		}
	}
	debug, valid := parseDebug(w, r)
	if !valid {
		return
	}
	q := artists.Query{Artist: strings.TrimSpace(params.Get("artist")), Provider: strings.ToLower(strings.TrimSpace(params.Get("provider"))), ID: strings.TrimSpace(params.Get("id")), Limit: 10}
	if q.Provider == "musicbrainz" {
		q.ID = strings.ToLower(q.ID)
	}
	if params.Has("sources") {
		for _, name := range strings.Split(params.Get("sources"), ",") {
			name = strings.ToLower(strings.TrimSpace(name))
			if name == "" {
				fail(400, "sources must contain artist provider names")
				return
			}
			q.Sources = append(q.Sources, name)
		}
	}
	if params.Has("limit") {
		n, err := strconv.Atoi(params.Get("limit"))
		if err != nil {
			fail(400, "invalid artist limit")
			return
		}
		q.Limit = n
	}
	if params.Has("bypass_cache") {
		v, err := strconv.ParseBool(params.Get("bypass_cache"))
		if err != nil {
			fail(400, "invalid bypass_cache")
			return
		}
		q.BypassCache = v
	}
	if err := artists.Validate(q); err != nil {
		fail(400, err.Error())
		return
	}
	if h.artists == nil {
		fail(503, "artist profiles are not available")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.ProviderTimeout)
	defer cancel()
	result := h.artists.Resolve(ctx, q)
	if !debug {
		result.Sources = nil
	}
	_ = json.NewEncoder(w).Encode(result)
}
