package provider

import (
	"encoding/json"
	"strconv"
	"strings"
)

// FlexibleInt tolerates JSON values that may be numbers ("height": 400) or
// percentage strings ("width": "100%"), which SoundCloud's oEmbed endpoint
// mixes in the same response.
type FlexibleInt int

func (f *FlexibleInt) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		s = strings.TrimSpace(strings.TrimSuffix(s, "%"))
		if n, err := strconv.Atoi(s); err == nil {
			*f = FlexibleInt(n)
		}
		return nil
	}
	var n int
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*f = FlexibleInt(n)
	return nil
}

// SoundCloudOembedResponse represents the SoundCloud oEmbed endpoint response.
type SoundCloudOembedResponse struct {
	Version      float64     `json:"version"`
	Type         string      `json:"type"`
	ProviderName string      `json:"provider_name"`
	ProviderURL  string      `json:"provider_url"`
	Height       int         `json:"height"`
	Width        FlexibleInt `json:"width"`
	Title        string      `json:"title"`
	Description  string      `json:"description"`
	ThumbnailURL string      `json:"thumbnail_url"`
	HTML         string      `json:"html"`
	AuthorName   string      `json:"author_name"`
	AuthorURL    string      `json:"author_url"`
}
