package musicgraph

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"html"
	"sort"
	"strings"

	"github.com/ommr/ommr/internal/models/canonical"
)

const MaxNodes = 4096
const MaxEdges = 8192

type Node struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Provider   string         `json:"provider,omitempty"`
	NativeID   string         `json:"native_id,omitempty"`
	Name       string         `json:"name,omitempty"`
	Local      bool           `json:"local,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
}
type Edge struct {
	CatalogNumber    string         `json:"catalog_number,omitempty"`
	From             string         `json:"from"`
	To               string         `json:"to"`
	Type             string         `json:"type"`
	Source           string         `json:"source"`
	Role             string         `json:"role,omitempty"`
	SourceRole       string         `json:"source_role,omitempty"`
	CreditedAs       string         `json:"credited_as,omitempty"`
	JoinPhrase       string         `json:"join_phrase,omitempty"`
	Attributes       []string       `json:"attributes,omitempty"`
	AttributeCredits map[string]any `json:"attribute_credits,omitempty"`
	SourceTypeID     string         `json:"source_type_id,omitempty"`
	SourceDirection  string         `json:"source_direction,omitempty"`
	Begin            string         `json:"begin,omitempty"`
	End              string         `json:"end,omitempty"`
	Ended            *bool          `json:"ended,omitempty"`
	Position         int            `json:"position,omitempty"`
	DiscNumber       int            `json:"disc_number,omitempty"`
	TrackNumber      string         `json:"track_number,omitempty"`
}
type Graph struct {
	Version   int    `json:"version"`
	Root      string `json:"root"`
	Nodes     []Node `json:"nodes"`
	Edges     []Edge `json:"edges"`
	Truncated bool   `json:"truncated"`
}
type Record struct {
	Provider string
	Track    canonical.Track
	Catalog  map[string]any
}
type Builder struct {
	graph       Graph
	nodes       map[string]Node
	edges       map[string]Edge
	creditNames map[string]bool
}

func New(root string) *Builder {
	return &Builder{graph: Graph{Version: 1, Root: root}, nodes: map[string]Node{}, edges: map[string]Edge{}, creditNames: map[string]bool{}}
}
func ID(provider, kind, native string) string {
	if native == "" {
		return ""
	}
	return provider + ":" + kind + ":" + native
}
func (b *Builder) Entity(provider, kind, native, name string, properties map[string]any) string {
	id := ID(provider, kind, native)
	if id == "" {
		return ""
	}
	if old, ok := b.nodes[id]; ok {
		if old.Name == "" {
			old.Name = name
		}
		if old.Properties == nil {
			old.Properties = map[string]any{}
		}
		for key, value := range properties {
			if _, ok := old.Properties[key]; !ok {
				old.Properties[key] = value
			}
		}
		b.nodes[id] = old
		return id
	}
	if len(b.nodes) >= MaxNodes {
		b.graph.Truncated = true
		return ""
	}
	b.nodes[id] = Node{ID: id, Type: kind, Provider: provider, NativeID: native, Name: name, Properties: properties}
	return id
}
func (b *Builder) Add(e Edge) {
	if e.From == "" || e.To == "" || e.From == e.To {
		return
	}
	if _, ok := b.nodes[e.From]; !ok {
		return
	}
	if _, ok := b.nodes[e.To]; !ok {
		return
	}
	sort.Strings(e.Attributes)
	data, _ := json.Marshal(e)
	key := string(data)
	if _, ok := b.edges[key]; ok {
		return
	}
	if len(b.edges) >= MaxEdges {
		b.graph.Truncated = true
		return
	}
	b.edges[key] = e
	if e.Type == "credit" {
		b.creditNames[creditKey(e.Source, e.To, e.Role, b.nodes[e.From].Name)] = true
	}
}
func (b *Builder) Result() Graph {
	g := b.graph
	g.Nodes = []Node{}
	g.Edges = []Edge{}
	for _, n := range b.nodes {
		g.Nodes = append(g.Nodes, n)
	}
	keys := make([]string, 0, len(b.edges))
	for key := range b.edges {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		g.Edges = append(g.Edges, b.edges[key])
	}
	sort.Slice(g.Nodes, func(i, j int) bool { return g.Nodes[i].ID < g.Nodes[j].ID })
	return g
}
func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func list(v any) []any            { a, _ := v.([]any); return a }
func text(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case json.Number:
		return string(v)
	case float64:
		return fmt.Sprint(v)
	}
	return ""
}
func fields(m map[string]any, keys ...string) map[string]any {
	out := map[string]any{}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			out[k] = v
		}
	}
	return out
}
func role(raw string) string {
	r := strings.ToLower(strings.TrimSpace(raw))
	switch r {
	case "main", "primary", "primary_artists", "artist":
		return "primary_artist"
	case "featured", "featured_artists":
		return "featured_artist"
	case "singer", "singers", "vocals", "vocal":
		return "vocalist"
	case "music":
		return "composer"
	case "작곡", "작곡가":
		return "composer"
	case "작사", "작사가":
		return "lyricist"
	case "편곡":
		return "arranger"
	case "mix":
		return "mixing_engineer"
	case "mastering":
		return "mastering_engineer"
	case "recording":
		return "recording_engineer"
	case "instrument":
		return "instrumentalist"
	}
	return strings.ReplaceAll(r, " ", "_")
}
func (b *Builder) Credit(provider, target, native, name, raw, credited, join string, attributes []string) {
	if target == "" || name == "" || raw == "" {
		return
	}
	if native == "" && b.creditNames[creditKey(provider, target, role(raw), name)] {
		return
	}
	artist := b.Entity(provider, "artist", native, name, nil)
	if artist == "" {
		// Name-only credits are local to a source entity, never globally joined by name.
		sum := sha256.Sum256([]byte(provider + "\x00" + target + "\x00" + name))
		artist = fmt.Sprintf("%s:local_artist:%x", provider, sum[:12])
		if _, ok := b.nodes[artist]; !ok {
			if len(b.nodes) >= MaxNodes {
				b.graph.Truncated = true
				return
			}
			b.nodes[artist] = Node{ID: artist, Type: "artist", Provider: provider, Name: name, Local: true}
		}
	}
	b.Add(Edge{From: artist, To: target, Type: "credit", Source: provider, Role: role(raw), SourceRole: raw, CreditedAs: credited, JoinPhrase: join, Attributes: attributes})
}
func TrackGraph(track canonical.Track, records []Record, release map[string]any) Graph {
	root := ID("ommr", "recording", track.CanonicalID)
	if root == "" && len(records) > 0 {
		root = ID(records[0].Provider, "recording", records[0].Track.IDs[records[0].Provider])
	}
	b := New(root)
	if root == "" {
		return b.Result()
	}
	if track.CanonicalID != "" {
		b.Entity("ommr", "recording", track.CanonicalID, track.Title, nil)
	}
	records = append([]Record(nil), records...)
	sort.Slice(records, func(i, j int) bool { return records[i].Provider < records[j].Provider })
	for _, r := range records {
		native := r.Track.IDs[r.Provider]
		node := b.Entity(r.Provider, "recording", native, r.Track.Title, nil)
		if node == "" {
			continue
		}
		b.Add(Edge{From: root, To: node, Type: "matched_catalog_entity", Source: r.Provider})
		if album := r.Track.Album.IDs[r.Provider]; album != "" {
			edition := b.Entity(r.Provider, "release", album, r.Track.Album.Title, nil)
			b.Add(Edge{From: node, To: edition, Type: "appears_on", Source: r.Provider, Position: r.Track.TrackNumber, DiscNumber: r.Track.DiscNumber})
		}
		if r.Provider == "musicbrainz" && text(r.Catalog["id"]) == native {
			b.MusicBrainz("recording", r.Catalog)
			continue
		}
		if ownsCatalog(r) {
			b.CatalogCredits(r.Provider, node, r.Catalog)
		}
		for _, a := range r.Track.Artists {
			raw := a.Role
			if raw == "" {
				raw = "artist"
			}
			b.Credit(r.Provider, node, a.IDs[r.Provider], a.Name, raw, a.Name, "", nil)
		}
		for _, c := range r.Track.Credits {
			for _, raw := range c.Roles {
				b.Credit(r.Provider, node, "", c.Name, raw, c.Name, "", nil)
			}
		}
	}
	// Release enrichment is accepted only if the resolved recording occurs on that exact edition.
	if text(release["id"]) == track.Album.IDs["musicbrainz"] && containsRecording(release, track.IDs["musicbrainz"]) {
		b.MusicBrainz("release", release)
	}
	return b.Result()
}
func containsRecording(release map[string]any, id string) bool {
	if id == "" {
		return false
	}
	for _, v := range list(release["media"]) {
		for _, t := range list(object(v)["tracks"]) {
			if text(object(object(t)["recording"])["id"]) == id {
				return true
			}
		}
	}
	return false
}
func (b *Builder) CatalogCredits(provider, target string, m map[string]any) {
	if provider == "melon" || provider == "genie" || provider == "bugs" {
		for _, v := range list(m["credits"]) {
			a := object(v)
			b.Credit(provider, target, text(a["id"]), text(a["name"]), first(a["source_role"], a["role"]), text(a["name"]), "", nil)
		}
	}
	if provider == "jiosaavn" {
		artistMap := object(object(m["more_info"])["artistMap"])
		if artistMap == nil {
			artistMap = object(m["artistMap"])
		}
		for _, key := range []string{"primary_artists", "featured_artists", "artists"} {
			for _, v := range list(artistMap[key]) {
				a := object(v)
				r := text(a["role"])
				if r == "" && key != "artists" {
					r = key
				}
				name := html.UnescapeString(text(a["name"]))
				b.Credit(provider, target, text(a["id"]), name, r, name, "", nil)
			}
		}
	}
	if provider == "deezer" {
		for _, v := range list(m["contributors"]) {
			a := object(v)
			b.Credit(provider, target, text(a["id"]), text(a["name"]), text(a["role"]), text(a["name"]), "", nil)
		}
	}
}

func creditKey(source, target, role, name string) string {
	data, _ := json.Marshal([]string{source, target, role, name})
	return string(data)
}

func ownsCatalog(r Record) bool {
	native := r.Track.IDs[r.Provider]
	if native == "" {
		return false
	}
	switch r.Provider {
	case "applemusic":
		return text(r.Catalog["trackId"]) == native
	case "ytmusic":
		return text(r.Catalog["videoId"]) == native
	case "jiosaavn":
		return text(r.Catalog["id"]) == native || (r.Track.IDs["jiosaavn_track_id"] != "" && text(r.Catalog["id"]) == r.Track.IDs["jiosaavn_track_id"])
	default:
		return text(r.Catalog["id"]) == native
	}
}
