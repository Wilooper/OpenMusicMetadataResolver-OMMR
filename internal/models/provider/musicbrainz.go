package provider

// MusicBrainzSearchResponse represents the JSON response from MusicBrainz recording query API.
type MusicBrainzSearchResponse struct {
	Created    string                 `json:"created"`
	Count      int                    `json:"count"`
	Offset     int                    `json:"offset"`
	Recordings []MusicBrainzRecording `json:"recordings"`
}

type MusicBrainzRecording struct {
	ID             string                    `json:"id"`
	Score          int                       `json:"score"`
	Title          string                    `json:"title"`
	Length         int64                     `json:"length"` // in milliseconds
	Disambiguation string                    `json:"disambiguation"`
	ISRCs          []string                  `json:"isrcs"`
	ArtistCredit   []MusicBrainzArtistCredit `json:"artist-credit"`
	Releases       []MusicBrainzRelease      `json:"releases"`
	Tags           []MusicBrainzTag          `json:"tags"`
	Relations      []MusicBrainzRelation     `json:"relations"`
}

type MusicBrainzRelation struct {
	Type   string            `json:"type"`
	Artist MusicBrainzArtist `json:"artist"`
	URL    struct {
		Resource string `json:"resource"`
	} `json:"url"`
	Work struct {
		ID        string                `json:"id"`
		ISWCs     []string              `json:"iswcs"`
		Language  string                `json:"language"`
		Relations []MusicBrainzRelation `json:"relations"`
	} `json:"work"`
}

type MusicBrainzArtistCredit struct {
	Name   string            `json:"name"`
	Artist MusicBrainzArtist `json:"artist"`
}

type MusicBrainzArtist struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	SortName       string `json:"sort-name"`
	Disambiguation string `json:"disambiguation"`
}

type MusicBrainzRelease struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Date    string `json:"date"`
	Country string `json:"country"`
	Status  string `json:"status"`
}

type MusicBrainzTag struct {
	Count int    `json:"count"`
	Name  string `json:"name"`
}
