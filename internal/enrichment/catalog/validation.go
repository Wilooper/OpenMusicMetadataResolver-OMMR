package catalog

import (
	"context"
	"fmt"
	"net/url"

	"github.com/ommr/ommr/internal/models/canonical"
)

func (c *LastFM) Validate(ctx context.Context) error {
	result, err := c.Enrich(ctx, canonical.Track{Title: "Believe", Artists: []canonical.Artist{{Name: "Cher"}}})
	if err != nil {
		return err
	}
	if result == nil {
		return fmt.Errorf("Last.fm access unavailable")
	}
	return nil
}
func (c *Discogs) Validate(ctx context.Context) error {
	var result struct {
		ID int64 `json:"id"`
	}
	if err := c.Client.Request(ctx, c.Name(), "GET", c.base+"/oauth/identity", "Discogs token="+c.token, nil, &result); err != nil {
		return err
	}
	if result.ID <= 0 {
		return fmt.Errorf("Discogs identity is missing")
	}
	return nil
}
func (c *AcoustID) Validate(ctx context.Context) error {
	form := url.Values{"client": {c.key}, "trackid": {"9ff43b6a-4f16-427c-93c2-92307ca505e0"}, "format": {"json"}}
	var result map[string]any
	if err := c.Client.Request(ctx, "acoustid", "POST", c.base, "", form, &result); err != nil {
		return err
	}
	if result["status"] != "ok" {
		return fmt.Errorf("AcoustID application access unavailable")
	}
	return nil
}
