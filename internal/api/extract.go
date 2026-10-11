package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/platformlinks"
)

var extractionFields = []string{"title", "artists", "album", "album_type", "release_date", "year", "artwork", "duration_ms", "isrc", "iswc", "genres", "language", "explicit", "preview_url", "track_number", "disc_number", "label", "barcode", "credits", "lyricist", "composer", "producer", "wikipedia_context"}

type extractionSource struct {
	Provider string           `json:"provider"`
	Version  string           `json:"version"`
	Status   string           `json:"status"`
	Error    string           `json:"error,omitempty"`
	Results  []extractedTrack `json:"results,omitempty"`
}

type extractedTrack struct {
	Track         canonical.Track       `json:"track"`
	Year          int                   `json:"year,omitempty"`
	ThumbnailURL  string                `json:"thumbnail_url,omitempty"`
	PresentFields []string              `json:"present_fields"`
	MissingFields []string              `json:"missing_fields"`
	Links         platformlinks.Catalog `json:"links"`
}

type extractionResponse struct {
	Query   adapters.Query     `json:"query"`
	Sources []extractionSource `json:"sources"`
}

// HandleExtract returns each provider's own normalized response and field coverage.
func (h *Handler) HandleExtract(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	providerID := strings.TrimSpace(q.Get("id"))
	providerName := strings.ToLower(strings.TrimSpace(q.Get("provider")))
	artist, title := strings.TrimSpace(q.Get("artist")), strings.TrimSpace(q.Get("title"))
	if (providerID != "" && providerName == "") || (providerID == "" && (artist == "" || title == "")) {
		http.Error(w, `{"error":"Provide provider and id, or both artist and title"}`, http.StatusBadRequest)
		return
	}

	var active []adapters.ProviderAdapter
	if providerName != "" {
		adapter, ok := h.registry.Get(providerName)
		if !ok {
			http.Error(w, `{"error":"Provider is not registered"}`, http.StatusBadRequest)
			return
		}
		active = []adapters.ProviderAdapter{adapter}
	} else {
		names := []string{}
		if raw := q.Get("sources"); raw != "" {
			names = strings.Split(raw, ",")
		}
		var err error
		active, err = h.registry.ListActive(names)
		if err != nil {
			http.Error(w, `{"error":"Unknown provider in sources"}`, http.StatusBadRequest)
			return
		}
	}
	sort.Slice(active, func(i, j int) bool { return active[i].Name() < active[j].Name() })

	timeout := h.cfg.ProviderTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	query := adapters.Query{Artist: artist, Title: title, Album: strings.TrimSpace(q.Get("album")), ISRC: strings.TrimSpace(q.Get("isrc"))}
	response := extractionResponse{Query: query, Sources: make([]extractionSource, len(active))}
	var wg sync.WaitGroup
	for index, adapter := range active {
		index, adapter := index, adapter
		wg.Add(1)
		go func() {
			defer wg.Done()
			source := extractionSource{Provider: adapter.Name(), Version: adapter.Version()}
			if err := h.resolver.WaitProvider(ctx, adapter.Name()); err != nil {
				source.Status, source.Error = "error", err.Error()
				response.Sources[index] = source
				return
			}
			var candidates []canonical.TrackCandidate
			var err error
			if providerID != "" {
				candidate, fetchErr := adapter.FetchByID(ctx, providerName, providerID)
				err = fetchErr
				if candidate != nil {
					candidates = []canonical.TrackCandidate{*candidate}
				}
			} else {
				candidates, err = adapter.Search(ctx, query)
			}
			if err != nil {
				source.Status, source.Error = "error", err.Error()
			} else if len(candidates) == 0 {
				source.Status = "no_results"
			} else {
				source.Status = "ok"
				source.Results = make([]extractedTrack, 0, len(candidates))
				for _, candidate := range candidates {
					candidate.Track.IDs = platformlinks.ProviderIDs(adapter.Name(), candidate.Track.IDs)
					source.Results = append(source.Results, describeExtractedTrack(candidate.Track))
				}
			}
			response.Sources[index] = source
		}()
	}
	wg.Wait()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func describeExtractedTrack(track canonical.Track) extractedTrack {
	result := extractedTrack{Track: track, PresentFields: make([]string, 0, len(extractionFields)), MissingFields: make([]string, 0, len(extractionFields))}
	result.Links = platformlinks.Build(track)
	if len(track.Images) > 0 {
		var best canonical.Image
		for _, image := range track.Images {
			if image.URL == "" {
				continue
			}
			if best.URL == "" || image.Type == "thumbnail" && best.Type != "thumbnail" || (image.Type == "thumbnail") == (best.Type == "thumbnail") && int64(image.Width)*int64(image.Height) > int64(best.Width)*int64(best.Height) {
				best = image
			}
		}
		result.ThumbnailURL = best.URL
	}
	date := track.ReleaseDate
	if date == "" {
		date = track.Album.ReleaseDate
	}
	if len(date) >= 4 {
		result.Year, _ = strconv.Atoi(date[:4])
	}
	fields := map[string]bool{
		"title": track.Title != "", "artists": len(track.Artists) > 0,
		"album": track.Album.Title != "", "album_type": track.Album.Type != "", "release_date": date != "", "year": result.Year > 0,
		"artwork": result.ThumbnailURL != "", "duration_ms": track.DurationMS > 0,
		"isrc": track.ISRC != "", "credits": len(track.Credits) > 0,
		"lyricist": hasCreditRole(track.Credits, "lyricist"),
		"composer": hasCreditRole(track.Credits, "composer"), "producer": hasCreditRole(track.Credits, "producer"),
		"iswc": track.ISWC != "", "genres": len(track.Genres) > 0, "language": track.Language != "",
		"preview_url": track.PreviewURL != "", "track_number": track.TrackNumber > 0, "disc_number": track.DiscNumber > 0,
		"label": track.Label != "", "barcode": track.Barcode != "", "wikipedia_context": len(track.Extensions["wikipedia"]) > 0,
	}
	// A false explicit flag does not tell us whether the source positively
	// reports clean content, so it is only present when explicit is true.
	fields["explicit"] = track.Explicit
	for _, field := range extractionFields {
		if fields[field] {
			result.PresentFields = append(result.PresentFields, field)
		} else {
			result.MissingFields = append(result.MissingFields, field)
		}
	}
	sort.Strings(result.PresentFields)
	sort.Strings(result.MissingFields)
	return result
}

func hasCreditRole(credits []canonical.Credit, role string) bool {
	for _, credit := range credits {
		for _, candidate := range credit.Roles {
			if strings.EqualFold(candidate, role) {
				return true
			}
		}
	}
	return false
}
