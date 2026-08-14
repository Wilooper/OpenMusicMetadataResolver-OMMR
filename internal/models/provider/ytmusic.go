package provider

// YTOembedResponse represents response from YouTube oEmbed endpoint.
type YTOembedResponse struct {
	Title        string `json:"title"`
	AuthorName   string `json:"author_name"`
	AuthorURL    string `json:"author_url"`
	Type         string `json:"type"`
	Height       int    `json:"height"`
	Width        int    `json:"width"`
	Version      string `json:"version"`
	ProviderName string `json:"provider_name"`
	ThumbnailURL string `json:"thumbnail_url"`
}

// YTPlayerResponse represents parsed YouTube InnerTube player metadata payload.
type YTPlayerResponse struct {
	VideoDetails YTVideoDetails `json:"videoDetails"`
}

type YTVideoDetails struct {
	VideoID          string   `json:"videoId"`
	Title            string   `json:"title"`
	LengthSeconds    string   `json:"lengthSeconds"`
	Keywords         []string `json:"keywords"`
	ChannelID        string   `json:"channelId"`
	Author           string   `json:"author"`
	ShortDescription string   `json:"shortDescription"`
	Thumbnail        YTThumb  `json:"thumbnail"`
}

type YTThumb struct {
	Thumbnails []YTImage `json:"thumbnails"`
}

type YTImage struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}
