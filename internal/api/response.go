package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/platformlinks"
)

func parseDebug(w http.ResponseWriter, r *http.Request) (bool, bool) {
	values, present := r.URL.Query()["debug"]
	if !present {
		return false, true
	}
	if len(values) == 1 {
		if enabled, err := strconv.ParseBool(values[0]); err == nil {
			return enabled, true
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "debug must be a single boolean value (true or false)"})
	return false, false
}

// Keep the public schema explicit so new internal diagnostics cannot leak by default.
type publicTrack struct {
	CanonicalID    string                `json:"canonical_id"`
	IdentityStatus string                `json:"identity_status,omitempty"`
	Title          string                `json:"title"`
	Artists        []canonical.Artist    `json:"artists"`
	Album          canonical.Album       `json:"album"`
	DurationMS     int64                 `json:"duration_ms"`
	ReleaseDate    string                `json:"release_date"`
	Explicit       bool                  `json:"explicit"`
	ISRC           string                `json:"isrc,omitempty"`
	ISWC           string                `json:"iswc,omitempty"`
	Genres         []string              `json:"genres,omitempty"`
	Language       string                `json:"language,omitempty"`
	Credits        []canonical.Credit    `json:"credits,omitempty"`
	Images         []canonical.Image     `json:"images,omitempty"`
	PreviewURL     string                `json:"preview_url,omitempty"`
	TrackNumber    int                   `json:"track_number,omitempty"`
	DiscNumber     int                   `json:"disc_number,omitempty"`
	Label          string                `json:"label,omitempty"`
	Barcode        string                `json:"barcode,omitempty"`
	Copyrights     []canonical.Copyright `json:"copyrights,omitempty"`
	PlayCount      int64                 `json:"play_count,omitempty"`
	IDs            map[string]string     `json:"ids"`
	Extensions     map[string]any        `json:"extensions,omitempty"`
}

func trackResponse(track *canonical.Track, debug bool) any {
	if track == nil {
		return nil
	}
	if debug {
		return track
	}
	extensions := map[string]any{}
	if context := track.Extensions["wikipedia"]; len(context) > 0 {
		// Context includes article attribution; this is metadata, not a fetch trace.
		extensions["wikipedia"] = context
	}
	extensions["platform_links"] = catalogResponse(platformlinks.Build(*track), false)
	return publicTrack{
		CanonicalID: track.CanonicalID, IdentityStatus: track.IdentityStatus,
		Title: track.Title, Artists: track.Artists, Album: track.Album,
		DurationMS: track.DurationMS, ReleaseDate: track.ReleaseDate, Explicit: track.Explicit,
		ISRC: track.ISRC, ISWC: track.ISWC, Genres: track.Genres, Language: track.Language,
		Credits: track.Credits, Images: track.Images, PreviewURL: track.PreviewURL,
		TrackNumber: track.TrackNumber, DiscNumber: track.DiscNumber, Label: track.Label,
		Barcode: track.Barcode, Copyrights: track.Copyrights, PlayCount: track.PlayCount,
		IDs: track.IDs, Extensions: extensions,
	}
}

func catalogResponse(catalog platformlinks.Catalog, debug bool) any {
	if debug {
		return catalog
	}
	type link struct {
		ID       string `json:"id"`
		IDType   string `json:"id_type"`
		LookupID string `json:"lookup_id,omitempty"`
		URL      string `json:"url,omitempty"`
	}
	links := make(map[string]link, len(catalog.Platforms))
	for provider, item := range catalog.Platforms {
		links[provider] = link{ID: item.ID, IDType: item.IDType, LookupID: item.LookupID, URL: item.URL}
	}
	return struct {
		OMMRID    string          `json:"ommr_id,omitempty"`
		ISRC      string          `json:"isrc,omitempty"`
		Platforms map[string]link `json:"platforms"`
	}{catalog.OMMRID, catalog.ISRC, links}
}

func searchResponse(result canonical.SearchResult, debug bool) any {
	if debug {
		return result
	}
	return struct {
		CanonicalID string             `json:"canonical_id"`
		Title       string             `json:"title"`
		Artists     []canonical.Artist `json:"artists"`
		Album       string             `json:"album"`
		Images      []canonical.Image  `json:"images"`
		AvailableOn []string           `json:"available_on"`
	}{result.CanonicalID, result.Title, result.Artists, result.Album, result.Images, result.AvailableOn}
}
