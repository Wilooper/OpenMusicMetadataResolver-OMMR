package korean

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/models/provider"
	"golang.org/x/net/html"
)

type Provider struct {
	name, base string
	client     *http.Client
}

var numeric = regexp.MustCompile(`^[0-9]{1,20}$`)

func New(name string) *Provider {
	bases := map[string]string{"bugs": "https://music.bugs.co.kr", "genie": "https://www.genie.co.kr", "melon": "https://www.melon.com"}
	return &Provider{name, bases[name], &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (p *Provider) Name() string    { return p.name }
func (p *Provider) Version() string { return p.name + "-public-v1" }
func (p *Provider) endpoint(kind, id string) string {
	switch p.name {
	case "bugs":
		return "/" + kind + "/" + id
	case "genie":
		return map[string]string{"track": "/detail/songInfo?xgnm=", "album": "/detail/albumInfo?axnm=", "artist": "/detail/artistInfo?xxnm="}[kind] + id
	case "melon":
		return map[string]string{"track": "/song/detail.htm?songId=", "album": "/album/detail.htm?albumId=", "artist": "/artist/detail.htm?artistId="}[kind] + id
	}
	return ""
}
func (p *Provider) get(ctx context.Context, path string) (*html.Node, []byte, error) {
	if p.base == "" {
		return nil, nil, errors.New("unknown Korean catalog")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", p.base+path, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", "OMMR/1.0 (Open Music Metadata Resolver)")
	req.Header.Set("Accept", "text/html")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, nil, errors.New("catalog network request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, nil, fmt.Errorf("catalog HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20+1))
	if err != nil || len(body) > 4<<20 {
		return nil, nil, errors.New("catalog response limit")
	}
	doc, err := html.Parse(strings.NewReader(string(body)))
	return doc, body, err
}
func (p *Provider) Search(ctx context.Context, q adapters.Query) ([]canonical.TrackCandidate, error) {
	terms := strings.TrimSpace(q.Artist + " " + q.Title)
	if terms == "" {
		return nil, nil
	}
	path := map[string]string{"bugs": "/search/track?q=", "genie": "/search/searchMain?query=", "melon": "/search/song/index.htm?q="}[p.name] + url.QueryEscape(terms)
	doc, body, err := p.get(ctx, path)
	if err != nil {
		return nil, err
	}
	var result []canonical.TrackCandidate
	seen := map[string]bool{}
	walk(doc, func(n *html.Node) {
		if n.Type != html.ElementNode || n.Data != "tr" || len(result) >= 10 {
			return
		}
		record := p.row(n)
		if record.ID != "" && record.Title != "" && len(record.Artists) > 0 && !seen[record.ID] {
			seen[record.ID] = true
			result = append(result, p.candidate(record))
		}
	})
	if len(result) == 0 && (strings.Contains(string(body), "검색된 결과가 없습니다") || strings.Contains(string(body), "결과가 없습니다") || strings.Contains(string(body), `class="no-result"`)) {
		return result, nil
	}
	if len(result) == 0 && !strings.Contains(string(body), "검색결과가 없습니다") && !strings.Contains(string(body), "검색 결과가 없습니다") && !strings.Contains(string(body), "검색된 곡이 없습니다") {
		return nil, errors.New("unrecognized catalog search response")
	}
	return result, nil
}
func (p *Provider) FetchByID(ctx context.Context, kind, id string) (*canonical.TrackCandidate, error) {
	if kind != p.name && kind != p.name+"_id" {
		return nil, errors.New("unsupported catalog ID type")
	}
	if !numeric.MatchString(id) {
		return nil, errors.New("numeric catalog ID required")
	}
	doc, body, err := p.get(ctx, p.endpoint("track", id))
	if err != nil {
		return nil, err
	}
	record, err := p.detail(doc, body, id)
	if err != nil {
		return nil, err
	}
	candidate := p.candidate(record)
	return &candidate, nil
}
func (p *Provider) EnrichCandidate(ctx context.Context, c canonical.TrackCandidate) (*canonical.TrackCandidate, error) {
	if c.Track.Extensions["korean_detail"] != nil {
		return nil, nil
	}
	return p.FetchByID(ctx, p.name, c.Track.IDs[p.name])
}
func (p *Provider) candidate(r provider.KoreanRecord) canonical.TrackCandidate {
	t := canonical.Track{Language: str(r.Structured, "inLanguage"), Title: r.Title, DurationMS: r.DurationMS, ReleaseDate: r.ReleaseDate, ISRC: r.ISRC, IDs: map[string]string{p.name: r.ID, p.name + "_url": p.base + p.endpoint("track", r.ID)}, Album: canonical.Album{Title: r.Album, IDs: map[string]string{}}, Extensions: map[string]json.RawMessage{}}
	if r.AlbumID != "" {
		t.Album.IDs[p.name] = r.AlbumID
	}
	for _, a := range r.Artists {
		ids := map[string]string{}
		if a.ID != "" {
			ids[p.name] = a.ID
		}
		t.Artists = append(t.Artists, canonical.Artist{Name: a.Name, Role: "main", IDs: ids})
	}
	if r.Image != "" {
		t.Images = []canonical.Image{{URL: r.Image}}
	}
	for label, value := range r.Fields {
		switch label {
		case "장르":
			t.Genres = []string{value}
		case "발매일", "발매":
			if t.ReleaseDate == "" {
				t.ReleaseDate = strings.ReplaceAll(value, ".", "-")
			}
		case "기획사", "제작사":
			t.Label = value
		case "작사", "작사가", "작곡", "작곡가", "편곡":
			role := map[string]string{"작사": "lyricist", "작사가": "lyricist", "작곡": "composer", "작곡가": "composer", "편곡": "arranger"}[label]
			for _, name := range strings.Split(value, ",") {
				if name = strings.TrimSpace(name); name != "" {
					t.Credits = append(t.Credits, canonical.Credit{Name: name, Roles: []string{role}})
				}
			}
		}
	}
	if r.Fields != nil {
		t.Extensions["korean_detail"] = json.RawMessage(`true`)
	}
	raw, _ := json.Marshal(r)
	return canonical.TrackCandidate{Provider: p.name, Track: t, RawResponse: raw}
}
func attr(n *html.Node, key string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}
func class(n *html.Node, name string) bool {
	for _, v := range strings.Fields(attr(n, "class")) {
		if v == name {
			return true
		}
	}
	return false
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
func first(n *html.Node, test func(*html.Node) bool) *html.Node {
	if n == nil {
		return nil
	}
	if test(n) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := first(c, test); found != nil {
			return found
		}
	}
	return nil
}
func byClass(n *html.Node, name string) *html.Node {
	return first(n, func(v *html.Node) bool { return class(v, name) })
}
func text(n *html.Node) string {
	var b strings.Builder
	var visit func(*html.Node)
	visit = func(v *html.Node) {
		if v == nil || v.Data == "script" || v.Data == "style" || class(v, "none") || class(v, "icon") || class(v, "icon_song") || class(v, "hide") {
			return
		}
		if v.Type == html.TextNode {
			b.WriteString(v.Data)
			b.WriteByte(' ')
		}
		for c := v.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

var patterns sync.Map

func capture(pattern, value string) string {
	cached, ok := patterns.Load(pattern)
	if !ok {
		cached, _ = patterns.LoadOrStore(pattern, regexp.MustCompile(pattern))
	}
	m := cached.(*regexp.Regexp).FindStringSubmatch(value)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}
func reference(n *html.Node, kind string) string {
	value := attr(n, "href") + " " + attr(n, "onclick")
	patterns := map[string]string{"track": `(?:/track/|songId=|goSongDetail\(['"]?|fnViewSongInfo\(['"]?)([0-9]+)`, "album": `(?:/album/|albumId=|goAlbumDetail\(['"]?|fnViewAlbumLayer\(['"]?|albumInfo['"]\s*,\s*['"]|axnm=)([0-9]+)`, "artist": `(?:/artist/|artistId=|goArtistDetail\(['"]?|fnViewArtist\(['"]?|artistInfo['"]\s*,\s*['"]|xxnm=)([0-9]+)`}
	return capture(patterns[kind], value)
}
func artists(n *html.Node) []provider.KoreanArtist {
	var result []provider.KoreanArtist
	seen := map[string]bool{}
	walk(n, func(v *html.Node) {
		if v.Data != "a" {
			return
		}
		id := reference(v, "artist")
		name := text(v)
		if id != "" && name != "" && !seen[id] {
			seen[id] = true
			result = append(result, provider.KoreanArtist{ID: id, Name: name})
		}
	})
	return result
}
func image(n *html.Node) string {
	img := first(n, func(v *html.Node) bool { return v.Data == "img" && attr(v, "src") != "" })
	value := attr(img, "src")
	if strings.HasPrefix(value, "//") {
		value = "https:" + value
	}
	if strings.HasPrefix(value, "https://") {
		return value
	}
	return ""
}
func (p *Provider) row(n *html.Node) provider.KoreanRecord {
	r := provider.KoreanRecord{ID: attr(n, "trackid")}
	if r.ID == "" {
		r.ID = attr(n, "songid")
	}
	if r.ID == "" {
		walk(n, func(v *html.Node) {
			if r.ID == "" {
				r.ID = attr(v, "data-song-no")
				if r.ID == "" {
					r.ID = reference(v, "track")
				}
			}
		})
	}
	title := byClass(n, "title")
	if p.name == "melon" {
		title = first(n, func(v *html.Node) bool { return v.Data == "a" && strings.Contains(attr(v, "href"), "playSong(") })
	}
	if title != nil {
		r.Title = text(title)
	}
	r.Artists = artists(n)
	r.Image = image(n)
	walk(n, func(v *html.Node) {
		if v.Data == "a" && reference(v, "album") != "" {
			if name := text(v); name != "" && name != "곡정보" {
				r.AlbumID = reference(v, "album")
				r.Album = name
			}
		}
	})
	if !numeric.MatchString(r.ID) {
		r.ID = ""
	}
	return r
}
func meta(doc *html.Node, key string) string {
	n := first(doc, func(n *html.Node) bool {
		return n.Data == "meta" && (attr(n, "property") == key || attr(n, "name") == key)
	})
	return attr(n, "content")
}
func structured(doc *html.Node, kind string) map[string]any {
	var found map[string]any
	walk(doc, func(n *html.Node) {
		if found != nil || n.Data != "script" || attr(n, "type") != "application/ld+json" {
			return
		}
		var b strings.Builder
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			b.WriteString(c.Data)
		}
		var value any
		if json.Unmarshal([]byte(b.String()), &value) != nil {
			return
		}
		var find func(any)
		find = func(x any) {
			switch v := x.(type) {
			case map[string]any:
				if v["@type"] == kind {
					found = v
					return
				}
				if g := v["@graph"]; g != nil {
					find(g)
				}
			case []any:
				for _, item := range v {
					find(item)
				}
			}
		}
		find(value)
	})
	return found
}
func str(m map[string]any, key string) string { s, _ := m[key].(string); return s }
func number(value string) int64               { n, _ := strconv.ParseInt(value, 10, 64); return n }
func duration(value string) int64 {
	if m := regexp.MustCompile(`^PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?$`).FindStringSubmatch(value); len(m) > 0 {
		return (number(m[1])*3600 + number(m[2])*60 + number(m[3])) * 1000
	}
	if m := regexp.MustCompile(`^(\d{1,2}):(\d{2})$`).FindStringSubmatch(value); len(m) > 0 {
		return (number(m[1])*60 + number(m[2])) * 1000
	}
	return 0
}
func (p *Provider) detail(doc *html.Node, body []byte, id string) (provider.KoreanRecord, error) {
	data := structured(doc, "MusicRecording")
	actual := reference(&html.Node{Attr: []html.Attribute{{Key: "href", Val: str(data, "sameAs") + " " + meta(doc, "og:url")}}}, "track")
	if p.name == "genie" {
		actual = capture(`var\s+iSnsSeq\s*=\s*['"]([0-9]+)`, string(body))
	}
	if actual != id {
		return provider.KoreanRecord{}, errors.New("catalog returned missing or different track identity")
	}
	scope := byClass(doc, map[string]string{"bugs": "basicInfo", "genie": "song-main-infos", "melon": "section_info"}[p.name])
	if scope == nil {
		scope = doc
	}
	r := provider.KoreanRecord{ID: id, Title: str(data, "name"), ReleaseDate: str(data, "datePublished"), DurationMS: duration(str(data, "duration")), Image: str(data, "image"), ISRC: str(data, "isrcCode"), Fields: map[string]string{}, Structured: map[string]any{}}
	// Preserve unknown entity metadata; action payloads are not catalog fields.
	for key, value := range data {
		if key != "potentialAction" {
			r.Structured[key] = value
		}
	}

	album, _ := data["inAlbum"].(map[string]any)
	r.Album = str(album, "name")
	r.AlbumID = capture(`(?:/album/|albumId=)([0-9]+)`, str(album, "url")+" "+str(album, "@id"))
	if r.Title == "" {
		r.Title = text(byClass(scope, map[string]string{"genie": "name", "melon": "song_name"}[p.name]))
	}
	if r.Image == "" {
		r.Image = meta(doc, "og:image")
	}
	if r.Image == "" {
		r.Image = image(byClass(scope, "photo-zone"))
	}
	walk(scope, func(n *html.Node) {
		if n.Data == "th" || n.Data == "dt" {
			next := n.NextSibling
			for next != nil && next.Type != html.ElementNode {
				next = next.NextSibling
			}
			if next != nil && (next.Data == "td" || next.Data == "dd") {
				label := text(n)
				value := text(next)
				if label != "" && len(value) < 4096 {
					r.Fields[label] = value
				}
				switch label {
				case "아티스트", "가수":
					r.Artists = artists(next)
				case "앨범":
					if r.Album == "" {
						r.Album = text(next)
					}
					if r.AlbumID == "" {
						walk(next, func(a *html.Node) {
							if found := reference(a, "album"); found != "" {
								r.AlbumID = found
							}
						})
					}
				}
			}
		}
	})
	if p.name == "genie" {
		walk(scope, func(n *html.Node) {
			if n.Data != "li" {
				return
			}
			labelNode := byClass(n, "attr")
			valueNode := byClass(n, "value")
			if labelNode == nil || valueNode == nil {
				return
			}
			img := first(labelNode, func(v *html.Node) bool { return v.Data == "img" })
			code := capture(`txt_([0-9]+)\.png`, attr(img, "src"))
			label := map[string]string{"5": "아티스트", "6": "앨범", "7": "장르", "8": "재생시간", "19": "작곡가", "20": "작사가"}[code]
			if label == "" {
				return
			}
			r.Fields[label] = text(valueNode)
			switch code {
			case "5":
				r.Artists = artists(valueNode)
			case "6":
				if r.Album == "" {
					r.Album = text(valueNode)
				}
				walk(valueNode, func(v *html.Node) {
					if a := reference(v, "album"); a != "" {
						r.AlbumID = a
					}
				})
			case "8":
				r.DurationMS = duration(text(valueNode))
			}
		})
	}
	if len(r.Artists) == 0 && p.name == "melon" {
		r.Artists = artists(byClass(scope, "artist"))
	}
	if len(r.Artists) == 0 {
		if by, ok := data["byArtist"].(map[string]any); ok && str(by, "name") != "" {
			r.Artists = []provider.KoreanArtist{{Name: str(by, "name")}}
		}
	}
	if r.ISRC == "" {
		r.ISRC = capture(`(?i)\b([A-Z]{2}[A-Z0-9]{3}[0-9]{7})\b`, r.Fields["ISRC"])
	}
	if r.Title == "" || len(r.Artists) == 0 {
		return r, errors.New("incomplete catalog track metadata")
	}
	return r, nil
}
