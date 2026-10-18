package webdata

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/net/html"
)

var ErrUnavailable = errors.New("provider returned an unavailable HTML page without catalog data")
var ErrNoData = errors.New("provider HTML contains no recognized catalog data")

const maxBody = 8 << 20

type Request struct{ Provider, Mode, ID string }

func Read(resp *http.Response, request Request) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, errors.New("provider response could not be read")
	}
	if len(body) > maxBody {
		return nil, errors.New("provider catalog response exceeds size limit")
	}
	body = bytes.TrimPrefix(body, []byte{0xef, 0xbb, 0xbf})
	if json.Valid(body) {
		var envelope map[string]json.RawMessage
		if json.Unmarshal(body, &envelope) == nil {
			for _, key := range []string{"error", "errors"} {
				if value, ok := envelope[key]; ok && hasError(value) {
					return nil, errors.New("provider catalog returned an API error")
				}
			}
		}
		return body, nil
	}
	return DecodeHTML(body, request)
}
func DecodeHTML(body []byte, request Request) ([]byte, error) {
	if len(body) > maxBody {
		return nil, errors.New("provider HTML exceeds size limit")
	}
	if request.Mode != "search" && request.Mode != "track" {
		return nil, ErrNoData
	}
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, ErrNoData
	}
	values := []any{}
	scripts := 0
	walk(doc, func(n *html.Node) {
		if n.Data != "script" || scripts >= 64 {
			return
		}
		scripts++
		var b strings.Builder
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			b.WriteString(c.Data)
		}
		raw := strings.TrimSpace(b.String())
		kind := strings.ToLower(strings.Split(attr(n, "type"), ";")[0])
		if kind == "application/ld+json" || kind == "application/json" || attr(n, "id") == "__NEXT_DATA__" {
			if v, ok := decode(raw); ok {
				values = append(values, v)
			}
			return
		}
		for _, marker := range []string{"window.__INITIAL_STATE__", "window.__INITIAL_DATA__", "window.__DZR_APP_STATE__", "__DZR_APP_STATE__"} {
			index := strings.Index(raw, marker)
			if index < 0 {
				continue
			}
			rest := strings.TrimSpace(raw[index+len(marker):])
			if !strings.HasPrefix(rest, "=") {
				continue
			}
			if v, ok := decode(strings.TrimSpace(rest[1:])); ok {
				values = append(values, v)
			}
			break
		}
	})
	// A sole preformatted JSON body is common in HTML-wrapped API responses.
	pre := first(doc, func(n *html.Node) bool { return n.Data == "pre" })
	pageBody := first(doc, func(n *html.Node) bool { return n.Data == "body" })
	if pre != nil && text(pre) == text(pageBody) && first(doc, func(n *html.Node) bool { return n.Data == "h1" }) == nil {
		if value, ok := decode(rawText(pre)); ok {
			values = append(values, value)
		}
	}
	// Recover only known catalog envelopes; arbitrary embedded objects are not songs.
	native := []any{}
	for _, value := range values {
		envelopes(value, request, 0, &native)
	}
	if len(native) == 1 {
		if request.Provider == "jiosaavn" && request.Mode == "track" {
			if m, ok := native[0].(map[string]any); ok && nativeRecord(m, request.Provider) {
				return json.Marshal(map[string]any{request.ID: m})
			}
		}
		return json.Marshal(native[0])
	}
	if len(native) > 1 {
		return nil, errors.New("ambiguous embedded catalog envelopes")
	}
	records := []map[string]any{}
	for _, value := range values {
		schemaRecords(value, request, 0, &records)
	}
	if len(records) == 0 && request.Provider == "musicbrainz" && request.Mode == "search" {
		records = musicBrainzRows(doc)
	}
	if len(records) > 0 {
		return wrap(records, request)
	}
	heading := first(doc, func(n *html.Node) bool { return n.Data == "h1" })
	title := first(doc, func(n *html.Node) bool { return n.Data == "title" })
	message := strings.ToLower(text(heading) + " " + text(title))
	for _, label := range []string{"site unavailable", "access denied", "service unavailable", "verify you are human"} {
		if strings.Contains(message, label) {
			return nil, ErrUnavailable
		}
	}
	return nil, ErrNoData
}
func decode(s string) (any, bool) {
	var value any
	d := json.NewDecoder(strings.NewReader(s))
	d.UseNumber()
	err := d.Decode(&value)
	return value, err == nil
}
func envelopes(v any, r Request, depth int, result *[]any) {
	if depth > 12 || len(*result) > 1 {
		return
	}
	switch m := v.(type) {
	case map[string]any:
		valid := false
		if r.Mode == "search" {
			switch r.Provider {
			case "deezer":
				_, valid = m["data"].([]any)
			case "musicbrainz":
				_, valid = m["recordings"].([]any)
			case "jiosaavn":
				_, valid = m["results"].([]any)
			}
		} else {
			valid = nativeRecord(m, r.Provider) && recordID(m, r.Provider) == r.ID
			if r.Provider == "jiosaavn" {
				if child, ok := m[r.ID].(map[string]any); ok && nativeRecord(child, r.Provider) && recordID(child, r.Provider) == r.ID {
					valid = true
				}
			}
		}
		if valid && validEnvelope(m, r) {
			*result = append(*result, m)
			return
		}
		// Only recognized app-state containers; never walk recommendations or arbitrary JSON.
		for _, key := range []string{"props", "pageProps", "initialState", "state", "search", "songs", "song", "track", "recording", "payload", "data", "pageData", "initialData"} {
			if child, ok := m[key]; ok {
				envelopes(child, r, depth+1, result)
			}
		}
	}
}
func nativeRecord(m map[string]any, provider string) bool {
	switch provider {
	case "deezer":
		artist, _ := m["artist"].(map[string]any)
		return scalar(m["id"]) != "" && str(m, "title") != "" && str(artist, "name") != ""
	case "jiosaavn":
		if str(m, "id") == "" {
			return false
		}
		if str(m, "song") != "" && (str(m, "primary_artists") != "" || str(m, "singers") != "") {
			return true
		}
		info, _ := m["more_info"].(map[string]any)
		artistMap, _ := info["artistMap"].(map[string]any)
		primary, _ := artistMap["primary_artists"].([]any)
		for _, value := range primary {
			artist, _ := value.(map[string]any)
			if str(m, "title") != "" && str(artist, "name") != "" {
				return true
			}
		}
		return false
	case "musicbrainz":
		credits, _ := m["artist-credit"].([]any)
		return validMBID(str(m, "id")) && str(m, "title") != "" && len(credits) > 0
	}
	return false
}
func recordID(m map[string]any, provider string) string { return scalar(m["id"]) }
func validEnvelope(m map[string]any, r Request) bool {
	if r.Mode == "track" {
		return true
	}
	key := map[string]string{"deezer": "data", "jiosaavn": "results", "musicbrainz": "recordings"}[r.Provider]
	entries, _ := m[key].([]any)
	// Empty API envelopes inside HTML cannot prove a genuine empty catalog page.
	if len(entries) == 0 {
		return false
	}
	for _, entry := range entries {
		record, ok := entry.(map[string]any)
		if !ok || !nativeRecord(record, r.Provider) {
			return false
		}
	}
	return true
}
func wrap(records []map[string]any, r Request) ([]byte, error) {
	unique := []map[string]any{}
	seen := map[string]bool{}
	for _, record := range records {
		key := recordID(record, r.Provider)
		if key == "" {
			key = str(record, "perma_url")
		}
		if key != "" && !seen[key] {
			seen[key] = true
			unique = append(unique, record)
		}
	}
	if r.Mode == "track" {
		matches := []map[string]any{}
		for _, record := range unique {
			if recordID(record, r.Provider) == r.ID || (r.Provider == "jiosaavn" && sourceID(r.Provider, str(record, "perma_url"), "track") == r.ID) {
				matches = append(matches, record)
			}
		}
		if len(matches) != 1 {
			return nil, errors.New("HTML recording identity missing or ambiguous")
		}
		if r.Provider == "jiosaavn" {
			return json.Marshal(map[string]any{r.ID: matches[0]})
		}
		return json.Marshal(matches[0])
	}
	if len(unique) > 10 {
		unique = unique[:10]
	}
	key := map[string]string{"deezer": "data", "jiosaavn": "results", "musicbrainz": "recordings"}[r.Provider]
	if key == "" {
		return nil, fmt.Errorf("unsupported HTML provider")
	}
	return json.Marshal(map[string]any{key: unique})
}
func attr(n *html.Node, key string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func walk(n *html.Node, fn func(*html.Node)) {
	if n == nil {
		return
	}
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}
func first(n *html.Node, fn func(*html.Node) bool) *html.Node {
	if n == nil {
		return nil
	}
	if fn(n) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if result := first(c, fn); result != nil {
			return result
		}
	}
	return nil
}
func text(n *html.Node) string {
	var s strings.Builder
	walk(n, func(v *html.Node) {
		if v.Type == html.TextNode {
			s.WriteString(v.Data)
			s.WriteByte(' ')
		}
	})
	return strings.Join(strings.Fields(s.String()), " ")
}
func scalar(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return string(x)
	}
	return ""
}
func str(m map[string]any, key string) string { s, _ := m[key].(string); return s }

func hasError(value json.RawMessage) bool {
	switch string(bytes.TrimSpace(value)) {
	case "null", "false", "0", `""`, "[]":
		return false
	default:
		return true
	}
}
func rawText(n *html.Node) string {
	var b strings.Builder
	walk(n, func(v *html.Node) {
		if v.Type == html.TextNode {
			b.WriteString(v.Data)
		}
	})
	return b.String()
}
