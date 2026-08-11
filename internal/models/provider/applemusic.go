package provider

// AppleSearchResponse represents raw JSON response from iTunes Search/Lookup API.
type AppleSearchResponse struct {
	ResultCount int                `json:"resultCount"`
	Results     []AppleTrackResult `json:"results"`
}

type AppleTrackResult struct {
	WrapperType            string `json:"wrapperType"`
	Kind                   string `json:"kind"`
	ArtistID               int64  `json:"artistId"`
	CollectionID           int64  `json:"collectionId"`
	TrackID                int64  `json:"trackId"`
	ArtistName             string `json:"artistName"`
	CollectionName         string `json:"collectionName"`
	TrackName              string `json:"trackName"`
	CollectionCensoredName string `json:"collectionCensoredName"`
	TrackCensoredName      string `json:"trackCensoredName"`
	ArtistViewURL          string `json:"artistViewUrl"`
	CollectionViewURL      string `json:"collectionViewUrl"`
	TrackViewURL           string `json:"trackViewUrl"`
	ArtworkUrl30           string `json:"artworkUrl30"`
	ArtworkUrl60           string `json:"artworkUrl60"`
	ArtworkUrl100          string `json:"artworkUrl100"`
	ReleaseDate            string `json:"releaseDate"`
	CollectionExplicitness string `json:"collectionExplicitness"`
	TrackExplicitness      string `json:"trackExplicitness"`
	TrackTimeMillis        int64  `json:"trackTimeMillis"`
	Country                string `json:"country"`
	Currency               string `json:"currency"`
	PrimaryGenreName       string `json:"primaryGenreName"`
	ISRC                   string `json:"isrc,omitempty"`
}
