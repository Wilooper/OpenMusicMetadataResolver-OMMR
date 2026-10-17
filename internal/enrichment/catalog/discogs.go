package catalog

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/ommr/ommr/internal/models/canonical"
)

type Discogs struct {
	Client      *Client
	token, base string
}

func NewDiscogs(c *Client, token string) *Discogs {
	return &Discogs{c, token, "https://api.discogs.com"}
}
func (*Discogs) Name() string { return "discogs" }
func (c *Discogs) Enrich(ctx context.Context, track canonical.Track) (*Result, error) {
	if c.token == "" || track.Title == "" || primary(track) == "" || track.Album.Title == "" {
		return nil, nil
	}
	q := url.Values{"type": {"release"}, "artist": {primary(track)}, "release_title": {track.Album.Title}, "track": {track.Title}, "per_page": {"3"}}
	var search struct {
		Results []struct {
			ID int64 `json:"id"`
		} `json:"results"`
	}
	auth := "Discogs token=" + c.token
	if err := c.Client.Request(ctx, c.Name(), "GET", c.base+"/database/search?"+q.Encode(), auth, nil, &search); err != nil {
		return nil, err
	}
	matches := []map[string]any{}
	for _, hit := range search.Results {
		if hit.ID <= 0 {
			continue
		}
		var record map[string]any
		if err := c.Client.Request(ctx, c.Name(), "GET", c.base+"/releases/"+strconv.FormatInt(hit.ID, 10), auth, nil, &record); err != nil {
			return nil, err
		}
		if !exact(fmt.Sprint(record["title"]), track.Album.Title) {
			continue
		}
		artists, _ := record["artists"].([]any)
		artistMatch := false
		for _, raw := range artists {
			artist, _ := raw.(map[string]any)
			name, _ := artist["name"].(string)
			if exact(name, primary(track)) {
				artistMatch = true
			}
		}
		if !artistMatch {
			continue
		}
		tracks, _ := record["tracklist"].([]any)
		for _, raw := range tracks {
			item, _ := raw.(map[string]any)
			if !exact(fmt.Sprint(item["title"]), track.Title) {
				continue
			}
			// A release match is edition context only; never manufacture a recording ID.
			if track.DurationMS > 0 {
				parts := strings.Split(fmt.Sprint(item["duration"]), ":")
				if len(parts) != 2 {
					continue
				}
				mins, e1 := strconv.ParseInt(parts[0], 10, 64)
				secs, e2 := strconv.ParseInt(parts[1], 10, 64)
				delta := (mins*60+secs)*1000 - track.DurationMS
				if e1 != nil || e2 != nil || delta < -2000 || delta > 2000 {
					continue
				}
			}
			matches = append(matches, map[string]any{"scope": "release_track_context", "release": record, "track": item})
		}
	}
	if len(matches) == 0 {
		return nil, nil
	}
	return &Result{Data: map[string]any{"editions": matches}}, nil
}
