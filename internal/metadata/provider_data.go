package metadata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/musicgraph"
)

// Provider data contains only accepted catalog records, never a complete search/player response.
func Record(c canonical.TrackCandidate) map[string]any {
	data, _ := json.Marshal(c.Track)
	var fields map[string]any
	_ = json.Unmarshal(data, &fields)
	for _, key := range []string{"canonical_id", "identity_status", "identity_reasons", "identity_matches", "sources", "field_sources", "match_score", "match_breakdown", "confidence", "completeness", "relations", "extensions"} {
		delete(fields, key)
	}
	ids := map[string]string{}
	for key, value := range c.Track.IDs {
		if key == c.Provider || key == c.Provider+"_url" || key == c.Provider+"_track_id" {
			ids[key] = value
		}
	}
	fields["ids"] = ids
	record := map[string]any{"metadata": fields}
	if len(c.Track.Extensions) > 0 {
		record["extra"] = sanitizeExtensions(c.Track.Extensions)
	}
	if len(c.RawResponse) > 0 && len(c.RawResponse) <= 8<<20 {
		decoder := json.NewDecoder(bytes.NewReader(c.RawResponse))
		decoder.UseNumber()
		var raw any
		if decoder.Decode(&raw) == nil {
			ids := map[string]bool{}
			for key, id := range c.Track.IDs {
				if !strings.HasSuffix(key, "_url") && id != "" {
					ids[id] = true
				}
			}
			if obj := findRecord(raw, ids, 0); obj != nil {
				record["catalog"] = clean(obj, 0)
			}
		}
	}
	return record
}

func sanitizeExtensions(values map[string]json.RawMessage) map[string]any {
	result := map[string]any{}
	for key, value := range values {
		if blocked(key) || key == "provider_data" || key == "platform_links" {
			continue
		}
		var data any
		if json.Unmarshal(value, &data) == nil {
			result[key] = clean(data, 0)
		}
	}
	return result
}

func findRecord(value any, ids map[string]bool, depth int) map[string]any {
	if depth > 16 {
		return nil
	}
	switch item := value.(type) {
	case map[string]any:
		// Public JioSaavn JSON-LD may expose only its verified share token, not a native lookup ID.
		if raw, ok := item["perma_url"].(string); ok {
			if u, err := url.Parse(raw); err == nil && u.Scheme == "https" && u.User == nil && u.Port() == "" && (u.Hostname() == "www.jiosaavn.com" || u.Hostname() == "jiosaavn.com") {
				parts := strings.Split(strings.Trim(u.Path, "/"), "/")
				if len(parts) == 3 && parts[0] == "song" && ids[parts[2]] {
					return item
				}
			}
		}
		for _, key := range []string{"id", "trackId", "videoId"} {
			if v, ok := item[key]; ok && ids[fmt.Sprint(v)] {
				return item
			}
		}
		// Prefer entity containers to artist/album subobjects whose numeric IDs may collide.
		for _, key := range []string{"videoDetails", "recordings", "results", "data", "tracks", "items"} {
			if child := findRecord(item[key], ids, depth+1); child != nil {
				return child
			}
		}
	case []any:
		for _, v := range item {
			if child := findRecord(v, ids, depth+1); child != nil {
				return child
			}
		}
	}
	return nil
}

func blocked(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(key))
	for _, part := range []string{"token", "cookie", "authorization", "password", "secret", "signature", "encryptedmediaurl", "streamingdata", "playback", "tracking", "responsecontext", "logging", "html", "cipher", "streamurl", "streamingurl", "downloadurl"} {
		if strings.Contains(normalized, part) {
			return true
		}
	}
	return normalized == "apikey" || normalized == "clientkey" || normalized == "request" || normalized == "headers"
}
func clean(value any, depth int) any {
	if depth > 24 {
		return nil
	}
	switch item := value.(type) {
	case map[string]any:
		result := map[string]any{}
		for key, v := range item {
			if !blocked(key) {
				result[key] = clean(v, depth+1)
			}
		}
		return result
	case []any:
		result := make([]any, len(item))
		for i, v := range item {
			result[i] = clean(v, depth+1)
		}
		return result
	default:
		return value
	}
}

func Attach(track *canonical.Track, candidates []canonical.TrackCandidate) {
	if track == nil {
		return
	}
	records := map[string]any{}
	for _, candidate := range candidates {
		records[candidate.Provider] = Record(candidate)
	}
	if track.Extensions == nil {
		track.Extensions = map[string]json.RawMessage{}
	}
	extensions := make(map[string]json.RawMessage, len(track.Extensions)+1)
	for key, value := range track.Extensions {
		extensions[key] = value
	}
	track.Extensions = extensions
	if len(track.Extensions["explicit_status"]) == 0 {
		status := "unknown"
		for _, candidate := range candidates {
			if explicit, known := Explicit(candidate); known {
				status = "clean"
				if explicit {
					status = "explicit"
					break
				}
			}
		}
		track.Extensions["explicit_status"], _ = json.Marshal(status)
	}
	track.Extensions["provider_data"], _ = json.Marshal(records)
	Compare(track, candidates)
}

func Explicit(c canonical.TrackCandidate) (bool, bool) {
	if c.Track.Explicit {
		return true, true
	}
	record := Record(c)
	if raw, ok := record["catalog"].(map[string]any); ok {
		if attributes, ok := raw["attributes"].(map[string]any); ok {
			raw = attributes
		}
		if parental, ok := raw["parentalSettings"].(map[string]any); ok {
			if value, ok := parental["hasExplicitLanguage"].(bool); ok {
				return value, true
			}
		}
		for _, key := range []string{"explicit", "explicit_lyrics"} {
			if value, ok := raw[key].(bool); ok {
				return value, true
			}
		}
		for _, key := range []string{"trackExplicitness", "contentRating"} {
			if value, ok := raw[key].(string); ok {
				switch strings.ToLower(value) {
				case "explicit":
					return true, true
				case "cleaned", "notexplicit", "clean":
					return false, true
				}
			}
		}
	}
	return false, false
}

// Catalog sanitizes an entity record, including unrecognized catalog metadata.
// Callers must select an entity first, rather than pass account/player envelopes.
func Catalog(record map[string]any) map[string]any {
	return clean(record, 0).(map[string]any)
}

// AttachGraph only receives accepted source records and identity-checked enrichment.
func AttachGraph(track *canonical.Track, candidates []canonical.TrackCandidate) {
	if track == nil {
		return
	}
	records := make([]musicgraph.Record, 0, len(candidates))
	for _, candidate := range candidates {
		record := Record(candidate)
		catalog, _ := record["catalog"].(map[string]any)
		if candidate.Provider == "musicbrainz" {
			var detail map[string]any
			if json.Unmarshal(candidate.Track.Extensions["recording_details"], &detail) == nil && detail["id"] == candidate.Track.IDs["musicbrainz"] {
				catalog = Catalog(detail)
			}
		}
		records = append(records, musicgraph.Record{Provider: candidate.Provider, Track: candidate.Track, Catalog: catalog})
	}
	var data map[string]any
	_ = json.Unmarshal(track.Extensions["provider_data"], &data)
	enrichment, _ := data["musicbrainz_release"].(map[string]any)
	release, _ := enrichment["release"].(map[string]any)
	graph := musicgraph.TrackGraph(*track, records, release)
	if graph.Root == "" {
		return
	}
	if track.Extensions == nil {
		track.Extensions = map[string]json.RawMessage{}
	}
	track.Extensions["music_graph"], _ = json.Marshal(graph)
}
