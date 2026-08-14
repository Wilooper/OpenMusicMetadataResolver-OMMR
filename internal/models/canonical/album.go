package canonical

// Album represents album metadata associated with a track.
type Album struct {
	Title       string            `json:"title"`
	ReleaseDate string            `json:"release_date,omitempty"`
	Type        string            `json:"type,omitempty"` // e.g. "album", "single", "ep", "compilation"
	Label       string            `json:"label,omitempty"`
	UPC         string            `json:"upc,omitempty"`
	Barcode     string            `json:"barcode,omitempty"`
	TotalTracks int               `json:"total_tracks,omitempty"`
	Copyrights  []Copyright       `json:"copyrights,omitempty"`
	Images      []Image           `json:"images,omitempty"`
	IDs         map[string]string `json:"ids,omitempty"` // Provider -> Album ID mapping
}
