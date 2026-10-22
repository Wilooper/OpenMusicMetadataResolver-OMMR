package catalog

import (
	"context"
	"fmt"
	"net/url"

	"github.com/ommr/ommr/internal/models/canonical"
)

type MusicBrainzRelease struct {
	Client *Client
	base   string
}

func NewMusicBrainzRelease(c *Client) *MusicBrainzRelease {
	return &MusicBrainzRelease{c, "https://musicbrainz.org/ws/2"}
}
func (*MusicBrainzRelease) Name() string { return "musicbrainz_release" }
func (c *MusicBrainzRelease) Enrich(ctx context.Context, track canonical.Track) (*Result, error) {
	id := track.Album.IDs["musicbrainz"]
	if !mbidPattern.MatchString(id) {
		return nil, nil
	}
	q := url.Values{"inc": {"artist-credits+labels+recordings+release-groups+release-rels+artist-rels+recording-rels+recording-level-rels+work-rels+work-level-rels+url-rels"}, "fmt": {"json"}}
	var record map[string]any
	if err := c.Client.Request(ctx, "musicbrainz", "GET", c.base+"/release/"+id+"?"+q.Encode(), "", nil, &record); err != nil {
		return nil, err
	}
	if record["id"] != id || !exact(fmt.Sprint(record["title"]), track.Album.Title) {
		return nil, fmt.Errorf("MusicBrainz release identity mismatch")
	}
	// The verified recording must actually occur on the returned edition.
	recordingID := track.IDs["musicbrainz"]
	found := false
	if media, ok := record["media"].([]any); ok {
		for _, m := range media {
			medium, _ := m.(map[string]any)
			tracks, _ := medium["tracks"].([]any)
			for _, t := range tracks {
				item, _ := t.(map[string]any)
				rec, _ := item["recording"].(map[string]any)
				if rec["id"] == recordingID && recordingID != "" {
					found = true
				}
			}
		}
	}
	if !found {
		return nil, fmt.Errorf("recording is not present on release")
	}
	return &Result{Data: map[string]any{"scope": "release", "release": record}}, nil
}
