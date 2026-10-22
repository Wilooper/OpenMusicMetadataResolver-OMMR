package catalog

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

type AcoustID struct {
	Client    *Client
	key, base string
}

func NewAcoustID(c *Client, key string) *AcoustID {
	return &AcoustID{c, key, "https://api.acoustid.org/v2/lookup"}
}
func (c *AcoustID) Lookup(ctx context.Context, fingerprint string, duration int) (map[string]any, error) {
	if c.key == "" {
		return nil, fmt.Errorf("AcoustID application key is not configured")
	}
	if fingerprint == "" || len(fingerprint) > 65536 || duration <= 0 || duration > 86400 {
		return nil, fmt.Errorf("valid fingerprint and duration (1..86400 seconds) required")
	}
	form := url.Values{"client": {c.key}, "fingerprint": {fingerprint}, "duration": {strconv.Itoa(duration)}, "meta": {"recordings releases releasegroups isrcs"}, "format": {"json"}}
	var result map[string]any
	if err := c.Client.Request(ctx, "acoustid", "POST", c.base, "", form, &result); err != nil {
		return nil, err
	}
	if result["status"] != "ok" {
		return nil, fmt.Errorf("AcoustID lookup failed")
	}
	// Return all candidates; acoustic similarity alone cannot select a unique edition.
	return result, nil
}
