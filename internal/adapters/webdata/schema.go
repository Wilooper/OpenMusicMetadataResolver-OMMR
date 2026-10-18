package webdata

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

var mbid = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)
var digits = regexp.MustCompile(`^[0-9]{1,18}$`)
var token = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)
var isoDuration = regexp.MustCompile(`^PT(?:(\d+(?:\.\d+)?)H)?(?:(\d+(?:\.\d+)?)M)?(?:(\d+(?:\.\d+)?)S)?$`)

func validMBID(s string) bool { return mbid.MatchString(s) }
func sourceID(provider, raw, kind string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Port() != "" {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	switch provider {
	case "deezer":
		if (u.Hostname() == "www.deezer.com" || u.Hostname() == "deezer.com") && len(parts) >= 2 && len(parts) <= 3 && parts[len(parts)-2] == kind && digits.MatchString(parts[len(parts)-1]) {
			return parts[len(parts)-1]
		}
	case "jiosaavn":
		if (u.Hostname() == "www.jiosaavn.com" || u.Hostname() == "jiosaavn.com") && kind == "track" && len(parts) == 3 && parts[0] == "song" && token.MatchString(parts[2]) {
			return parts[2]
		}
	case "musicbrainz":
		resource := kind
		if kind == "track" {
			resource = "recording"
		}
		if u.Hostname() == "musicbrainz.org" && len(parts) == 2 && parts[0] == resource && validMBID(parts[1]) {
			return parts[1]
		}
	}
	return ""
}
func resource(m map[string]any) string {
	for _, key := range []string{"url", "@id", "sameAs"} {
		if raw := str(m, key); raw != "" {
			return raw
		}
	}
	return ""
}
func schemaRecords(v any, r Request, depth int, records *[]map[string]any) {
	if depth > 12 || len(*records) >= 20 {
		return
	}
	switch m := v.(type) {
	case map[string]any:
		kind := str(m, "@type")
		for _, value := range list(m["@type"]) {
			if value == "MusicRecording" {
				kind = "MusicRecording"
			}
		}
		if kind == "MusicRecording" {
			if record := convert(m, r.Provider); record != nil {
				*records = append(*records, record)
			}
			return
		}
		for _, key := range []string{"@graph", "itemListElement", "item", "mainEntity"} {
			if value, ok := m[key]; ok {
				schemaRecords(value, r, depth+1, records)
			}
		}
	case []any:
		for _, item := range m {
			schemaRecords(item, r, depth+1, records)
		}
	}
}
func seconds(raw string) float64 {
	m := isoDuration.FindStringSubmatch(raw)
	if len(m) == 0 {
		return 0
	}
	h, _ := strconv.ParseFloat(m[1], 64)
	min, _ := strconv.ParseFloat(m[2], 64)
	s, _ := strconv.ParseFloat(m[3], 64)
	total := h*3600 + min*60 + s
	if total > 86400 {
		return 0
	}
	return total
}
func objects(v any) []map[string]any {
	if name, ok := v.(string); ok && strings.TrimSpace(name) != "" {
		return []map[string]any{{"name": name}}
	}
	if item, ok := v.(map[string]any); ok {
		return []map[string]any{item}
	}
	var result []map[string]any
	for _, item := range list(v) {
		result = append(result, objects(item)...)
	}
	return result
}
func list(v any) []any { items, _ := v.([]any); return items }
func convert(m map[string]any, provider string) map[string]any {
	id := ""
	link := ""
	for _, key := range []string{"url", "@id", "sameAs"} {
		if raw := str(m, key); raw != "" {
			if current := sourceID(provider, raw, "track"); current != "" {
				if id != "" && id != current {
					return nil
				}
				id, link = current, raw
			}
		}
	}
	artists := objects(m["byArtist"])
	if len(artists) == 0 {
		artists = objects(m["by"])
	}
	if id == "" || str(m, "name") == "" || len(artists) == 0 {
		return nil
	}
	names := []string{}
	for _, artist := range artists {
		if str(artist, "name") == "" {
			return nil
		}
		names = append(names, str(artist, "name"))
	}
	extra := map[string]any{}
	for key, value := range m {
		if key != "potentialAction" {
			extra[key] = value
		}
	}
	duration := seconds(str(m, "duration"))
	album, _ := m["inAlbum"].(map[string]any)
	switch provider {
	case "deezer":
		artist := map[string]any{"name": names[0]}
		if a := sourceID(provider, resource(artists[0]), "artist"); a != "" {
			artist["id"] = json.Number(a)
		}
		result := map[string]any{"id": json.Number(id), "title": str(m, "name"), "artist": artist, "link": link, "structured_metadata": extra}
		contributors := []any{}
		for _, a := range artists {
			item := map[string]any{"name": str(a, "name")}
			if aid := sourceID(provider, resource(a), "artist"); aid != "" {
				item["id"] = json.Number(aid)
			}
			contributors = append(contributors, item)
		}
		result["contributors"] = contributors
		a := map[string]any{"title": str(album, "name"), "cover_xl": schemaImage(m["image"])}
		if aid := sourceID(provider, resource(album), "album"); aid != "" {
			a["id"] = json.Number(aid)
		}
		result["album"] = a
		if duration > 0 {
			result["duration"] = int64(duration)
		}
		if value := str(m, "datePublished"); value != "" {
			result["release_date"] = value
		}
		if value := str(m, "isrcCode"); value != "" {
			result["isrc"] = value
		}
		return result
	case "jiosaavn":
		result := map[string]any{"song": str(m, "name"), "primary_artists": strings.Join(names, ", "), "album": str(album, "name"), "perma_url": link, "image": schemaImage(m["image"]), "structured_metadata": extra}
		if duration > 0 {
			result["duration"] = strconv.FormatInt(int64(duration), 10)
		}
		if value := str(m, "datePublished"); value != "" {
			result["release_date"] = value
		}
		if value := str(m, "inLanguage"); value != "" {
			result["language"] = value
		}
		return result
	case "musicbrainz":
		credits := []any{}
		for _, a := range artists {
			item := map[string]any{"name": str(a, "name")}
			if aid := sourceID(provider, resource(a), "artist"); aid != "" {
				item["id"] = aid
			}
			credits = append(credits, map[string]any{"name": str(a, "name"), "artist": item})
		}
		result := map[string]any{"id": id, "title": str(m, "name"), "artist-credit": credits, "structured_metadata": extra}
		if duration > 0 {
			result["length"] = int64(duration * 1000)
		}
		if value := str(m, "isrcCode"); value != "" {
			result["isrcs"] = []string{value}
		}
		return result
	}
	return nil
}
func musicBrainzRows(doc *html.Node) []map[string]any {
	result := []map[string]any{}
	walk(doc, func(row *html.Node) {
		if row.Data != "tr" || len(result) >= 10 {
			return
		}
		cells := []*html.Node{}
		for child := row.FirstChild; child != nil; child = child.NextSibling {
			if child.Data == "td" {
				cells = append(cells, child)
			}
		}
		if len(cells) < 2 {
			return
		}
		// Only the recording-name cell owns the row, never release/related links.
		index := -1
		var recording *html.Node
		for i, cell := range cells {
			if i > 1 {
				break
			}
			recording = first(cell, func(n *html.Node) bool { return n.Data == "a" && strings.HasPrefix(attr(n, "href"), "/recording/") })
			if recording != nil {
				index = i
				break
			}
		}
		if index < 0 || index+1 >= len(cells) {
			return
		}
		id := strings.TrimPrefix(attr(recording, "href"), "/recording/")
		if !validMBID(id) {
			return
		}
		credits := []any{}
		walk(cells[index+1], func(a *html.Node) {
			if a.Data != "a" || !strings.HasPrefix(attr(a, "href"), "/artist/") {
				return
			}
			aid := strings.TrimPrefix(attr(a, "href"), "/artist/")
			if validMBID(aid) && text(a) != "" {
				credits = append(credits, map[string]any{"name": text(a), "artist": map[string]string{"id": aid, "name": text(a)}})
			}
		})
		if text(recording) != "" && len(credits) > 0 {
			result = append(result, map[string]any{"id": id, "title": text(recording), "artist-credit": credits})
		}
	})
	return result
}

func schemaImage(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case map[string]any:
		if raw := str(v, "url"); raw != "" {
			return raw
		}
		return str(v, "contentUrl")
	case []any:
		for _, item := range v {
			if raw := schemaImage(item); raw != "" {
				return raw
			}
		}
	}
	return ""
}
