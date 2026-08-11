package canonical

// Album represents album metadata associated with a track.
type Album struct {
	Title       string            `json:"title"`
	ReleaseDate string            `json:"release_date,omitempty"`
	Type        string            `json:"type,omitempty"` // e.g. "album", "single", "ep", "compilation"
	Images      []Image           `json:"images,omitempty"`
	IDs         map[string]string `json:"ids,omitempty"` // Provider -> Album ID mapping
}
