package artists

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func object(value any) map[string]any { m, _ := value.(map[string]any); return m }
func list(value any) []any            { a, _ := value.([]any); return a }
func text(value any) string {
	switch v := value.(type) {
	case string:
		return html.UnescapeString(v)
	case json.Number:
		return string(v)
	}
	return ""
}
func number(value any) *int64 {
	raw := strings.TrimSpace(text(value))
	if raw == "" {
		return nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 0 {
		return nil
	}
	return &n
}
func value(n *int64) int64 {
	if n == nil {
		return 0
	}
	return *n
}
func safeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return ""
	}
	return u.String()
}
func images(raw any) []Image {
	if raw == nil {
		return nil
	}
	if str, ok := raw.(string); ok {
		if u := safeURL(str); u != "" {
			return []Image{{URL: u}}
		}
		return nil
	}
	out := []Image{}
	for _, item := range list(raw) {
		m := object(item)
		if u := safeURL(text(m["url"])); u != "" {
			out = append(out, Image{URL: u, Width: value(number(m["width"])), Height: value(number(m["height"]))})
		}
	}
	return out
}
func metric(name, label, field, period, scope string, raw any) *Metric {
	display := strings.TrimSpace(text(raw))
	if display == "" || display == "null" {
		return nil
	}
	// Only full integers become exact values. Abbreviations stay as source text.
	numeric := strings.ToLower(display)
	for _, suffix := range []string{" views", " plays", " listeners", " subscribers"} {
		numeric = strings.TrimSuffix(numeric, suffix)
	}
	exact := number(strings.ReplaceAll(numeric, ",", ""))
	return &Metric{Name: name, Label: label, Value: exact, DisplayValue: display, Period: period, Scope: scope, SourceField: field}
}
func addMetric(to *[]Metric, m *Metric) {
	if m != nil {
		*to = append(*to, *m)
	}
}
func selected(m map[string]any, keys ...string) map[string]any {
	result := map[string]any{}
	for _, key := range keys {
		if v, ok := m[key]; ok && v != nil {
			result[key] = v
		}
	}
	return result
}
func endpoint(base string, params url.Values) string { return base + "?" + params.Encode() }
func saavn(params url.Values) string {
	params.Set("_format", "json")
	params.Set("_marker", "0")
	params.Set("ctx", "web6dot0")
	params.Set("api_version", "4")
	return endpoint("https://www.jiosaavn.com/api.php", params)
}
func (s *Service) storefront() string {
	if s.Storefront == "" {
		return "us"
	}
	return s.Storefront
}

func (s *Service) search(ctx context.Context, provider, name string, fresh bool) ([]Reference, error) {
	var data map[string]any
	var err error
	var rows []any
	switch provider {
	case "deezer":
		data, err = s.get(ctx, provider, endpoint("https://api.deezer.com/search/artist", url.Values{"q": {name}, "limit": {"20"}}))
		rows = list(data["data"])
	case "jiosaavn":
		data, err = s.get(ctx, provider, saavn(url.Values{"__call": {"search.getArtistResults"}, "q": {name}, "p": {"1"}, "n": {"20"}}))
		rows = list(data["results"])
	case "applemusic":
		data, err = s.get(ctx, provider, endpoint("https://itunes.apple.com/search", url.Values{"term": {name}, "entity": {"musicArtist"}, "limit": {"20"}, "country": {s.storefront()}}))
		rows = list(data["results"])
	case "musicbrainz":
		escaped := name
		for _, char := range []string{`\`, "+", "-", "!", "(", ")", "{", "}", "[", "]", "^", `"`, "~", "*", "?", ":", "/", "$", "&", "|"} {
			escaped = strings.ReplaceAll(escaped, char, `\`+char)
		}
		data, err = s.get(ctx, provider, endpoint("https://musicbrainz.org/ws/2/artist", url.Values{"query": {`artist:"` + escaped + `"`}, "fmt": {"json"}, "limit": {"20"}}))
		rows = list(data["artists"])
	case "ytmusic":
		data, err = s.call(ctx, "artist_search", name, 20, fresh)
		rows = list(data["results"])
	}
	if err != nil {
		return nil, err
	}
	if len(rows) > 20 {
		rows = rows[:20]
	}
	refs := []Reference{}
	for _, item := range rows {
		m := object(item)
		id, artistName, link := text(m["id"]), text(m["name"]), text(m["perma_url"])
		switch provider {
		case "deezer":
			link = text(m["link"])
		case "applemusic":
			id, artistName, link = text(m["artistId"]), text(m["artistName"]), text(m["artistLinkUrl"])
		case "musicbrainz":
			link = "https://musicbrainz.org/artist/" + id
		case "ytmusic":
			id, artistName = text(m["browseId"]), text(m["artist"])
			link = "https://music.youtube.com/channel/" + id
		}
		if ValidID(provider, id) && artistName != "" {
			refs = append(refs, Reference{Provider: provider, ID: id, Name: artistName, URL: safeURL(link), Disambiguation: text(m["disambiguation"])})
		}
	}
	return refs, nil
}

func (s *Service) profile(ctx context.Context, provider, id string, limit int, fresh bool) (*Profile, bool, error) {
	switch provider {
	case "deezer":
		return s.deezer(ctx, id, limit)
	case "jiosaavn":
		return s.jiosaavn(ctx, id, limit)
	case "applemusic":
		return s.apple(ctx, id, limit)
	case "musicbrainz":
		return s.musicbrainz(ctx, id, limit)
	case "ytmusic":
		return s.youtube(ctx, id, limit, fresh)
	}
	return nil, false, ErrResponse
}

func (s *Service) deezer(ctx context.Context, id string, limit int) (*Profile, bool, error) {
	raw, err := s.get(ctx, "deezer", "https://api.deezer.com/artist/"+id)
	if err != nil {
		return nil, false, err
	}
	if text(raw["id"]) != id {
		return nil, false, ErrIdentity
	}
	p := &Profile{Reference: Reference{Provider: "deezer", ID: id, Name: text(raw["name"]), URL: "https://www.deezer.com/artist/" + id}, Images: images(raw["picture_xl"]), Counts: Counts{Albums: number(raw["nb_album"]), Tracks: number(raw["nb_track"])}}
	addMetric(&p.Metrics, metric("fans", "Fans", "nb_fan", "snapshot", "artist", raw["nb_fan"]))
	top, err := s.get(ctx, "deezer", fmt.Sprintf("https://api.deezer.com/artist/%s/top?limit=%d", id, limit))
	if err == nil {
		p.TopTracks = tracks("deezer", list(top["data"]), limit)
	}
	return p, err != nil, nil
}

func (s *Service) jiosaavn(ctx context.Context, id string, limit int) (*Profile, bool, error) {
	raw, err := s.get(ctx, "jiosaavn", saavn(url.Values{"__call": {"artist.getArtistPageDetails"}, "artistId": {id}, "n_song": {strconv.Itoa(limit)}, "n_album": {strconv.Itoa(limit)}}))
	if err != nil {
		return nil, false, err
	}
	if text(raw["artistId"]) != id {
		return nil, false, ErrIdentity
	}
	p := &Profile{Reference: Reference{Provider: "jiosaavn", ID: id, Name: text(raw["name"]), URL: safeURL(text(object(raw["urls"])["overview"]))}, Images: images(raw["image"]), Counts: Counts{Tracks: number(raw["song_count"]), Albums: number(raw["album_count"])}, ProviderData: selected(raw, "subtitle", "isVerified", "dominantLanguage", "dominantType", "dob", "wiki", "availableLanguages", "urls")}
	addMetric(&p.Metrics, metric("followers", "Followers", "follower_count", "snapshot", "artist", raw["follower_count"]))
	// fan_count is shown as Listeners in the observed artist subtitle. The
	// page does not establish a month/year window for this count.
	if strings.Contains(strings.ToLower(text(raw["subtitle"])), "listeners") {
		addMetric(&p.Metrics, metric("listeners", text(raw["subtitle"]), "fan_count", "unspecified", "artist", raw["fan_count"]))
	} else {
		addMetric(&p.Metrics, metric("fans", "Fans", "fan_count", "unspecified", "artist", raw["fan_count"]))
	}
	bio := strings.TrimSpace(text(raw["bio"]))
	if strings.HasPrefix(bio, "[") {
		var sections []map[string]any
		if json.Unmarshal([]byte(bio), &sections) == nil {
			for _, section := range sections {
				if part := text(section["text"]); part != "" {
					if p.Biography != "" {
						p.Biography += "\n\n"
					}
					p.Biography += part
				}
			}
		}
	} else if bio != "" {
		p.Biography = bio
	}
	p.TopTracks = tracks("jiosaavn", list(raw["topSongs"]), limit)
	return p, false, nil
}

func (s *Service) apple(ctx context.Context, id string, limit int) (*Profile, bool, error) {
	raw, err := s.get(ctx, "applemusic", endpoint("https://itunes.apple.com/lookup", url.Values{"id": {id}, "entity": {"song"}, "limit": {strconv.Itoa(limit)}, "sort": {"recent"}, "country": {s.storefront()}}))
	if err != nil {
		return nil, false, err
	}
	var artist map[string]any
	rows := []any{}
	for _, item := range list(raw["results"]) {
		m := object(item)
		if text(m["wrapperType"]) == "artist" && text(m["artistId"]) == id {
			artist = m
		}
		if text(m["kind"]) == "song" {
			rows = append(rows, m)
		}
	}
	if artist == nil {
		return nil, false, ErrIdentity
	}
	p := &Profile{Reference: Reference{Provider: "applemusic", ID: id, Name: text(artist["artistName"]), URL: safeURL(text(artist["artistLinkUrl"]))}, ProviderData: selected(artist, "artistType", "primaryGenreName", "primaryGenreId")}
	// iTunes lookup provides recent songs, not a total or an audience statistic.
	p.RecentTracks = tracks("applemusic", rows, limit)
	return p, false, nil
}

func (s *Service) musicbrainz(ctx context.Context, id string, limit int) (*Profile, bool, error) {
	raw, err := s.get(ctx, "musicbrainz", endpoint("https://musicbrainz.org/ws/2/artist/"+id, url.Values{"fmt": {"json"}, "inc": {"aliases+tags+url-rels+annotation"}}))
	if err != nil {
		return nil, false, err
	}
	if text(raw["id"]) != id {
		return nil, false, ErrIdentity
	}
	p := &Profile{Reference: Reference{Provider: "musicbrainz", ID: id, Name: text(raw["name"]), URL: "https://musicbrainz.org/artist/" + id, Disambiguation: text(raw["disambiguation"])}, ProviderData: selected(raw, "type", "gender", "country", "area", "begin-area", "end-area", "life-span", "aliases", "tags", "relations", "annotation", "isnis", "ipis", "sort-name")}
	catalog, err := s.get(ctx, "musicbrainz", endpoint("https://musicbrainz.org/ws/2/recording", url.Values{"artist": {id}, "fmt": {"json"}, "limit": {strconv.Itoa(limit)}}))
	if err == nil {
		p.Counts.Recordings = number(catalog["recording-count"])
		p.CatalogTracks = tracks("musicbrainz", list(catalog["recordings"]), limit)
	}
	return p, err != nil, nil
}

func (s *Service) call(ctx context.Context, kind, id string, limit int, fresh bool) (map[string]any, error) {
	var raw json.RawMessage
	var err error
	if bypass, ok := s.Pool.(interface {
		CallFresh(context.Context, string, string, int) (json.RawMessage, error)
	}); fresh && ok {
		raw, err = bypass.CallFresh(ctx, kind, id, limit)
	} else {
		raw, err = s.Pool.Call(ctx, kind, id, limit)
	}
	if err != nil {
		return nil, err
	}
	var result map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if decoder.Decode(&result) != nil {
		return nil, ErrResponse
	}
	return result, nil
}

func (s *Service) youtube(ctx context.Context, id string, limit int, fresh bool) (*Profile, bool, error) {
	// The artist cache owns freshness for audience counters; the general worker
	// cache can otherwise retain a profile for six hours and relabel it as new.
	raw, err := s.call(ctx, "artist", id, limit, true)
	if err != nil {
		return nil, false, err
	}
	if text(raw["id"]) != id {
		return nil, false, ErrIdentity
	}
	m := object(raw["metadata"])
	p := &Profile{Reference: Reference{Provider: "ytmusic", ID: id, Name: text(m["name"]), URL: "https://music.youtube.com/channel/" + id}, Biography: text(m["description"]), Images: images(m["thumbnails"]), ProviderData: selected(m, "channelId", "descriptionRuns")}
	p.ObservedAt, _ = time.Parse(time.RFC3339Nano, text(raw["observed_at"]))
	addMetric(&p.Metrics, metric("subscribers", "Subscribers", "subscribers", "snapshot", "artist_channel", m["subscribers"]))
	// The pinned parser calls this monthlyListeners but strips the UI label
	// "monthly audience". Preserve the actual audience concept, not plays.
	addMetric(&p.Metrics, metric("monthly_audience", "Monthly audience", "monthlyListeners", "monthly", "artist", m["monthlyListeners"]))
	addMetric(&p.Metrics, metric("views", "Views", "views", "unspecified", "artist_channel", m["views"]))
	p.TopTracks = tracks("ytmusic", list(object(m["songs"])["results"]), limit)
	return p, raw["partial"] == true, nil
}

func tracks(provider string, rows []any, limit int) []Track {
	out := []Track{}
	seen := map[string]bool{}
	for _, item := range rows {
		if len(out) >= limit {
			break
		}
		m := object(item)
		t := Track{ID: text(m["id"]), Title: text(m["title"])}
		switch provider {
		case "deezer":
			t.URL = safeURL(text(m["link"]))
			t.Artists = []string{text(object(m["artist"])["name"])}
			t.Album = text(object(m["album"])["title"])
			t.DurationMS = value(number(m["duration"])) * 1000
			t.Images = images(object(m["album"])["cover_xl"])
		case "jiosaavn":
			if t.Title == "" {
				t.Title = text(m["song"])
			}
			t.URL = safeURL(text(m["perma_url"]))
			info := object(m["more_info"])
			t.Album = text(info["album"])
			if t.Album == "" {
				t.Album = text(m["album"])
			}
			t.DurationMS = value(number(info["duration"])) * 1000
			if t.DurationMS == 0 {
				t.DurationMS = value(number(m["duration"])) * 1000
			}
			t.Images = images(m["image"])
			for _, artist := range list(object(info["artistMap"])["primary_artists"]) {
				if n := text(object(artist)["name"]); n != "" {
					t.Artists = append(t.Artists, n)
				}
			}
			addMetric(&t.Metrics, metric("plays", "Play count", "play_count", "unspecified", "track", m["play_count"]))
		case "applemusic":
			t.ID, t.Title = text(m["trackId"]), text(m["trackName"])
			t.URL = safeURL(text(m["trackViewUrl"]))
			t.Artists = []string{text(m["artistName"])}
			t.Album = text(m["collectionName"])
			t.DurationMS = value(number(m["trackTimeMillis"]))
		case "musicbrainz":
			t.URL = "https://musicbrainz.org/recording/" + t.ID
			t.DurationMS = value(number(m["length"]))
			for _, credit := range list(m["artist-credit"]) {
				if n := text(object(credit)["name"]); n != "" {
					t.Artists = append(t.Artists, n)
				}
			}
		case "ytmusic":
			t.ID = text(m["videoId"])
			t.URL = "https://music.youtube.com/watch?v=" + t.ID
			t.Album = text(object(m["album"])["name"])
			t.DurationMS = value(number(m["duration_seconds"])) * 1000
			t.Images = images(m["thumbnails"])
			for _, artist := range list(m["artists"]) {
				if n := text(object(artist)["name"]); n != "" {
					t.Artists = append(t.Artists, n)
				}
			}
			addMetric(&t.Metrics, metric("views", "Views", "views", "unspecified", "track", m["views"]))
		}
		if t.ID != "" && t.Title != "" && !seen[t.ID] {
			out = append(out, t)
			seen[t.ID] = true
		}
	}
	return out
}
