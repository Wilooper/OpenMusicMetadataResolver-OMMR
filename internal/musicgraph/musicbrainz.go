package musicgraph

func (b *Builder) MusicBrainz(kind string, m map[string]any) string { return b.musicBrainz(kind, m, 0) }
func (b *Builder) musicBrainz(kind string, m map[string]any, depth int) string {
	if depth > 4 {
		b.graph.Truncated = true
		return ""
	}
	node := b.Entity("musicbrainz", kind, text(m["id"]), first(m["title"], m["name"]), fields(m, "disambiguation", "date", "status", "barcode", "primary-type", "secondary-types", "isrcs", "iswcs", "language", "video"))
	if node == "" {
		return ""
	}
	for _, v := range list(m["artist-credit"]) {
		a := object(v)
		artist := object(a["artist"])
		name := first(a["name"], artist["name"])
		b.Credit("musicbrainz", node, text(artist["id"]), first(artist["name"], a["name"]), "primary_artist", name, text(a["joinphrase"]), nil)
	}
	if group := object(m["release-group"]); group != nil {
		g := b.musicBrainz("release_group", group, depth+1)
		b.Add(Edge{From: node, To: g, Type: "edition_of", Source: "musicbrainz"})
	}
	for _, v := range list(m["releases"]) {
		release := object(v)
		r := b.musicBrainz("release", release, depth+1)
		b.Add(Edge{From: node, To: r, Type: "appears_on", Source: "musicbrainz"})
	}
	for _, v := range list(m["label-info"]) {
		info := object(v)
		label := object(info["label"])
		target := b.Entity("musicbrainz", "label", text(label["id"]), text(label["name"]), nil)
		b.Add(Edge{From: node, To: target, Type: "released_by", Source: "musicbrainz", CatalogNumber: text(info["catalog-number"])})
	}
	for _, v := range list(m["relations"]) {
		r := object(v)
		raw := text(r["type"])
		if raw == "" {
			continue
		}
		attributes := []string{}
		for _, a := range list(r["attributes"]) {
			if text(a) != "" {
				attributes = append(attributes, text(a))
			}
		}
		for _, targetKind := range []string{"artist", "recording", "release", "release-group", "work", "label"} {
			target := object(r[targetKind])
			if target == nil {
				continue
			}
			targetNode := b.musicBrainz(normalKind(targetKind), target, depth+1)
			if targetNode == "" {
				continue
			}
			e := Edge{From: node, To: targetNode, Type: "relationship", Source: "musicbrainz", SourceRole: raw, Attributes: attributes, AttributeCredits: object(r["attribute-credits"]), SourceTypeID: text(r["type-id"]), SourceDirection: text(r["direction"]), Begin: text(r["begin"]), End: text(r["end"])}
			if ended, ok := r["ended"].(bool); ok {
				e.Ended = &ended
			}
			if targetKind == "artist" && creditRole(raw) {
				e.From = targetNode
				e.To = node
				e.Type = "credit"
				e.Role = role(raw)
				e.CreditedAs = first(r["target-credit"], target["name"])
			} else {
				if e.SourceDirection == "backward" {
					e.From, e.To = e.To, e.From
				}
				if e.SourceDirection == "forward" || e.SourceDirection == "backward" {
					e.Type = relationship(raw)
				}
			}
			b.Add(e)
		}
	}
	return node
}
func first(values ...any) string {
	for _, v := range values {
		if text(v) != "" {
			return text(v)
		}
	}
	return ""
}
func normalKind(kind string) string {
	if kind == "release-group" {
		return "release_group"
	}
	return kind
}
func relationship(raw string) string {
	switch raw {
	case "performance":
		return "recording_of"
	case "remix":
		return "remix_of"
	case "edit":
		return "edit_of"
	case "remaster":
		return "remaster_of"
	case "samples material":
		return "samples_material"
	case "replaced by":
		return "replaced_by"
	case "supporting release":
		return "supports_release"
	}
	return "relationship"
}
func creditRole(r string) bool {
	switch r {
	case "composer", "lyricist", "writer", "librettist", "producer", "co-producer", "executive producer", "engineer", "audio", "sound", "mix", "mastering", "recording", "balance", "programming", "editor", "performer", "instrument", "vocal", "arranger", "instrument arranger", "vocal arranger", "orchestrator", "conductor", "chorus master", "concertmaster", "remixer", "translator", "artwork", "design", "photography", "graphic design", "illustration", "liner notes":
		return true
	}
	return false
}
