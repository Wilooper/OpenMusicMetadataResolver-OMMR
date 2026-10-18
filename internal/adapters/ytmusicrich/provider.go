package ytmusicrich

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/extractors"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/providerhealth"
)

type Caller interface {
	Call(context.Context, string, string, int) (json.RawMessage, error)
}
type Provider struct {
	adapters.ProviderAdapter
	Pool     Caller
	Health   *providerhealth.Manager
	Profiles bool
}

func (p *Provider) Version() string { return p.ProviderAdapter.Version() + "+python-v1" }
func (p *Provider) EnrichCandidate(ctx context.Context, c canonical.TrackCandidate) (*canonical.TrackCandidate, error) {
	type target struct{ kind, id string }
	targets := []target{{"song", c.Track.IDs["ytmusic"]}}
	if p.Profiles {
		if id := c.Track.Album.IDs["ytmusic"]; id != "" {
			targets = append(targets, target{"album", id})
		}
		count := 0
		for _, artist := range c.Track.Artists {
			if count == 2 {
				break
			}
			if id := artist.IDs["ytmusic"]; id != "" {
				targets = append(targets, target{"artist", id})
				count++
			}
		}
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	records := map[string]json.RawMessage{}
	partial := false
	for _, item := range targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			raw, err := p.call(ctx, item.kind, item.id, 20)
			var value struct {
				ID      string `json:"id"`
				Partial bool   `json:"partial"`
			}
			if err == nil && (json.Unmarshal(raw, &value) != nil || value.ID != item.id) {
				err = errors.New("catalog identity mismatch")
			}
			if err == nil && item.kind == "song" && !sameSong(raw, item.id) {
				err = errors.New("song identity mismatch")
			}
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				partial = true
				return
			}
			partial = partial || value.Partial
			records[item.kind+":"+item.id] = raw
		}()
	}
	wg.Wait()
	if len(records) == 0 {
		return nil, errors.New("YouTube catalog metadata unavailable")
	}
	// Accepted native references own the context. Profiles never introduce recording IDs.
	encoded, _ := json.Marshal(c)
	var rich canonical.TrackCandidate
	_ = json.Unmarshal(encoded, &rich)
	if rich.Track.Extensions == nil {
		rich.Track.Extensions = map[string]json.RawMessage{}
	}
	rich.Track.Extensions["youtube_catalog"], _ = json.Marshal(records)
	if partial {
		rich.Track.Extensions["_enrichment_partial"] = json.RawMessage(`true`)
	}
	return &rich, nil
}
func sameSong(raw json.RawMessage, id string) bool {
	var value struct {
		Song struct {
			VideoDetails struct {
				VideoID string `json:"videoId"`
			} `json:"videoDetails"`
		} `json:"song"`
		Watch struct {
			Tracks []struct {
				VideoID string `json:"videoId"`
			} `json:"tracks"`
		} `json:"watch"`
	}
	if json.Unmarshal(raw, &value) != nil {
		return false
	}
	if value.Song.VideoDetails.VideoID != "" && value.Song.VideoDetails.VideoID != id {
		return false
	}
	for _, track := range value.Watch.Tracks {
		if track.VideoID != id {
			return false
		}
	}
	return value.Song.VideoDetails.VideoID == id || len(value.Watch.Tracks) > 0
}
func (p *Provider) call(ctx context.Context, kind, id string, limit int) (json.RawMessage, error) {
	if p.Health == nil {
		return p.Pool.Call(ctx, kind, id, limit)
	}
	lease, err := p.Health.Begin("ytmusic_python")
	if err != nil {
		return nil, err
	}
	data, err := p.Pool.Call(ctx, kind, id, limit)
	if errors.Is(err, extractors.ErrBusy) {
		p.Health.Skip("ytmusic_python", lease)
	} else {
		p.Health.Finish(ctx, "ytmusic_python", lease, err)
	}
	return data, err
}
