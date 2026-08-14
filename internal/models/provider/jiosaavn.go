package provider

import (
	"encoding/json"
	"strconv"
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
	Total   int                  `json:"total"`
	Start   int                  `json:"start"`
	Results []JioSaavnSongResult `json:"results"`
}

// JioSaavnSongResult represents a single track result from the JioSaavn search API.
type JioSaavnSongResult struct {
	ID              string     `json:"id"`
	Type            string     `json:"type"`
	Song            string     `json:"song"`
	Album           string     `json:"album"`
	AlbumID         JSONString `json:"albumid"`
	Year            JSONString `json:"year"`
	Music           string     `json:"music"`
	PrimaryArtists  string     `json:"primary_artists"`
	FeaturedArtists string     `json:"featured_artists"`
	Singers         string     `json:"singers"`
	Starring        string     `json:"starring"`
	Image           string     `json:"image"`
	Label           string     `json:"label"`
	Language        string     `json:"language"`
	PlayCount       JSONString `json:"play_count"`
	PermaURL        string     `json:"perma_url"`
	Duration        JSONString `json:"duration"`
	ReleaseDate     string     `json:"release_date"`
	ExplicitContent JSONString `json:"explicit_content"`
	CopyrightText   string     `json:"copyright_text"`
}

func (r JioSaavnSongResult) PlayCountInt() int64 {
	n, _ := strconv.ParseInt(string(r.PlayCount), 10, 64)
	return n
}

func (r JioSaavnSongResult) DurationSeconds() int {
	n, _ := strconv.Atoi(string(r.Duration))
	return n
}
