package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/ommr/ommr/internal/models/canonical"
)

type LastFM struct {
	Client    *Client
	key, base string
}

func NewLastFM(client *Client, key string) *LastFM {
	return &LastFM{client, key, "https://ws.audioscrobbler.com/2.0/"}
}
func (*LastFM) Name() string { return "lastfm" }
func (c *LastFM) Enrich(ctx context.Context, track canonical.Track) (*Result, error) {
	if c.key == "" || track.Title == "" || primary(track) == "" {
		return nil, nil
	}
	q := url.Values{"method": {"track.getInfo"}, "api_key": {c.key}, "format": {"json"}, "autocorrect": {"0"}, "artist": {primary(track)}, "track": {track.Title}}
	if mbidPattern.MatchString(track.IDs["musicbrainz"]) {
		q.Set("mbid", track.IDs["musicbrainz"])
	}
	var payload struct {
		Track json.RawMessage `json:"track"`
		Error int             `json:"error"`
	}
	if err := c.Client.Request(ctx, c.Name(), "GET", c.base+"?"+q.Encode(), "", nil, &payload); err != nil {
		return nil, err
	}
	if payload.Error != 0 {
		return nil, fmt.Errorf("Last.fm API error %d", payload.Error)
	}
	var record struct {
		Name   string `json:"name"`
		MBID   string `json:"mbid"`
		Artist struct {
			Name string `json:"name"`
		} `json:"artist"`
	}
	if err := json.Unmarshal(payload.Track, &record); err != nil {
		return nil, fmt.Errorf("missing Last.fm track")
	}
	if !exact(record.Name, track.Title) || !exact(record.Artist.Name, primary(track)) {
		return nil, fmt.Errorf("Last.fm track identity mismatch")
	}
	if track.IDs["musicbrainz"] != "" && record.MBID != "" && record.MBID != track.IDs["musicbrainz"] {
		return nil, fmt.Errorf("Last.fm recording identity mismatch")
	}
	// Tags and wiki text are contextual metadata, not recording identity evidence.
	return &Result{Data: map[string]any{"scope": "track_context", "track": payload.Track}}, nil
}
