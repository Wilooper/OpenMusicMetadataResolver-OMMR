package artists

import "time"

type Image struct {
	URL    string `json:"url"`
	Width  int64  `json:"width,omitempty"`
	Height int64  `json:"height,omitempty"`
}

type Metric struct {
	Name         string `json:"name"`
	Label        string `json:"label"`
	Value        *int64 `json:"value,omitempty"`
	DisplayValue string `json:"display_value"`
	Period       string `json:"period"`
	Scope        string `json:"scope"`
	SourceField  string `json:"source_field"`
}

type Counts struct {
	Tracks     *int64 `json:"total_tracks,omitempty"`
	Albums     *int64 `json:"total_albums,omitempty"`
	Recordings *int64 `json:"total_recordings,omitempty"`
}

type Track struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	URL        string   `json:"url,omitempty"`
	Artists    []string `json:"artists,omitempty"`
	Album      string   `json:"album,omitempty"`
	DurationMS int64    `json:"duration_ms,omitempty"`
	Images     []Image  `json:"images,omitempty"`
	Metrics    []Metric `json:"metrics,omitempty"`
}

type Reference struct {
	Provider       string `json:"provider"`
	ID             string `json:"id"`
	Name           string `json:"name"`
	URL            string `json:"url,omitempty"`
	Disambiguation string `json:"disambiguation,omitempty"`
}

type Profile struct {
	Reference
	ObservedAt     time.Time      `json:"observed_at,omitzero"`
	IdentityMethod string         `json:"identity_method"`
	Biography      string         `json:"biography,omitempty"`
	Images         []Image        `json:"images,omitempty"`
	Metrics        []Metric       `json:"metrics,omitempty"`
	Counts         Counts         `json:"counts"`
	TopTracks      []Track        `json:"top_tracks,omitempty"`
	RecentTracks   []Track        `json:"recent_tracks,omitempty"`
	CatalogTracks  []Track        `json:"catalog_tracks,omitempty"`
	ProviderData   map[string]any `json:"provider_data,omitempty"`
}

type SourceStatus struct {
	Provider string `json:"provider"`
	Status   string `json:"status"`
	Error    string `json:"error,omitempty"`
	Cached   bool   `json:"cached"`
	Partial  bool   `json:"partial,omitempty"`
}

type Result struct {
	Profiles   []Profile      `json:"profiles"`
	Candidates []Reference    `json:"candidates,omitempty"`
	Sources    []SourceStatus `json:"sources,omitempty"`
}

type Query struct {
	Artist      string
	Provider    string
	ID          string
	Sources     []string
	Limit       int
	BypassCache bool
}
