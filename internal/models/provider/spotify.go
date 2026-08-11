package provider

// SpotifyOembedResponse represents Spotify oEmbed endpoint response.
type SpotifyOembedResponse struct {
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

// SpotifyTrackWebResponse represents Spotify Web API track object.
type SpotifyTrackWebResponse struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Artists     []SpotifyArtistSimple `json:"artists"`
	Album       SpotifyAlbumSimple    `json:"album"`
	DurationMS  int64                  `json:"duration_ms"`
	Explicit    bool                   `json:"explicit"`
	ExternalIDs SpotifyExternalIDs     `json:"external_ids"`
	Populatity  int                    `json:"popularity"`
}

type SpotifyArtistSimple struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URI  string `json:"uri"`
}

type SpotifyAlbumSimple struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	ReleaseDate string         `json:"release_date"`
	Images      []SpotifyImage `json:"images"`
}

type SpotifyExternalIDs struct {
	ISRC string `json:"isrc"`
}

type SpotifyImage struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}
