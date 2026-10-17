package catalogapi

import (
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
)

func entityJSON(item map[string]any) []byte { body, _ := json.Marshal(item); return body }

func findSingleTrack(root any) map[string]any {
	items := findTrackItems(root)
	if len(items) > 0 {
		return items[0]
	}
	return nil
}

func findTrackItems(root any) []map[string]any {
	index := map[string]map[string]any{}
	if envelope, ok := root.(map[string]any); ok {
		if included, ok := envelope["included"].([]any); ok {
			for _, entry := range included {
				if obj, ok := entry.(map[string]any); ok {
					index[resourceKey(obj)] = obj
				}
			}
		}
	}
	found := []map[string]any{}
	seen := map[string]bool{}
	var visit func(any, int)
	visit = func(value any, depth int) {
		if depth > 12 {
			return
		}
		switch obj := value.(type) {
		case []any:
			for _, entry := range obj {
				visit(entry, depth+1)
			}
		case map[string]any:
			kind := strings.ToLower(firstString(obj, "type"))
			if kind == "tracks" && obj["attributes"] == nil {
				if full := index[resourceKey(obj)]; full != nil {
					obj = full
				}
			}
			view := resourceView(obj, index, 0)
			if (kind == "" || kind == "track" || kind == "tracks") && firstString(view, "title", "name") != "" && (view["artists"] != nil || view["artist"] != nil || view["performer"] != nil || view["album"] != nil || kind == "tracks") {
				id := firstString(view, "id", "trackId")
				if id == "" || !seen[id] {
					found = append(found, view)
					seen[id] = true
				}
				return
			}
			// Only documented entity containers are searched; included albums/artists are not tracks.
			for _, key := range []string{"data", "entity", "tracks", "items", "entities"} {
				if child, ok := obj[key]; ok {
					visit(child, depth+1)
				}
			}
		}
	}
	visit(root, 0)
	return found
}

func resourceKey(obj map[string]any) string {
	return firstString(obj, "type") + ":" + firstString(obj, "id")
}

func linked(obj map[string]any, relation string, index map[string]map[string]any) []map[string]any {
	relationships, _ := obj["relationships"].(map[string]any)
	link, _ := relationships[relation].(map[string]any)
	data := link["data"]
	refs, _ := data.([]any)
	if one, ok := data.(map[string]any); ok {
		refs = []any{one}
	}
	result := []map[string]any{}
	for _, ref := range refs {
		item, _ := ref.(map[string]any)
		if value := index[resourceKey(item)]; value != nil {
			result = append(result, value)
		}
	}
	return result
}

func resourceView(obj map[string]any, index map[string]map[string]any, depth int) map[string]any {
	attrs, _ := obj["attributes"].(map[string]any)
	view := mergeMaps(obj, attrs)
	if depth >= 3 {
		return view
	}
	if artists := linked(obj, "artists", index); len(artists) > 0 {
		values := make([]any, 0, len(artists))
		for _, artist := range artists {
			if firstString(artist, "type") == "artists" {
				values = append(values, resourceView(artist, index, depth+1))
			}
		}
		view["artists"] = values
	}
	if albums := linked(obj, "albums", index); len(albums) > 0 && firstString(albums[0], "type") == "albums" {
		view["album"] = resourceView(albums[0], index, depth+1)
	}
	if artwork := linked(obj, "coverArt", index); len(artwork) > 0 {
		art := resourceView(artwork[0], index, depth+1)
		if files, ok := art["files"].([]any); ok {
			view["images"] = files
		}
	}
	return view
}

var isoDuration = regexp.MustCompile(`^PT(?:(\d+(?:\.\d+)?)H)?(?:(\d+(?:\.\d+)?)M)?(?:(\d+(?:\.\d+)?)S)?$`)

func durationMilliseconds(value any, milliseconds bool) int64 {
	var n float64
	switch v := value.(type) {
	case json.Number:
		n, _ = v.Float64()
	case float64:
		n = v
	case string:
		if strings.HasPrefix(v, "PT") {
			parts := isoDuration.FindStringSubmatch(v)
			if parts == nil || v == "PT" {
				return 0
			}
			for idx, scale := range []float64{3600, 60, 1} {
				component, _ := strconv.ParseFloat(parts[idx+1], 64)
				n += component * scale
			}
		} else {
			n, _ = strconv.ParseFloat(v, 64)
		}
	default:
		return 0
	}
	if !milliseconds {
		n *= 1000
	}
	if math.IsNaN(n) || math.IsInf(n, 0) || n <= 0 || n > 86400000 {
		return 0
	}
	return int64(math.Round(n))
}

func labelName(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	if obj, ok := value.(map[string]any); ok {
		return firstString(obj, "name", "title")
	}
	return ""
}

func explicitValue(obj map[string]any) (bool, bool) {
	for _, key := range []string{"explicit", "explicit_lyrics"} {
		if value, ok := obj[key].(bool); ok {
			return value, true
		}
	}
	if parental, ok := obj["parentalSettings"].(map[string]any); ok {
		if value, ok := parental["hasExplicitLanguage"].(bool); ok {
			return value, true
		}
	}
	return false, false
}
