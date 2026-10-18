package provider

import (
	"encoding/json"
	"html"
	"strconv"
	"strings"
)

// JSONString tolerates JSON values that may be quoted strings or bare numbers
// (JioSaavn mixes both, e.g. "duration": 367 vs "play_count": "696888").
type JSONString string

func (j *JSONString) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*j = JSONString(s)
		return nil
	}
	*j = JSONString(string(b))
	return nil
}

// JioSaavnSearchResponse is the response of JioSaavn's public search.getResults API.
type JioSaavnSearchResponse struct {
	Total   JSONString           `json:"total"`
	Start   JSONString           `json:"start"`
	Results []JioSaavnSongResult `json:"results"`
}

// JioSaavnSongResult represents a single track result from the JioSaavn search API.
type JioSaavnSongResult struct {
	ID              string            `json:"id"`
	ArtistMap       JioSaavnArtistMap `json:"artistMap"`
	Type            string            `json:"type"`
	Song            string            `json:"song"`
	Album           string            `json:"album"`
	AlbumID         JSONString        `json:"albumid"`
	Year            JSONString        `json:"year"`
	Music           string            `json:"music"`
	PrimaryArtists  string            `json:"primary_artists"`
	FeaturedArtists string            `json:"featured_artists"`
	Singers         string            `json:"singers"`
	Starring        string            `json:"starring"`
	Image           string            `json:"image"`
	Label           string            `json:"label"`
	Language        string            `json:"language"`
	PlayCount       JSONString        `json:"play_count"`
	PermaURL        string            `json:"perma_url"`
	Duration        JSONString        `json:"duration"`
	ReleaseDate     string            `json:"release_date"`
	ExplicitContent JSONString        `json:"explicit_content"`
	CopyrightText   string            `json:"copyright_text"`
}

func (r JioSaavnSongResult) PlayCountInt() int64 {
	n, _ := strconv.ParseInt(string(r.PlayCount), 10, 64)
	return n
}

func (r JioSaavnSongResult) DurationSeconds() int {
	n, _ := strconv.Atoi(string(r.Duration))
	return n
}

// API v4 nests catalog fields in more_info; detail and recovered HTML records
// still use the original flat shape. Do not use subtitle as artist evidence.
type JioSaavnArtist struct {
	ID   JSONString `json:"id"`
	Name string     `json:"name"`
	Role string     `json:"role"`
}
type JioSaavnArtistMap struct {
	Primary  []JioSaavnArtist `json:"primary_artists"`
	Featured []JioSaavnArtist `json:"featured_artists"`
	All      []JioSaavnArtist `json:"artists"`
}

func (r *JioSaavnSongResult) UnmarshalJSON(data []byte) error {
	type flat JioSaavnSongResult
	var value struct {
		*flat
		Title    string `json:"title"`
		MoreInfo struct {
			Album       string            `json:"album"`
			AlbumID     JSONString        `json:"album_id"`
			Duration    JSONString        `json:"duration"`
			Label       string            `json:"label"`
			ReleaseDate string            `json:"release_date"`
			Copyright   string            `json:"copyright_text"`
			ArtistMap   JioSaavnArtistMap `json:"artistMap"`
		} `json:"more_info"`
	}
	// Reset when decoding into an existing value as well as a newly allocated one.
	*r = JioSaavnSongResult{}
	value.flat = (*flat)(r)
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if r.Song == "" {
		r.Song = value.Title
	}
	info := value.MoreInfo
	if r.Album == "" {
		r.Album = info.Album
	}
	if r.AlbumID == "" {
		r.AlbumID = info.AlbumID
	}
	if r.Duration == "" {
		r.Duration = info.Duration
	}
	if r.Label == "" {
		r.Label = info.Label
	}
	if r.ReleaseDate == "" {
		r.ReleaseDate = info.ReleaseDate
	}
	if r.CopyrightText == "" {
		r.CopyrightText = info.Copyright
	}
	if len(r.ArtistMap.Primary) == 0 && len(r.ArtistMap.Featured) == 0 && len(r.ArtistMap.All) == 0 {
		r.ArtistMap = info.ArtistMap
	}
	r.Song = html.UnescapeString(r.Song)
	r.Album = html.UnescapeString(r.Album)
	r.Label = html.UnescapeString(r.Label)
	r.PrimaryArtists = html.UnescapeString(r.PrimaryArtists)
	r.FeaturedArtists = html.UnescapeString(r.FeaturedArtists)
	r.Singers = html.UnescapeString(r.Singers)
	for _, list := range [][]JioSaavnArtist{r.ArtistMap.Primary, r.ArtistMap.Featured, r.ArtistMap.All} {
		for i := range list {
			list[i].Name = html.UnescapeString(strings.TrimSpace(list[i].Name))
		}
	}
	return nil
}
