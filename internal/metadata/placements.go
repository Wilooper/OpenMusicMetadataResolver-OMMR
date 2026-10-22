package metadata

import (
	"encoding/json"
	"sort"

	"github.com/ommr/ommr/internal/models/canonical"
)

type AlbumPlacement struct {
	Provider    string `json:"provider"`
	AlbumID     string `json:"album_id"`
	TrackID     string `json:"track_id"`
	DiscNumber  int    `json:"disc_number,omitempty"`
	TrackNumber int    `json:"track_number,omitempty"`
	Selected    bool   `json:"selected,omitempty"`
}

func attachPlacements(track *canonical.Track, candidates []canonical.TrackCandidate) {
	placements := []AlbumPlacement{}
	for _, c := range candidates {
		a, t := c.Track.Album.IDs[c.Provider], c.Track.IDs[c.Provider]
		if a == "" || t == "" {
			continue
		}
		selected := len(track.FieldSources["album_position"]) == 1 && track.FieldSources["album_position"][0] == c.Provider && track.Album.IDs[c.Provider] == a
		placements = append(placements, AlbumPlacement{Provider: c.Provider, AlbumID: a, TrackID: t, DiscNumber: c.Track.DiscNumber, TrackNumber: c.Track.TrackNumber, Selected: selected})
	}
	sort.Slice(placements, func(i, j int) bool { return placements[i].Provider < placements[j].Provider })
	if len(placements) > 0 {
		track.Extensions["album_placements"], _ = json.Marshal(placements)
	}
}
