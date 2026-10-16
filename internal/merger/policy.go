package merger

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	"github.com/ommr/ommr/internal/metadata"
	"github.com/ommr/ommr/internal/models/canonical"
)

type Policy struct {
	Mode       string              `json:"mode"`
	Priorities map[string][]string `json:"priorities,omitempty"`
}

func ParsePolicy(raw string) (Policy, error) {
	policy := Policy{Mode: "consensus"}
	if strings.TrimSpace(raw) != "" {
		decoder := json.NewDecoder(strings.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&policy); err != nil {
			return policy, fmt.Errorf("invalid metadata policy JSON")
		}
		var trailing any
		if decoder.Decode(&trailing) != io.EOF {
			return policy, fmt.Errorf("metadata policy must be one JSON object")
		}
	}
	if policy.Mode != "consensus" && policy.Mode != "priority" {
		return policy, fmt.Errorf("metadata policy mode must be consensus or priority")
	}
	for field, names := range policy.Priorities {
		if !policyFields[field] {
			return policy, fmt.Errorf("unsupported metadata policy field %q", field)
		}
		seen := map[string]bool{}
		for i, name := range names {
			name = strings.ToLower(strings.TrimSpace(name))
			if name == "" || seen[name] {
				return policy, fmt.Errorf("invalid or duplicate metadata policy provider")
			}
			names[i] = name
			seen[name] = true
		}
	}
	return policy, nil
}

var policyFields = map[string]bool{"title": true, "artists": true, "album": true, "duration_ms": true, "release_date": true, "isrc": true, "iswc": true, "language": true, "label": true, "barcode": true, "preview_url": true, "track_number": true, "disc_number": true, "explicit": true}

func rank(provider string, priority []string) int {
	for i, name := range priority {
		if strings.EqualFold(provider, name) {
			return i
		}
	}
	return len(priority)
}
func (m *Merger) applyPolicy(track *canonical.Track, candidates []canonical.TrackCandidate) {
	value := reflect.ValueOf(track).Elem()
	typ := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]
		if !policyFields[field] {
			continue
		}
		priorities := titlePriority
		switch field {
		case "isrc":
			priorities = isrcPriority
		case "label":
			priorities = labelPriority
		case "preview_url":
			priorities = previewPriority
		}
		if names, ok := m.policy.Priorities[field]; ok {
			priorities = names
		}
		type choice struct {
			candidate canonical.TrackCandidate
			value     reflect.Value
			key       string
		}
		choices := []choice{}
		counts := map[string]int{}
		for _, candidate := range candidates {
			v := reflect.ValueOf(candidate.Track).Field(i)
			if field == "explicit" {
				b, known := metadata.Explicit(candidate)
				if !known {
					continue
				}
				v = reflect.ValueOf(b)
			} else if v.IsZero() || ((v.Kind() == reflect.Slice || v.Kind() == reflect.Map) && v.Len() == 0) {
				continue
			}
			encoded, _ := json.Marshal(v.Interface())
			key := strings.ToLower(strings.TrimSpace(string(encoded)))
			// Artist identity and album title vote independently of provider-specific IDs.
			if field == "artists" {
				names := []string{}
				for _, a := range candidate.Track.Artists {
					names = append(names, strings.ToLower(strings.TrimSpace(a.Name)))
				}
				encoded, _ = json.Marshal(names)
				key = string(encoded)
			}
			if field == "album" {
				key = strings.ToLower(strings.TrimSpace(candidate.Track.Album.Title))
				if key == "" {
					continue
				}
			}
			choices = append(choices, choice{candidate, v, key})
			counts[key]++
		}
		sort.SliceStable(choices, func(i, j int) bool {
			a, b := choices[i], choices[j]
			// Explicit per-field priorities override consensus when specified.
			if _, override := m.policy.Priorities[field]; !override && m.policy.Mode == "consensus" && counts[a.key] != counts[b.key] {
				return counts[a.key] > counts[b.key]
			}
			ra, rb := rank(a.candidate.Provider, priorities), rank(b.candidate.Provider, priorities)
			if ra != rb {
				return ra < rb
			}
			if a.candidate.MatchScore != b.candidate.MatchScore {
				return a.candidate.MatchScore > b.candidate.MatchScore
			}
			return a.candidate.Provider < b.candidate.Provider
		})
		if len(choices) > 0 {
			value.Field(i).Set(choices[0].value)
			if field == "album" {
				album := track.Album
				album.IDs = map[string]string{}
				for _, choice := range choices {
					if choice.key != choices[0].key {
						continue
					}
					a := choice.candidate.Track.Album
					for key, id := range a.IDs {
						if key == choice.candidate.Provider && album.IDs[key] == "" {
							album.IDs[key] = id
						}
					}
					if album.Label == "" {
						album.Label = a.Label
					}
					if album.UPC == "" {
						album.UPC = a.UPC
					}
					if album.Barcode == "" {
						album.Barcode = a.Barcode
					}
					if album.TotalTracks == 0 {
						album.TotalTracks = a.TotalTracks
					}
					if album.Type == "" {
						album.Type = a.Type
					}
					if album.ReleaseDate == "" {
						album.ReleaseDate = a.ReleaseDate
					}
				}
				track.Album = album
			}
			sources := []string{}
			for _, c := range choices {
				if c.key == choices[0].key {
					sources = append(sources, c.candidate.Provider)
				}
			}
			sort.Strings(sources)
			track.FieldSources[field] = sources
			if field == "duration_ms" {
				track.FieldSources["duration"] = sources
			}
		}
		if field == "explicit" {
			status := "unknown"
			if len(choices) > 0 {
				status = "clean"
				if track.Explicit {
					status = "explicit"
				}
			}
			if track.Extensions == nil {
				track.Extensions = map[string]json.RawMessage{}
			}
			track.Extensions["explicit_status"], _ = json.Marshal(status)
		}
	}
}
