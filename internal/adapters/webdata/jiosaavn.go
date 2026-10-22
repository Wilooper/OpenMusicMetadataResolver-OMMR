package webdata

import (
	"encoding/json"
	"strings"
)

// The page's surrounding state is JavaScript (Date and undefined), but its
// top-level song property is JSON. Scan strings and braces without executing JS
// or treating recommendations elsewhere in the state as the requested song.
func jiosaavnPage(raw, id string) map[string]any {
	if !strings.HasPrefix(raw, "{") {
		return nil
	}
	depth, matches := 0, 0
	var selected map[string]any
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case '\'', '`', '/':
			// Only the observed serialized state is supported, not JS comments,
			// templates or arbitrary expressions containing apparent properties.
			return nil
		case '{', '[':
			depth++
			if depth > 64 {
				return nil
			}
		case '}', ']':
			depth--
			if depth == 0 {
				if matches == 1 {
					return selected
				}
				return nil
			}
			if depth < 0 {
				return nil
			}
		case '"':
			start := i
			i++
			for i < len(raw) {
				if raw[i] == '\\' {
					i += 2
					continue
				}
				if raw[i] == '"' {
					break
				}
				i++
			}
			if i >= len(raw) {
				return nil
			}
			if depth != 1 || raw[start:i+1] != `"song"` {
				continue
			}
			rest := strings.TrimSpace(raw[i+1:])
			if !strings.HasPrefix(rest, ":") {
				continue
			}
			valueJSON := strings.TrimSpace(rest[1:])
			decoder := json.NewDecoder(strings.NewReader(valueJSON))
			decoder.UseNumber()
			var value any
			if decoder.Decode(&value) != nil {
				return nil
			}
			tail := strings.TrimSpace(valueJSON[decoder.InputOffset():])
			if !strings.HasPrefix(tail, ",") && !strings.HasPrefix(tail, "}") {
				return nil
			}
			state, _ := value.(map[string]any)
			song, _ := state["song"].(map[string]any)
			record := jiosaavnDisplaySong(song, id)
			if record == nil {
				return nil
			}
			matches++
			selected = record
		}
	}
	return nil
}

func jiosaavnDisplaySong(song map[string]any, id string) map[string]any {
	title, _ := song["title"].(map[string]any)
	album, _ := song["album"].(map[string]any)
	copyright, _ := song["copyright"].(map[string]any)
	if str(song, "type") != "song" || str(song, "id") == "" || str(title, "text") == "" || sourceID("jiosaavn", str(song, "perma_url"), "track") != id {
		return nil
	}
	if action := str(title, "action"); action != "" && sourceID("jiosaavn", "https://www.jiosaavn.com"+action, "track") != id {
		return nil
	}
	primary, featured := []any{}, []any{}
	for _, value := range list(song["subtitle"]) {
		item, _ := value.(map[string]any)
		role, name := str(item, "role"), str(item, "text")
		if name == "" || (role != "primary_artists" && role != "featured_artists") {
			continue
		}
		artist := map[string]any{"name": name}
		// Only a matching structured artist row owns the native artist ID.
		for _, value := range list(song["artists"]) {
			row, _ := value.(map[string]any)
			if str(row, "name") == name && str(row, "perma_url") == "https://www.jiosaavn.com"+str(item, "action") && str(item, "action") != "" {
				if previous := scalar(artist["id"]); previous != "" && previous != scalar(row["id"]) {
					return nil
				}
				artist["id"] = row["id"]
			}
		}
		if role == "primary_artists" {
			primary = append(primary, artist)
		} else {
			featured = append(featured, artist)
		}
	}
	if len(primary) == 0 {
		return nil
	}
	names := []string{}
	for _, value := range primary {
		names = append(names, str(value.(map[string]any), "name"))
	}
	extra := map[string]any{}
	for _, key := range []string{"type", "id", "title", "album", "artists", "subtitle", "copyright", "duration", "has_lyrics", "image", "language", "play_count", "perma_url", "year"} {
		if value, ok := song[key]; ok {
			extra[key] = value
		}
	}
	result := map[string]any{
		"id": song["id"], "type": "song", "song": title["text"],
		"album": album["text"], "image": schemaImage(song["image"]),
		"perma_url": song["perma_url"], "primary_artists": strings.Join(names, ", "),
		"artistMap": map[string]any{
			"primary_artists": primary, "featured_artists": featured, "artists": song["artists"],
		},
		"copyright_text": copyright["text"], "structured_metadata": extra,
	}
	for _, key := range []string{"duration", "language", "play_count", "year"} {
		if value, ok := song[key]; ok {
			result[key] = value
		}
	}
	// Year is the only date precision reported by this page.
	if year := scalar(song["year"]); len(year) == 4 && digits.MatchString(year) {
		result["release_date"] = year
	}
	return result
}
