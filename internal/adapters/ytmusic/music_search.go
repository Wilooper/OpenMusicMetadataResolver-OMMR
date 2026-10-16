package ytmusic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/canonical"
)

var musicSearchURL = "https://music.youtube.com/youtubei/v1/search?prettyPrint=false"

type musicRun struct {
	Text               string `json:"text"`
	NavigationEndpoint struct {
		WatchEndpoint struct {
			VideoID string `json:"videoId"`
			Configs struct {
				Music struct {
					Type string `json:"musicVideoType"`
				} `json:"watchEndpointMusicConfig"`
			} `json:"watchEndpointMusicSupportedConfigs"`
		} `json:"watchEndpoint"`
		BrowseEndpoint struct {
			BrowseID string `json:"browseId"`
			Configs  struct {
				Music struct {
					PageType string `json:"pageType"`
				} `json:"browseEndpointContextMusicConfig"`
			} `json:"browseEndpointContextSupportedConfigs"`
		} `json:"browseEndpoint"`
	} `json:"navigationEndpoint"`
}

type musicItem struct {
	FlexColumns []struct {
		Column struct {
			Text struct {
				Runs []musicRun `json:"runs"`
			} `json:"text"`
		} `json:"musicResponsiveListItemFlexColumnRenderer"`
	} `json:"flexColumns"`
	Thumbnail struct {
		Renderer struct {
			Thumbnail struct {
				Thumbnails []struct {
					URL    string `json:"url"`
					Width  int    `json:"width"`
					Height int    `json:"height"`
				} `json:"thumbnails"`
			} `json:"thumbnail"`
		} `json:"musicThumbnailRenderer"`
	} `json:"thumbnail"`
	Badges []struct {
		Renderer struct {
			Icon struct {
				Type string `json:"iconType"`
			} `json:"icon"`
		} `json:"musicInlineBadgeRenderer"`
	} `json:"badges"`
}

func (i *InnerTube) searchSongs(ctx context.Context, term string, limit int) ([]canonical.TrackCandidate, error) {
	payload, _ := json.Marshal(map[string]any{
		"context": map[string]any{"client": map[string]any{"clientName": "WEB_REMIX", "clientVersion": "1.20260930.03.00", "hl": "en"}},
		"query":   term,
		"params":  "EgWKAQIIAWoKEAkQBRAKEAMQBA%3D%3D",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, musicSearchURL, strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	i.setAuth(req)
	resp, err := i.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("youtube music search status %d", resp.StatusCode)
	}
	body, err := adapters.ReadJSONBody(resp)
	if err != nil {
		return nil, err
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if _, ok := parsed["error"]; ok {
		return nil, fmt.Errorf("youtube music search returned an API error")
	}
	var contents any
	if err := json.Unmarshal(parsed["contents"], &contents); err != nil {
		return nil, fmt.Errorf("youtube music search returned no contents")
	}
	results := make([]canonical.TrackCandidate, 0, limit)
	seen := map[string]bool{}
	var walk func(any, int)
	walk = func(value any, depth int) {
		if depth > 24 || len(results) >= limit {
			return
		}
		switch node := value.(type) {
		case map[string]any:
			if renderer, ok := node["musicResponsiveListItemRenderer"].(map[string]any); ok {
				if candidate, ok := parseMusicItem(renderer); ok && !seen[candidate.Track.IDs[ProviderName]] {
					seen[candidate.Track.IDs[ProviderName]] = true
					results = append(results, candidate)
				}
				return
			}
			keys := make([]string, 0, len(node))
			for key := range node {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				walk(node[key], depth+1)
			}
		case []any:
			for _, child := range node {
				walk(child, depth+1)
			}
		}
	}
	walk(contents, 0)
	return results, nil
}

func parseMusicItem(renderer map[string]any) (canonical.TrackCandidate, bool) {
	body, _ := json.Marshal(renderer)
	var item musicItem
	if json.Unmarshal(body, &item) != nil || len(item.FlexColumns) == 0 {
		return canonical.TrackCandidate{}, false
	}
	track := canonical.Track{IDs: map[string]string{}, Sources: []string{ProviderName}}
	for col, column := range item.FlexColumns {
		for _, run := range column.Column.Text.Runs {
			if col == 0 {
				track.Title += run.Text
				watch := run.NavigationEndpoint.WatchEndpoint
				if watch.VideoID != "" {
					if watch.Configs.Music.Type != "MUSIC_VIDEO_TYPE_ATV" {
						return canonical.TrackCandidate{}, false
					}
					track.IDs[ProviderName] = watch.VideoID
				}
				continue
			}
			browse := run.NavigationEndpoint.BrowseEndpoint
			switch browse.Configs.Music.PageType {
			case "MUSIC_PAGE_TYPE_ARTIST":
				track.Artists = append(track.Artists, canonical.Artist{Name: run.Text, Role: "main", IDs: map[string]string{ProviderName: browse.BrowseID}})
			case "MUSIC_PAGE_TYPE_ALBUM":
				track.Album.Title = run.Text
				track.Album.IDs = map[string]string{ProviderName: browse.BrowseID}
			default:
				if lengthPattern.MatchString(strings.TrimSpace(run.Text)) {
					track.DurationMS = parseLengthToMS(run.Text)
				}
			}
		}
	}
	if track.IDs[ProviderName] == "" || track.Title == "" || len(track.Artists) == 0 {
		return canonical.TrackCandidate{}, false
	}
	for _, image := range item.Thumbnail.Renderer.Thumbnail.Thumbnails {
		track.Images = append(track.Images, canonical.Image{URL: image.URL, Width: image.Width, Height: image.Height, Type: "cover"})
	}
	for _, badge := range item.Badges {
		if badge.Renderer.Icon.Type == "MUSIC_EXPLICIT_BADGE" {
			track.Explicit = true
		}
	}
	// Attach only this song's catalog row; menus and overlays contain account actions.
	entity := make(map[string]any, len(renderer))
	for key, value := range renderer {
		if key != "menu" && key != "overlay" && key != "trackingParams" {
			entity[key] = value
		}
	}
	entity["videoId"] = track.IDs[ProviderName]
	raw, _ := json.Marshal(entity)
	return canonical.TrackCandidate{Provider: ProviderName, Track: track, RawResponse: raw}, true
}
