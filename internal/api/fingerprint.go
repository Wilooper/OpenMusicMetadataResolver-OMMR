package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
)

func (h *Handler) HandleFingerprint(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}
	if h.cfg.AcoustIDKey == "" {
		w.WriteHeader(503)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "AcoustID application key is not configured"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 128<<10)
	var input struct {
		Fingerprint string `json:"fingerprint"`
		Duration    int    `json:"duration"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&input); err != nil {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON payload"})
		return
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF || input.Fingerprint == "" || len(input.Fingerprint) > 65536 || input.Duration < 1 || input.Duration > 86400 {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Provide a fingerprint and duration in seconds (1..86400) in one JSON object"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.ProviderTimeout)
	defer cancel()
	var lease uint64
	if h.health != nil {
		var err error
		if lease, err = h.health.Begin("acoustid"); err != nil {
			w.WriteHeader(503)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Fingerprint provider is unavailable"})
			return
		}
	}
	result, err := h.fingerprint.Lookup(ctx, input.Fingerprint, input.Duration)
	if h.health != nil {
		h.health.Finish(ctx, "acoustid", lease, err)
	}
	if err != nil {
		w.WriteHeader(502)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Fingerprint lookup unavailable"})
		return
	}
	_ = json.NewEncoder(w).Encode(result)
}
