package albums

import (
	"github.com/ommr/ommr/internal/musicgraph"
	"strconv"
)

func attachGraph(album *Album) {
	root := musicgraph.ID(album.Provider, "release", album.ID)
	b := musicgraph.New(root)
	if album.Provider == "musicbrainz" {
		// The root record omits media to respect track-list limits. Add only returned entries below.
		b.MusicBrainz("release", album.ProviderData)
	} else {
		b.Entity(album.Provider, "release", album.ID, album.Title, nil)
		for _, a := range album.Artists {
			b.Credit(album.Provider, root, a.ID, a.Name, "primary_artist", a.Name, a.JoinPhrase, nil)
		}
		b.CatalogCredits(album.Provider, root, album.ProviderData)
	}
	for _, t := range album.Tracks {
		entry := b.Entity(album.Provider, "track_entry", album.ID+"/"+strconv.Itoa(t.DiscNumber)+"/"+strconv.Itoa(t.Position)+"/"+t.ID, t.Title, nil)
		b.Add(musicgraph.Edge{From: entry, To: root, Type: "appears_on", Source: album.Provider, Position: t.Position, DiscNumber: t.DiscNumber, TrackNumber: t.TrackNumber})
		target := entry
		if album.Provider == "musicbrainz" && t.RecordingID != "" {
			recording := obj(t.ProviderData["recording"])
			if text(recording["id"]) == t.RecordingID {
				target = b.MusicBrainz("recording", recording)
				b.Add(musicgraph.Edge{From: entry, To: target, Type: "instance_of", Source: album.Provider})
			}
		}
		for _, a := range t.Artists {
			b.Credit(album.Provider, entry, a.ID, a.Name, "primary_artist", a.Name, a.JoinPhrase, nil)
		}
		b.CatalogCredits(album.Provider, target, t.ProviderData)
	}
	graph := b.Result()
	album.MusicGraph = &graph
	album.CreditDetails = musicgraph.DetailedCredits(graph)
}
