package albums

import (
	"context"
	"encoding/json"
	"html"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/ommr/ommr/internal/metadata"
	"github.com/ommr/ommr/internal/models/provider"
)

func obj(v any) map[string]any { m, _ := v.(map[string]any); return m }
func arr(v any) []any          { a, _ := v.([]any); return a }
func text(v any) string {
	switch v := v.(type) {
	case string:
		return html.UnescapeString(v)
	case json.Number:
		return string(v)
	}
	return ""
}
func number(v any) int64 { n, _ := strconv.ParseInt(text(v), 10, 64); return n }
func count(v any) *int {
	if text(v) == "" {
		return nil
	}
	n, e := strconv.Atoi(text(v))
	if e != nil || n < 0 {
		return nil
	}
	return &n
}
func first(values ...any) string {
	for _, v := range values {
		if t := text(v); t != "" {
			return t
		}
	}
	return ""
}
func publicURL(v any) string {
	u, e := url.Parse(text(v))
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return ""
	}
	return u.String()
}
func catalog(m map[string]any, omit ...string) map[string]any {
	c := map[string]any{}
	for k, v := range m {
		c[k] = v
	}
	for _, k := range omit {
		delete(c, k)
	}
	return metadata.Catalog(c)
}
func basic(m map[string]any, q Query) *Album {
	return &Album{Provider: q.Provider, ID: text(m["id"]), Title: first(m["title"], m["name"]), URL: publicURL(m["link"]), Tracks: []Track{}, TrackList: TrackList{Reason: "unknown_total"}, ProviderData: catalog(m, "tracks", "songs", "list", "media", "results")}
}
func finish(a *Album, q Query, valid bool) {
	a.TrackList.Returned = len(a.Tracks)
	if valid && a.TrackList.Total != nil && *a.TrackList.Total == len(a.Tracks) {
		a.TrackList.Complete = true
		a.TrackList.Reason = ""
		return
	}
	if len(a.Tracks) >= q.Limit && (a.TrackList.Total == nil || *a.TrackList.Total > len(a.Tracks)) {
		a.TrackList.Reason = "limit"
	} else if a.TrackList.Total != nil {
		a.TrackList.Reason = "incomplete_response"
	}
}
func (s *Service) fetch(ctx context.Context, q Query) (*Album, error) {
	switch q.Provider {
	case "applemusic":
		return s.apple(ctx, q)
	case "deezer":
		return s.deezer(ctx, q)
	case "jiosaavn":
		return s.jio(ctx, q)
	case "musicbrainz":
		return s.brainz(ctx, q)
	}
	return nil, ErrInput
}
func (s *Service) apple(ctx context.Context, q Query) (*Album, error) {
	country := s.Storefront
	if country == "" {
		country = "us"
	}
	params := url.Values{"id": {q.ID}, "entity": {"song"}, "limit": {"200"}, "country": {country}}
	m, err := s.get(ctx, q.Provider, "https://itunes.apple.com/lookup?"+params.Encode())
	if err != nil {
		return nil, err
	}
	rows := arr(m["results"])
	var root map[string]any
	for _, v := range rows {
		r := obj(v)
		if text(r["wrapperType"]) == "collection" && text(r["collectionId"]) == q.ID {
			root = r
			break
		}
	}
	if root == nil {
		if len(rows) == 0 {
			return nil, ErrNotFound
		}
		return nil, ErrResponse
	}
	a := basic(root, q)
	a.ID = q.ID
	a.Title = text(root["collectionName"])
	a.URL = publicURL(root["collectionViewUrl"])
	a.ReleaseDate = text(root["releaseDate"])
	a.Copyright = text(root["copyright"])
	a.Artists = []Artist{{ID: text(root["artistId"]), Name: text(root["artistName"])}}
	a.TrackList.Total = count(root["trackCount"])
	if image := publicURL(root["artworkUrl100"]); image != "" {
		a.Images = []string{image}
	}
	valid := true
	seen := map[string]bool{}
	discs := map[int]Disc{}
	for _, v := range rows {
		r := obj(v)
		if text(r["wrapperType"]) != "track" {
			continue
		}
		if text(r["collectionId"]) != q.ID {
			valid = false
			continue
		}
		t := Track{ID: text(r["trackId"]), Title: text(r["trackName"]), Position: int(number(r["trackNumber"])), DiscNumber: int(number(r["discNumber"])), TrackNumber: text(r["trackNumber"]), DurationMS: number(r["trackTimeMillis"]), Artists: []Artist{{ID: text(r["artistId"]), Name: text(r["artistName"])}}, ProviderData: catalog(r)}
		key := strconv.Itoa(t.DiscNumber) + "/" + t.TrackNumber
		if t.ID == "" || t.Title == "" || t.Position < 1 || t.DiscNumber < 1 || seen[key] {
			valid = false
			continue
		}
		seen[key] = true
		declared := count(r["trackCount"])
		if previous, ok := discs[t.DiscNumber]; ok && previous.Total != nil && declared != nil && *previous.Total != *declared {
			valid = false
		}
		discs[t.DiscNumber] = Disc{Number: t.DiscNumber, Total: declared}
		a.Tracks = append(a.Tracks, t)
	}
	for _, disc := range discs {
		a.Discs = append(a.Discs, disc)
	}
	sort.Slice(a.Discs, func(i, j int) bool { return a.Discs[i].Number < a.Discs[j].Number })
	sort.SliceStable(a.Tracks, func(i, j int) bool {
		if a.Tracks[i].DiscNumber != a.Tracks[j].DiscNumber {
			return a.Tracks[i].DiscNumber < a.Tracks[j].DiscNumber
		}
		return a.Tracks[i].Position < a.Tracks[j].Position
	})
	if len(a.Tracks) > q.Limit {
		a.Tracks = a.Tracks[:q.Limit]
	}
	finish(a, q, valid)
	return a, nil
}
func (s *Service) deezer(ctx context.Context, q Query) (*Album, error) {
	m, err := s.get(ctx, q.Provider, "https://api.deezer.com/album/"+q.ID)
	if err != nil {
		return nil, err
	}
	a := basic(m, q)
	if a.ID != q.ID || a.Title == "" {
		return nil, ErrResponse
	}
	a.ReleaseDate = text(m["release_date"])
	a.Label = text(m["label"])
	a.UPC = text(m["upc"])
	a.TrackList.Total = count(m["nb_tracks"])
	a.Artists = []Artist{{ID: text(obj(m["artist"])["id"]), Name: text(obj(m["artist"])["name"])}}
	if image := publicURL(m["cover_xl"]); image != "" {
		a.Images = []string{image}
	}
	valid := true
	offset := 0
	seenPages := map[string]bool{}
	seenEntries := map[string]bool{}
	// Reconstruct every page on the fixed host; never follow provider-supplied next URLs.
	for page := 0; page < 20 && len(a.Tracks) < q.Limit; page++ {
		n := min(100, q.Limit-len(a.Tracks))
		endpoint := "https://api.deezer.com/album/" + q.ID + "/tracks?limit=" + strconv.Itoa(n) + "&index=" + strconv.Itoa(offset)
		p, e := s.get(ctx, q.Provider, endpoint)
		if e != nil {
			return a, e
		}
		rows := arr(p["data"])
		if len(rows) > n {
			return a, ErrResponse
		}
		fingerprint, _ := json.Marshal(rows)
		if len(rows) > 0 && seenPages[string(fingerprint)] {
			return a, ErrResponse
		}
		seenPages[string(fingerprint)] = true
		for rowIndex, v := range rows {
			r := obj(v)
			if parent := text(obj(r["album"])["id"]); parent != "" && parent != q.ID {
				return a, ErrResponse
			}
			t := Track{ID: text(r["id"]), Title: text(r["title"]), Position: int(number(r["track_position"])), DiscNumber: int(number(r["disk_number"])), TrackNumber: text(r["track_position"]), DurationMS: number(r["duration"]) * 1000, ISRC: text(r["isrc"]), Artists: []Artist{{ID: text(obj(r["artist"])["id"]), Name: text(obj(r["artist"])["name"])}}, ProviderData: catalog(r)}
			if t.ID == "" || t.Title == "" {
				valid = false
				continue
			}
			entry := t.ID
			if t.Position > 0 && t.DiscNumber > 0 {
				entry += "/" + strconv.Itoa(t.DiscNumber) + "/" + strconv.Itoa(t.Position)
			}
			if seenEntries[entry] {
				valid = false
				continue
			}
			seenEntries[entry] = true
			if t.Position == 0 {
				t.Position = offset + rowIndex + 1
			}
			a.Tracks = append(a.Tracks, t)
		}
		offset += len(rows)
		if len(rows) == 0 {
			break
		}
		// The root's nb_tracks is the album total, not a page's result count.
		if a.TrackList.Total != nil && offset >= *a.TrackList.Total {
			break
		}
		if text(p["next"]) == "" && (a.TrackList.Total == nil || offset >= *a.TrackList.Total) {
			break
		}
	}
	finish(a, q, valid)
	return a, nil
}
func jioArtists(m map[string]any) []Artist {
	info := obj(m["more_info"])
	artistMap := obj(info["artistMap"])
	if artistMap == nil {
		artistMap = obj(m["artistMap"])
	}
	out := []Artist{}
	for _, v := range arr(artistMap["primary_artists"]) {
		r := obj(v)
		if text(r["name"]) != "" {
			out = append(out, Artist{ID: text(r["id"]), Name: text(r["name"]), Role: text(r["role"])})
		}
	}
	if len(out) == 0 {
		for _, name := range strings.Split(text(m["primary_artists"]), ",") {
			if strings.TrimSpace(name) != "" {
				out = append(out, Artist{Name: strings.TrimSpace(name)})
			}
		}
	}
	return out
}
func (s *Service) jio(ctx context.Context, q Query) (*Album, error) {
	params := url.Values{"__call": {"content.getAlbumDetails"}, "albumid": {q.ID}, "api_version": {"4"}, "_format": {"json"}, "_marker": {"0"}, "ctx": {"web6dot0"}}
	m, err := s.get(ctx, q.Provider, "https://www.jiosaavn.com/api.php?"+params.Encode())
	if err != nil {
		return nil, err
	}
	a := basic(m, q)
	if a.ID != q.ID || a.Title == "" {
		return nil, ErrResponse
	}
	info := obj(m["more_info"])
	a.URL = publicURL(m["perma_url"])
	a.Artists = jioArtists(m)
	a.ReleaseDate = first(info["release_date"], m["release_date"], m["year"])
	a.Label = first(info["label"], m["label"])
	a.Language = text(m["language"])
	a.Copyright = first(info["copyright_text"], m["copyright_text"])
	if image := publicURL(m["image"]); image != "" {
		a.Images = []string{image}
	}
	for _, v := range []any{m["list_count"], info["song_count"], m["song_count"]} {
		if a.TrackList.Total == nil {
			a.TrackList.Total = count(v)
		}
	}
	rows := arr(m["list"])
	if rows == nil {
		rows = arr(m["songs"])
	}
	valid := true
	seen := map[string]bool{}
	for i, v := range rows {
		if len(a.Tracks) >= q.Limit {
			break
		}
		r := obj(v)
		b, _ := json.Marshal(r)
		var song provider.JioSaavnSongResult
		if json.Unmarshal(b, &song) != nil || song.ID == "" || song.Song == "" || seen[song.ID] {
			valid = false
			continue
		}
		seen[song.ID] = true
		if song.AlbumID != "" && string(song.AlbumID) != q.ID {
			return a, ErrResponse
		}
		t := Track{ID: song.ID, Title: song.Song, Position: i + 1, TrackNumber: strconv.Itoa(i + 1), DurationMS: int64(song.DurationSeconds()) * 1000, Artists: jioArtists(r), ProviderData: catalog(r)}
		a.Tracks = append(a.Tracks, t)
	}
	finish(a, q, valid)
	return a, nil
}
func brainzArtists(v any) []Artist {
	out := []Artist{}
	for _, v := range arr(v) {
		r := obj(v)
		artist := obj(r["artist"])
		if name := first(r["name"], artist["name"]); name != "" {
			out = append(out, Artist{ID: text(artist["id"]), Name: name, JoinPhrase: text(r["joinphrase"])})
		}
	}
	return out
}
func (s *Service) brainz(ctx context.Context, q Query) (*Album, error) {
	params := url.Values{"fmt": {"json"}, "inc": {"recordings+artist-credits+labels+release-groups+genres+isrcs+url-rels"}}
	m, err := s.get(ctx, q.Provider, "https://musicbrainz.org/ws/2/release/"+q.ID+"?"+params.Encode())
	if err != nil {
		return nil, err
	}
	a := basic(m, q)
	if a.ID != q.ID || a.Title == "" {
		return nil, ErrResponse
	}
	a.URL = "https://musicbrainz.org/release/" + q.ID
	a.ReleaseDate = text(m["date"])
	a.UPC = text(m["barcode"])
	a.Language = text(obj(m["text-representation"])["language"])
	a.Artists = brainzArtists(m["artist-credit"])
	media := arr(m["media"])
	valid := len(media) > 0
	total := 0
	known := true
	seen := map[string]bool{}
	slots := map[string]bool{}
	discNumbers := map[int]bool{}
	for _, v := range media {
		disc := obj(v)
		d := Disc{Number: int(number(disc["position"])), Title: text(disc["title"]), Format: text(disc["format"]), Total: count(disc["track-count"]), ProviderData: catalog(disc, "tracks")}
		a.Discs = append(a.Discs, d)
		rows := arr(disc["tracks"])
		if d.Total == nil {
			known = false
		} else {
			total += *d.Total
			if len(rows) != *d.Total {
				valid = false
			}
		}
		if d.Number < 1 || discNumbers[d.Number] {
			valid = false
		}
		discNumbers[d.Number] = true
		for _, v := range rows {
			r := obj(v)
			recording := obj(r["recording"])
			t := Track{ID: text(r["id"]), RecordingID: text(recording["id"]), Title: first(r["title"], recording["title"]), Position: int(number(r["position"])), DiscNumber: d.Number, TrackNumber: text(r["number"]), DurationMS: number(r["length"]), Artists: brainzArtists(r["artist-credit"]), ProviderData: catalog(r)}
			if t.DurationMS == 0 {
				t.DurationMS = number(recording["length"])
			}
			if len(t.Artists) == 0 {
				t.Artists = brainzArtists(recording["artist-credit"])
			}
			isrcs := arr(recording["isrcs"])
			if len(isrcs) == 1 {
				t.ISRC = text(isrcs[0])
			}
			if t.ID == "" || t.Title == "" || t.Position < 1 || seen[t.ID] {
				valid = false
				continue
			}
			seen[t.ID] = true
			slot := strconv.Itoa(d.Number) + "/" + strconv.Itoa(t.Position)
			if slots[slot] {
				valid = false
			}
			slots[slot] = true
			if len(a.Tracks) < q.Limit {
				a.Tracks = append(a.Tracks, t)
			}
		}
	}
	if known && len(media) > 0 {
		a.TrackList.Total = &total
	}
	finish(a, q, valid)
	return a, nil
}
