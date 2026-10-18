> This document describes the full internal canonical schema, returned by data endpoints with `debug=true`. Default production responses use an explicit metadata-only projection; see [API_SPEC.md](API_SPEC.md#production-and-debug-responses). Internal Track JSON and cache entries are unchanged.

# OMMR Domain Schema Specification

## Canonical Domain Schemas (`internal/models/canonical/`)

### Track Entity (`Track`)

The `Track` entity represents the unified canonical track metadata object.

`extensions.wikipedia` is present only when a MusicBrainz work relationship names a validated English Wikipedia article or a Wikidata entity with an English sitelink. It contains `title`, optional `description` and `summary`, `article_url`, and `source` (`musicbrainz_work_wikipedia_link` or `musicbrainz_work_wikidata_link`). It is contextual prose, not a contributor credit or platform ID. See [field definitions](METADATA_CATALOG.md).
`extensions.musicbrainz_work_id` is a composition ID. MusicBrainz work relations can also supply `iswc` and `language`; these remain separate from recording ISRC and release metadata.

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
    ISWC            string                     `json:"iswc,omitempty"`   // International Standard Musical Work Code
    Genres          []string                   `json:"genres"`
    Language        string                     `json:"language,omitempty"`
    Credits         []Credit                   `json:"credits"`
    Images          []Image                    `json:"images"`
    PreviewURL      string                     `json:"preview_url,omitempty"` // 30s audio preview (Spotify/Apple/Deezer)
    TrackNumber     int                        `json:"track_number,omitempty"`
    DiscNumber      int                        `json:"disc_number,omitempty"`
    Label           string                     `json:"label,omitempty"`
    Barcode         string                     `json:"barcode,omitempty"`
    Copyrights      []Copyright                `json:"copyrights,omitempty"`
    PlayCount       int64                      `json:"play_count,omitempty"` // Provider-reported play/rank count
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

#### Copyright (`Copyright`)
```go
type Copyright struct {
    Text string `json:"text"`
    Type string `json:"type"` // "P" (phonogram) or "C" (copyright)
}
```

#### Album Entity (`Album`)
The `Album` entity additionally carries `Label`, `UPC`, `Barcode`, `TotalTracks`, and `Copyrights`.

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
    SoundCloudID  string          `json:"soundcloud_id,omitempty"`
    JioSaavnID    string          `json:"jiosaavn_id,omitempty"`
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
    EnrichmentStatus string `json:"enrichment_status,omitempty"`
    CandidateScore  float64 `json:"candidate_score,omitempty"`
    Version         string  `json:"version"`
}
```

#### Resolver Statistics (`ResolverStats`)

`enrichment_status` is present for accepted candidates with an enrichment adapter:
`ok`, `unavailable`, or `identity_mismatch`. Enrichment failure preserves the
accepted candidate; it cannot add IDs or change the matched recording. Source
latency includes the enrichment request and its rate-limit wait.

```go
type ResolverStats struct {
    ProvidersQueried     int `json:"providers_queried"`
    ProvidersMatched     int `json:"providers_matched"`
    ProvidersContributed int `json:"providers_contributed"`
    CandidatesEvaluated  int `json:"candidates_evaluated"`
}
```


### Platform link extension

`Track` remains unchanged. `extensions.platform_links` contains `ommr_id`, `isrc`,
`platforms`, and `unavailable_platforms`. Each platform entry contains `id`,
`id_type`, optional `lookup_id`, `url`, `url_source`, `confidence`, and
`match_method`. See [PLATFORM_LINKS.md](PLATFORM_LINKS.md). Unknown identifiers
and URLs are omitted. Only a provider's own ID is emitted as an IdentityMatch;
provider `_url` and `_track_id` helpers in `ids` do not assert extra matches.

## Additive metadata extensions and public projection

The immutable internal `Track` stores accepted source records in `extensions.provider_data` and explicit evidence state in `extensions.explicit_status`. Production promotes these to `provider_data` and `explicit_status`. Canonical matching/provenance diagnostics remain debug-only. The public provider-data schema and source scopes are documented in [ENRICHMENT.md](ENRICHMENT.md); new catalog fields do not require changing Track's Go fields.

## Provider lifecycle status

`/v1/providers` is an operational schema independent of canonical metadata. It contains name/state/mode, startup credential outcome, failure streak, last evaluation and retry time. Credentials and account profile details are never included. API policy and safe status meanings are defined in [PROVIDER_LIFECYCLE.md](PROVIDER_LIFECYCLE.md).

## Debug metadata comparison

`track.extensions.metadata_comparison` is an optional debug-only map keyed by canonical field name. Each entry contains `selected`, `selected_sources`, `values` (provider to original reported value) and `different`. It compares populated evidence only; unknown explicit defaults do not vote. Representational/precision differences are retained rather than treated as automatic rejection. This extension is omitted by production DTOs. Canonical artists may now use an evidence-backed separate list while source-specific combined displays remain in provider data. Native person IDs are never copied from a compound display to each person.

## Rich public catalogs

See [RICH_CATALOGS.md](RICH_CATALOGS.md) for native Bugs/Genie/Melon song adapters, `bugs_id`/`genie_id`/`melon_id`, accepted YouTube artist/album context in provider data, optional Python pooling and `/v1/catalog` (also `operation: catalog` through the JSON gateway). These Korean adapters require no credentials; the optional Python path currently uses public YouTube access.

Artist profiles use an independent source-owned schema rather than the canonical Track schema. See [artist profile fields and metric periods](ARTIST_PROFILES.md#fields); `internal/artists/models.go` defines the wire types. Metrics expose exact values only when reported and preserve display text/time scope.
