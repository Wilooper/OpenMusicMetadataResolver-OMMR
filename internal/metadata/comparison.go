package metadata

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/ommr/ommr/internal/models/canonical"
)

func Compare(track *canonical.Track, candidates []canonical.TrackCandidate) {
	if track == nil || len(candidates) < 2 {
		return
	}
	fields := map[string]bool{"title": true, "artists": true, "album": true, "duration_ms": true, "release_date": true, "explicit": true, "isrc": true, "iswc": true, "language": true, "label": true, "barcode": true, "track_number": true, "disc_number": true}
	type comparison struct {
		Selected        any            `json:"selected"`
		SelectedSources []string       `json:"selected_sources"`
		Values          map[string]any `json:"values"`
		Different       bool           `json:"different"`
	}
	results := map[string]comparison{}
	selected := reflect.ValueOf(*track)
	typ := selected.Type()
	for i := 0; i < selected.NumField(); i++ {
		name := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]
		if !fields[name] {
			continue
		}
		values := map[string]any{}
		distinct := map[string]bool{}
		for _, candidate := range candidates {
			value := reflect.ValueOf(candidate.Track).Field(i)
			var data any
			if name == "explicit" {
				v, known := Explicit(candidate)
				if !known {
					continue
				}
				data = v
			} else {
				if value.IsZero() || (value.Kind() == reflect.Slice && value.Len() == 0) {
					continue
				}
				data = value.Interface()
				if name == "album" {
					if candidate.Track.Album.Title == "" {
						continue
					}
					data = candidate.Track.Album.Title
				}
				if name == "artists" {
					names := []string{}
					for _, a := range candidate.Track.Artists {
						names = append(names, a.Name)
					}
					data = names
				}
			}
			values[candidate.Provider] = data
			encoded, _ := json.Marshal(data)
			distinct[string(encoded)] = true
		}
		if len(values) < 2 {
			continue
		}
		data := selected.Field(i).Interface()
		if name == "album" {
			data = track.Album.Title
		}
		if name == "artists" {
			names := []string{}
			for _, a := range track.Artists {
				names = append(names, a.Name)
			}
			data = names
		}
		results[name] = comparison{data, track.FieldSources[name], values, len(distinct) > 1}
	}
	if len(results) > 0 {
		if track.Extensions == nil {
			track.Extensions = map[string]json.RawMessage{}
		}
		track.Extensions["metadata_comparison"], _ = json.Marshal(results)
	}
}
