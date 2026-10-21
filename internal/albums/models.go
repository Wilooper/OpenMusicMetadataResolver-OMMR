package albums

import (
	"github.com/ommr/ommr/internal/musicgraph"
	"time"
)

const MaxTracks = 1000

type Query struct {
	Provider    string
	ID          string
	Limit       int
	BypassCache bool
}
type Artist struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name"`
	Role       string `json:"role,omitempty"`
	JoinPhrase string `json:"join_phrase,omitempty"`
}
type Track struct {
	ID           string         `json:"id"`
	RecordingID  string         `json:"recording_id,omitempty"`
	Title        string         `json:"title"`
	Position     int            `json:"position"`
	DiscNumber   int            `json:"disc_number,omitempty"`
	TrackNumber  string         `json:"track_number,omitempty"`
	DurationMS   int64          `json:"duration_ms,omitempty"`
	ISRC         string         `json:"isrc,omitempty"`
	Artists      []Artist       `json:"artists,omitempty"`
	ProviderData map[string]any `json:"provider_data,omitempty"`
}
type Disc struct {
	TrackList    *TrackList     `json:"track_list,omitempty"`
	ProviderData map[string]any `json:"provider_data,omitempty"`
	Number       int            `json:"number"`
	Title        string         `json:"title,omitempty"`
	Format       string         `json:"format,omitempty"`
	Total        *int           `json:"total_tracks,omitempty"`
}
type TrackList struct {
	Returned int    `json:"returned"`
	Total    *int   `json:"total,omitempty"`
	Complete bool   `json:"complete"`
	Reason   string `json:"reason,omitempty"`
}
type Album struct {
	MusicGraph   *musicgraph.Graph `json:"music_graph,omitempty"`
	Provider     string            `json:"provider"`
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	URL          string            `json:"url,omitempty"`
	Artists      []Artist          `json:"artists,omitempty"`
	ReleaseDate  string            `json:"release_date,omitempty"`
	Label        string            `json:"label,omitempty"`
	UPC          string            `json:"upc,omitempty"`
	Copyright    string            `json:"copyright,omitempty"`
	Language     string            `json:"language,omitempty"`
	Images       []string          `json:"images,omitempty"`
	Discs        []Disc            `json:"discs,omitempty"`
	Tracks       []Track           `json:"tracks"`
	TrackList    TrackList         `json:"track_list"`
	ProviderData map[string]any    `json:"provider_data,omitempty"`
	ObservedAt   time.Time         `json:"observed_at"`
}
type Diagnostics struct {
	Status string `json:"status"`
	Cached bool   `json:"cached"`
	Error  string `json:"error,omitempty"`
}
type Result struct {
	Album  *Album       `json:"album,omitempty"`
	Source *Diagnostics `json:"source,omitempty"`
}
