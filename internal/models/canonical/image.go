package canonical

// Image represents artwork or image assets associated with a track, album, or artist.
type Image struct {
	URL    string `json:"url"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	Type   string `json:"type,omitempty"` // e.g. "cover", "artist", "thumbnail"
}
