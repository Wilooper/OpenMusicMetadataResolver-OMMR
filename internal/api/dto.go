package api

import (
	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/platformlinks"
)

type linksResponse struct {
	Links          platformlinks.Catalog    `json:"links"`
	Title          string                   `json:"title,omitempty"`
	Artists        []canonical.Artist       `json:"artists,omitempty"`
	IdentityStatus string                   `json:"identity_status,omitempty"`
	ProviderStatus []canonical.SourceStatus `json:"provider_status"`
}

// ResolutionStrategy provides transparency into how a query was resolved.
type ResolutionStrategy struct {
	InputType     string `json:"input_type"`     // e.g. "youtube_id", "spotify_id", "artist_title"
	PrimarySource string `json:"primary_source"` // e.g. "ytmusic", "musicbrainz"
	CrossResolved bool   `json:"cross_resolved"`
}

// BulkResolveRequest payload for POST /v1/bulk.
type BulkResolveRequest struct {
	Queries []adapters.Query `json:"queries"`
	Sources []string         `json:"sources,omitempty"`
}

// BulkItemResult single item result in POST /v1/bulk response.
type BulkItemResult struct {
	QueryIndex int                      `json:"query_index"`
	Success    bool                     `json:"success"`
	Error      string                   `json:"error,omitempty"`
	Track      *canonical.Track         `json:"track,omitempty"`
	Sources    []canonical.SourceStatus `json:"sources,omitempty"`
}

// BulkResolveResponse payload returned by POST /v1/bulk.
type BulkResolveResponse struct {
	Results    []BulkItemResult `json:"results"`
	Total      int              `json:"total"`
	Successful int              `json:"successful"`
	Failed     int              `json:"failed"`
}

// SearchResponse payload returned by GET /v1/search.
type SearchResponse struct {
	Query   string                   `json:"query"`
	Results []canonical.SearchResult `json:"results"`
	Page    int                      `json:"page"`
	Limit   int                      `json:"limit"`
	Total   int                      `json:"total"`
}

// HealthResponse payload returned by GET /v1/health.
type HealthResponse struct {
	Status        string `json:"status"`
	Version       string `json:"version"`
	UptimeSeconds int64  `json:"uptime_seconds"`
	CacheProvider string `json:"cache_provider"`
}
