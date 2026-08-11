# OMMR Domain Schema Specification

## Canonical Domain Schemas (`internal/models/canonical/`)

### Track Entity (`Track`)

The `Track` entity represents the unified canonical track metadata object.

```go
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
    Genres          []string                   `json:"genres"`
    Language        string                     `json:"language,omitempty"`
    Credits         []Credit                   `json:"credits"`
    Images          []Image                    `json:"images"`
    IDs             map[string]string          `json:"ids"`
    IdentityMatches []IdentityMatch            `json:"identity_matches,omitempty"`
    Sources         []string                   `json:"sources"`
    FieldSources    map[string][]string        `json:"field_sources,omitempty"`
    MatchScore      float64                    `json:"match_score"`
    MatchBreakdown  MatchBreakdown             `json:"match_breakdown"`
    Confidence      Confidence                 `json:"confidence"`
    Completeness    float64                    `json:"completeness"`
    Relations       Relations                  `json:"relations"`
    Extensions      map[string]json.RawMessage `json:"extensions,omitempty"`
}
```

---

### Identity Entities

#### Identity Match (`IdentityMatch`)
```go
type IdentityMatch struct {
    Provider   string  `json:"provider"`
    ID         string  `json:"id"`
    Confidence float64 `json:"confidence"`
    Method     string  `json:"method"` // "isrc", "exact_title_artist", "title_artist_duration", "album_match"
}
```

#### Identity Graph (`IdentityGraph`)
```go
type IdentityGraph struct {
    CanonicalID   string          `json:"canonical_id"`
    SpotifyID     string          `json:"spotify_id,omitempty"`
    YouTubeID     string          `json:"youtube_id,omitempty"`
    AppleMusicID  string          `json:"applemusic_id,omitempty"`
    DeezerID      string          `json:"deezer_id,omitempty"`
    MusicBrainzID string          `json:"musicbrainz_id,omitempty"`
    ISRC          string          `json:"isrc,omitempty"`
    Matches       []IdentityMatch `json:"matches"`
}
```

---

### Diagnostics & Support Schemas

#### Nullable Match Breakdown (`MatchBreakdown`)
```go
type MatchBreakdown struct {
    Title       *float64 `json:"title,omitempty"`
    Artists     *float64 `json:"artists,omitempty"`
    Album       *float64 `json:"album,omitempty"`
    Duration    *float64 `json:"duration,omitempty"`
    ReleaseDate *float64 `json:"release_date,omitempty"`
    ISRC        *float64 `json:"isrc,omitempty"`
}
```

#### Source Status (`SourceStatus`)
```go
type SourceStatus struct {
    Name            string  `json:"name"`
    Success         bool    `json:"success"`
    Matched         bool    `json:"matched"`
    Contributed     bool    `json:"contributed"`
    Cached          bool    `json:"cached"`
    LatencyMS       int64   `json:"latency_ms"`
    Error           string  `json:"error,omitempty"`
    RejectionReason string  `json:"rejection_reason,omitempty"`
    CandidateScore  float64 `json:"candidate_score,omitempty"`
    Version         string  `json:"version"`
}
```

#### Resolver Statistics (`ResolverStats`)
```go
type ResolverStats struct {
    ProvidersQueried     int `json:"providers_queried"`
    ProvidersMatched     int `json:"providers_matched"`
    ProvidersContributed int `json:"providers_contributed"`
    CandidatesEvaluated  int `json:"candidates_evaluated"`
}
```
