package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/ommr/ommr/internal/models/canonical"
)

var mbidPattern = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)

type CoverArt struct {
	Client *Client
	base   string
}

func NewCoverArt(client *Client) *CoverArt { return &CoverArt{client, "https://coverartarchive.org"} }
func (*CoverArt) Name() string             { return "coverartarchive" }
func (c *CoverArt) Enrich(ctx context.Context, track canonical.Track) (*Result, error) {
	id := track.Album.IDs["musicbrainz"]
	if !mbidPattern.MatchString(id) {
		return nil, nil
	}
	var payload struct {
		Release string `json:"release"`
		Images  []struct {
			Image      string            `json:"image"`
			Front      bool              `json:"front"`
			Back       bool              `json:"back"`
			Approved   bool              `json:"approved"`
			Types      []string          `json:"types"`
			Thumbnails map[string]string `json:"thumbnails"`
			ID         any               `json:"id"`
			Comment    string            `json:"comment"`
		} `json:"images"`
	}
	var raw map[string]any
	if err := c.Client.Request(ctx, c.Name(), "GET", c.base+"/release/"+id, "", nil, &raw); err != nil {
		return nil, err
	}
	encoded, _ := json.Marshal(raw)
	if err := json.Unmarshal(encoded, &payload); err != nil {
		return nil, fmt.Errorf("invalid cover art response")
	}
	if payload.Release != "https://musicbrainz.org/release/"+id && payload.Release != "http://musicbrainz.org/release/"+id {
		return nil, fmt.Errorf("cover art release identity mismatch")
	}
	result := &Result{Data: map[string]any{"release_id": id, "artwork": raw}}
	for _, item := range payload.Images {
		kind := "artwork"
		if item.Front {
			kind = "cover"
		} else if item.Back {
			kind = "back"
		}
		if imageURL := artURL(item.Image); imageURL != "" {
			result.Images = append(result.Images, canonical.Image{URL: imageURL, Type: kind})
		}
		sizes := make([]string, 0, len(item.Thumbnails))
		for size := range item.Thumbnails {
			sizes = append(sizes, size)
		}
		sort.Strings(sizes)
		for _, size := range sizes {
			raw := item.Thumbnails[size]
			if imageURL := artURL(raw); imageURL != "" {
				result.Images = append(result.Images, canonical.Image{URL: imageURL, Type: "thumbnail"})
			}
		}
	}
	return result, nil
}

func artURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if u.Scheme == "http" && (u.Host == "coverartarchive.org" || u.Host == "archive.org" || strings.HasSuffix(u.Host, ".archive.org")) {
		u.Scheme = "https"
	}
	if httpsURL(u.String()) {
		return u.String()
	}
	return ""
}
