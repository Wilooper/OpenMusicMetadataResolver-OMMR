package provider

// KoreanRecord is one public catalog entity, with source-labelled detail fields.
type KoreanRecord struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Artists     []KoreanArtist    `json:"artists"`
	AlbumID     string            `json:"album_id,omitempty"`
	Album       string            `json:"album,omitempty"`
	Image       string            `json:"image,omitempty"`
	DurationMS  int64             `json:"duration_ms,omitempty"`
	ReleaseDate string            `json:"release_date,omitempty"`
	ISRC        string            `json:"isrc,omitempty"`
	Fields      map[string]string `json:"fields,omitempty"`
	Structured  map[string]any    `json:"structured,omitempty"`
}
type KoreanArtist struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
}
