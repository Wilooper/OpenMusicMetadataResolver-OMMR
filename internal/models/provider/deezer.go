package provider

// DeezerTrackResponse represents the raw JSON response from Deezer API /track/{id}.
type DeezerTrackResponse struct {
	ID                    int64         `json:"id"`
	Readable              bool          `json:"readable"`
	Title                 string        `json:"title"`
	TitleShort            string        `json:"title_short"`
	TitleVersion          string        `json:"title_version"`
	ISRC                  string        `json:"isrc"`
	Link                  string        `json:"link"`
	Duration              int64         `json:"duration"` // in seconds
	Rank                  int           `json:"rank"`
	ExplicitLyrics        bool          `json:"explicit_lyrics"`
	ExplicitContentLyrics int           `json:"explicit_content_lyrics"`
	ExplicitContentCover  int           `json:"explicit_content_cover"`
	ReleaseDate           string        `json:"release_date"`
	Artist                DeezerArtist  `json:"artist"`
	Album                 DeezerAlbum   `json:"album"`
	Contributors          []DeezerArtist `json:"contributors"`
}

type DeezerArtist struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Link      string `json:"link"`
	Picture   string `json:"picture"`
	PictureXL string `json:"picture_xl"`
	Role      string `json:"role"`
}

type DeezerAlbum struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Cover       string `json:"cover"`
	CoverMedium string `json:"cover_medium"`
	CoverBig    string `json:"cover_big"`
	CoverXL     string `json:"cover_xl"`
	ReleaseDate string `json:"release_date"`
}

// DeezerSearchResponse represents raw search response from /search.
type DeezerSearchResponse struct {
	Data  []DeezerTrackResponse `json:"data"`
	Total int                   `json:"total"`
}
