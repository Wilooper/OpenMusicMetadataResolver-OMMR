package canonical

import "encoding/json"

// Confidence levels
const (
	ConfidenceVerified = "verified" // ISRC matched across 2+ distinct providers
	ConfidenceExact    = "exact"    // match_score >= 0.95
	ConfidenceHigh     = "high"     // match_score >= 0.85
	ConfidenceMedium   = "medium"   // match_score >= 0.70
	ConfidenceLow      = "low"      // match_score < 0.70
)

// IdentityStatus classifications
const (
	IdentityStatusVerified = "verified" // ISRC matched across 2+ providers or MBID verified with high score
	IdentityStatusStrong   = "strong"   // match_score >= 0.90 with multi-provider ID consensus
	IdentityStatusProbable = "probable" // match_score >= 0.75
	IdentityStatusWeak     = "weak"     // match_score < 0.75
)

// Confidence encapsulates structured match confidence level and score.
type Confidence struct {
	Level string  `json:"level"`
	Score float64 `json:"score"`
}

// IdentityMatch encapsulates a discovered platform ID, calculated confidence, and discovery method.
type IdentityMatch struct {
	Provider   string  `json:"provider"`
	ID         string  `json:"id"`
	Confidence float64 `json:"confidence"`
	Method     string  `json:"method"` // e.g. "isrc", "exact_title_artist", "title_artist_duration", "album_match"
}

// MatchBreakdown details component similarity scores (nil if field un-evaluated/missing).
type MatchBreakdown struct {
	Title       *float64 `json:"title,omitempty"`
	Artists     *float64 `json:"artists,omitempty"`
	Album       *float64 `json:"album,omitempty"`
	Duration    *float64 `json:"duration,omitempty"`
	ReleaseDate *float64 `json:"release_date,omitempty"`
	ISRC        *float64 `json:"isrc,omitempty"`
}

// SearchResult represents a lightweight canonical track object returned by GET /v1/search.
type SearchResult struct {
	CanonicalID     string     `json:"canonical_id"`
	Title           string     `json:"title"`
	Artists         []Artist   `json:"artists"`
	Album           string     `json:"album"`
	Images          []Image    `json:"images"`
	MatchScore      float64    `json:"match_score"`
	Confidence      Confidence `json:"confidence"`
	IdentityStatus  string     `json:"identity_status"`
	IdentityReasons []string   `json:"identity_reasons,omitempty"`
	SourceCount     int        `json:"source_count"`
	AvailableOn     []string   `json:"available_on"`
}

// Track represents the unified canonical track metadata entity (IMMUTABLE SCHEMA).
type Track struct {
	CanonicalID     string                     `json:"canonical_id"`
	IdentityStatus  string                     `json:"identity_status"`
	IdentityReasons []string                   `json:"identity_reasons,omitempty"`
	Title           string                     `json:"title"`
	Artists         []Artist                   `json:"artists"`
	Album           Album                      `json:"album"`
	DurationMS      int64                      `json:"duration_ms"`
	ReleaseDate     string                     `json:"release_date"`
	Explicit        bool                       `json:"explicit"`
	ISRC            string                     `json:"isrc,omitempty"`
	ISWC            string                     `json:"iswc,omitempty"` // International Standard Musical Work Code
	Genres          []string                   `json:"genres"`
	Language        string                     `json:"language,omitempty"`
	Credits         []Credit                   `json:"credits"`
	Images          []Image                    `json:"images"`
	PreviewURL      string                     `json:"preview_url,omitempty"` // 30s audio preview (e.g. Spotify/Apple)
	TrackNumber     int                        `json:"track_number,omitempty"`
	DiscNumber      int                        `json:"disc_number,omitempty"`
	Label           string                     `json:"label,omitempty"`
	Barcode         string                     `json:"barcode,omitempty"`
	Copyrights      []Copyright                `json:"copyrights,omitempty"`
	PlayCount       int64                      `json:"play_count,omitempty"` // Provider-reported play/rank count
	IDs             map[string]string          `json:"ids"`                  // Provider name -> Track/Video ID
	IdentityMatches []IdentityMatch            `json:"identity_matches,omitempty"`
	Sources         []string                   `json:"sources"` // List of contributing provider names
	FieldSources    map[string][]string        `json:"field_sources,omitempty"`
	MatchScore      float64                    `json:"match_score"`
	MatchBreakdown  MatchBreakdown             `json:"match_breakdown"`
	Confidence      Confidence                 `json:"confidence"`
	Completeness    float64                    `json:"completeness"`
	Relations       Relations                  `json:"relations"`
	Extensions      map[string]json.RawMessage `json:"extensions,omitempty"`
}

// GetConfidenceLevel returns confidence level string given a score and source ISRC count.
func GetConfidenceLevel(score float64, isrcMatchCount int) string {
	if isrcMatchCount >= 2 {
		return ConfidenceVerified
	}
	switch {
	case score >= 0.95:
		return ConfidenceExact
	case score >= 0.85:
		return ConfidenceHigh
	case score >= 0.70:
		return ConfidenceMedium
	default:
		return ConfidenceLow
	}
}

// GetIdentityStatus calculates identity status level string and reasons given track properties.
func GetIdentityStatus(score float64, isrcMatchCount int, hasMBID bool, hasISRC bool) (string, []string) {
	reasons := make([]string, 0, 4)

	if hasISRC {
		reasons = append(reasons, "isrc_match")
	}
	if hasMBID {
		reasons = append(reasons, "musicbrainz_match")
	}
	if score >= 0.90 {
		reasons = append(reasons, "exact_title_artist_match")
	}
	if isrcMatchCount >= 2 {
		reasons = append(reasons, "multi_provider_isrc_consensus")
	}

	if score < 0.75 {
		return IdentityStatusWeak, reasons
	}
	if score < 0.85 {
		return IdentityStatusProbable, reasons
	}
	if isrcMatchCount >= 2 || (hasMBID && hasISRC) {
		return IdentityStatusVerified, reasons
	}
	if score >= 0.90 {
		return IdentityStatusStrong, reasons
	}
	return IdentityStatusProbable, reasons
}

// CalculateCompleteness evaluates metadata field population completeness score [0.0 - 1.0].
func CalculateCompleteness(t Track) float64 {
	score := 0.0
	if t.Title != "" {
		score += 0.12
	}
	if len(t.Artists) > 0 {
		score += 0.12
	}
	if t.ISRC != "" {
		score += 0.12
	}
	if t.Album.Title != "" {
		score += 0.08
	}
	if t.DurationMS > 0 {
		score += 0.08
	}
	if len(t.Images) > 0 {
		score += 0.08
	}
	if len(t.Genres) > 0 {
		score += 0.07
	}
	if len(t.Credits) > 0 {
		score += 0.07
	}
	if t.ReleaseDate != "" {
		score += 0.05
	}
	if t.PreviewURL != "" {
		score += 0.05
	}
	if t.Label != "" {
		score += 0.04
	}
	if t.TrackNumber > 0 {
		score += 0.03
	}
	if len(t.Copyrights) > 0 {
		score += 0.03
	}
	if t.ISWC != "" {
		score += 0.03
	}
	if t.PlayCount > 0 {
		score += 0.03
	}
	if score > 1.0 {
		score = 1.0
	}
	return score
}
