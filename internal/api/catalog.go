package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"

	"github.com/ommr/ommr/internal/extractors"
)

func (h *Handler) SetExtractor(pool *extractors.Pool) { h.extractor = pool }

var catalogIDs = map[string]*regexp.Regexp{
	"artist_albums":  regexp.MustCompile(`^UC[A-Za-z0-9_-]{20,30}$`),
	"artist_singles": regexp.MustCompile(`^UC[A-Za-z0-9_-]{20,30}$`),
	"song":           regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`),
	"album":          regexp.MustCompile(`^MPRE[A-Za-z0-9_-]{1,150}$`),
	"artist":         regexp.MustCompile(`^UC[A-Za-z0-9_-]{20,30}$`),
	"playlist":       regexp.MustCompile(`^(OLAK5uy_|PL|RD)[A-Za-z0-9_-]{5,150}$`),
}

func (h *Handler) HandleCatalog(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fail := func(status int, message string) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
	}
	if r.Method != http.MethodGet {
		fail(405, "Use GET")
		return
	}
	if _, valid := parseDebug(w, r); !valid {
		return
	}
	q := r.URL.Query()
	kind, id := q.Get("type"), q.Get("id")
	if q.Get("provider") != "ytmusic" {
		fail(400, "catalog currently supports provider=ytmusic")
		return
	}
	pattern, ok := catalogIDs[kind]
	if !ok || !pattern.MatchString(id) {
		fail(400, "invalid catalog type or ID")
		return
	}
	limit := 20
	if q.Has("limit") {
		var err error
		limit, err = strconv.Atoi(q.Get("limit"))
		if err != nil || limit < 1 || limit > 100 {
			fail(400, "limit must be between 1 and 100")
			return
		}
	}
	if h.extractor == nil || !h.cfg.Lifecycle.ProviderEnabled("ytmusic") {
		fail(503, "YouTube catalog extractor is not enabled or available")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.ProviderTimeout)
	defer cancel()
	var lease uint64
	if h.health != nil {
		var err error
		lease, err = h.health.Begin("ytmusic_python")
		if err != nil {
			fail(503, "YouTube catalog extractor is suspended")
			return
		}
	}
	data, err := h.extractor.Call(ctx, kind, id, limit)
	if h.health != nil {
		if errors.Is(err, extractors.ErrBusy) {
			h.health.Skip("ytmusic_python", lease)
		} else {
			h.health.Finish(ctx, "ytmusic_python", lease, err)
		}
	}
	if err != nil {
		if errors.Is(err, extractors.ErrBusy) {
			fail(503, "extractor queue is full")
		} else if errors.Is(err, context.DeadlineExceeded) {
			fail(504, "catalog deadline exceeded")
		} else {
			fail(502, "catalog metadata unavailable")
		}
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"provider": "ytmusic", "type": kind, "id": id, "data": data})
}
