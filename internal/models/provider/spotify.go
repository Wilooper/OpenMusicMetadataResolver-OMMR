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

// SpotifyWebTokenResponse represents the Spotify Accounts client-credentials token response.
type SpotifyWebTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// SpotifyAnonymousTokenResponse represents the anonymous token granted by the public web player endpoint.
type SpotifyAnonymousTokenResponse struct {
	ClientID                         string `json:"clientId"`
	AccessToken                      string `json:"accessToken"`
	AccessTokenExpirationTimestampMs int64  `json:"accessTokenExpirationTimestampMs"`
	IsAnonymous                      bool   `json:"isAnonymous"`
}

// SpotifyWebTrack represents the Spotify Web API track object (official + anonymous token modes).
type SpotifyWebTrack struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	Artists      []SpotifyWebArtist `json:"artists"`
	Album        SpotifyWebAlbum    `json:"album"`
	DurationMS   int64              `json:"duration_ms"`
	Explicit     bool               `json:"explicit"`
	ExternalIDs  SpotifyExternalIDs `json:"external_ids"`
	Popularity   int                `json:"popularity"`
	PreviewURL   string             `json:"preview_url"`
	TrackNumber  int                `json:"track_number"`
	DiscNumber   int                `json:"disc_number"`
	ISRC         string             `json:"isrc"`
	Restrictions struct {
		Reason string `json:"reason"`
	} `json:"restrictions"`
	ExternalURLs struct {
		Spotify string `json:"spotify"`
	} `json:"external_urls"`
}

type SpotifyWebArtist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type SpotifyExternalIDs struct {
	ISRC string `json:"isrc"`
}

type SpotifyImage struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type SpotifyWebAlbum struct {
	ID                   string             `json:"id"`
	Name                 string             `json:"name"`
	AlbumType            string             `json:"album_type"`
	ReleaseDate          string             `json:"release_date"`
	ReleaseDatePrecision string             `json:"release_date_precision"`
	Label                string             `json:"label"`
	UPC                  string             `json:"upc"`
	TotalTracks          int                `json:"total_tracks"`
	Images               []SpotifyImage     `json:"images"`
	Copyrights           []SpotifyCopyright `json:"copyrights"`
}

type SpotifyCopyright struct {
	Text string `json:"text"`
	Type string `json:"type"`
}

// SpotifyWebSearchResponse represents the Spotify Web API /v1/search?type=track response.
type SpotifyWebSearchResponse struct {
	Tracks struct {
		Items []SpotifyWebTrack `json:"items"`
	} `json:"tracks"`
}

// SpotifyArtistWebResponse represents the Spotify Web API artist object (used for genre enrichment).
type SpotifyArtistWebResponse struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Genres     []string `json:"genres"`
	Popularity int      `json:"popularity"`
}

// SpotifyEmbedNextData represents the __NEXT_DATA__ JSON payload embedded in open.spotify.com/embed/track/{id}.
type SpotifyEmbedNextData struct {
	Props SpotifyEmbedProps `json:"props"`
}

type SpotifyEmbedProps struct {
	PageProps SpotifyEmbedPageProps `json:"pageProps"`
}

type SpotifyEmbedPageProps struct {
	State SpotifyEmbedState `json:"state"`
}

type SpotifyEmbedState struct {
	Data SpotifyEmbedData `json:"data"`
}

type SpotifyEmbedData struct {
	Entity SpotifyEmbedEntity `json:"entity"`
}

type SpotifyEmbedEntity struct {
	Name        string               `json:"name"`
	URI         string               `json:"uri"`
	ID          string               `json:"id"`
	Duration    int64                `json:"duration"`
	DurationMS  int64                `json:"duration_ms"`
	Explicit    bool                 `json:"explicit"`
	ISRC        string               `json:"isrc"`
	TrackNumber int                  `json:"trackNumber"`
	DiscNumber  int                  `json:"discNumber"`
	PreviewURL  string               `json:"preview_url"`
	Artists     []SpotifyEmbedArtist `json:"artists"`
	Album       SpotifyEmbedAlbum    `json:"album"`
}

type SpotifyEmbedArtist struct {
	Name string `json:"name"`
	ID   string `json:"id"`
	URI  string `json:"uri"`
}

type SpotifyEmbedAlbum struct {
	Name        string             `json:"name"`
	ID          string             `json:"id"`
	Images      []SpotifyImage     `json:"images"`
	ReleaseDate SpotifyEmbedDate   `json:"releaseDate"`
	TotalTracks int                `json:"totalTracks"`
	Label       string             `json:"label"`
	Copyright   []SpotifyCopyright `json:"copyright"`
}

type SpotifyEmbedDate struct {
	ISOString string `json:"isoString"`
}
