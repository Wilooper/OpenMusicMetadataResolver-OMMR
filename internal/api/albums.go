package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/ommr/ommr/internal/albums"
)

func (h *Handler) SetAlbums(service *albums.Service) { h.albums = service }
func (h *Handler) HandleAlbums(w http.ResponseWriter, r *http.Request) {
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
	allowed := map[string]bool{"provider": true, "id": true, "limit": true, "debug": true, "bypass_cache": true}
	for key, values := range params {
		if !allowed[key] || len(values) != 1 {
			fail(400, "unknown or repeated album query field")
			return
		}
	}
	debug, valid := parseDebug(w, r)
	if !valid {
		return
	}
	q := albums.Query{Provider: strings.ToLower(strings.TrimSpace(params.Get("provider"))), ID: strings.TrimSpace(params.Get("id")), Limit: albums.MaxTracks}
	if q.Provider == "musicbrainz" {
		q.ID = strings.ToLower(q.ID)
	}
	if params.Has("limit") {
		n, err := strconv.Atoi(params.Get("limit"))
		if err != nil {
			fail(400, "invalid album limit")
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
	if err := albums.Validate(q); err != nil {
		fail(400, err.Error())
		return
	}
	if h.albums == nil || !h.cfg.Lifecycle.ProviderEnabled(q.Provider) {
		fail(503, "album service unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.ProviderTimeout)
	defer cancel()
	result, err := h.albums.Resolve(ctx, q)
	if err != nil {
		switch {
		case errors.Is(err, albums.ErrNotFound):
			fail(404, "album not found")
		case errors.Is(err, albums.ErrUnavailable):
			fail(503, "album provider unavailable")
		case errors.Is(err, context.DeadlineExceeded):
			fail(504, "album deadline exceeded")
		default:
			fail(502, "album metadata unavailable")
		}
		return
	}
	if !debug {
		result.Source = nil
	}
	_ = json.NewEncoder(w).Encode(result)
}
