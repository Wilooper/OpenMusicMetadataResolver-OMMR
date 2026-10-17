package metadata

import (
	"encoding/json"
	"reflect"

	"github.com/ommr/ommr/internal/models/canonical"
)

func Hydrate(original, detail canonical.TrackCandidate) canonical.TrackCandidate {
	target := reflect.ValueOf(&detail.Track).Elem()
	previous := reflect.ValueOf(original.Track)
	for i := 0; i < target.NumField(); i++ {
		field := target.Field(i)
		if target.Type().Field(i).Name == "Explicit" {
			continue
		}
		if field.IsZero() || ((field.Kind() == reflect.Slice || field.Kind() == reflect.Map) && field.Len() == 0) {
			field.Set(previous.Field(i))
		}
	}
	if _, known := Explicit(detail); !known {
		detail.Track.Explicit = original.Track.Explicit
	}
	extensions := map[string]json.RawMessage{}
	for key, value := range original.Track.Extensions {
		extensions[key] = append(json.RawMessage(nil), value...)
	}
	for key, value := range detail.Track.Extensions {
		extensions[key] = append(json.RawMessage(nil), value...)
	}
	detail.Track.Extensions = extensions
	// Both candidates already passed identity checks; retain search-only entity fields.
	merged := map[string]any{}
	for _, candidate := range []canonical.TrackCandidate{original, detail} {
		if catalog, ok := Record(candidate)["catalog"].(map[string]any); ok {
			for key, value := range catalog {
				merged[key] = value
			}
		}
	}
	if len(merged) > 0 {
		detail.RawResponse, _ = json.Marshal(merged)
	}
	return detail
}
