package canonical

// Copyright represents a copyright or phonographic copyright notice for a track or album.
type Copyright struct {
	Type  string `json:"type,omitempty"` // e.g. "C", "P"
	Text  string `json:"text"`
	Label string `json:"label,omitempty"`
}
