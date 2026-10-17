# OMMR Provider Adapters

OMMR integrates 11 metadata provider adapters implementing the `adapters.ProviderAdapter` interface (`internal/adapters/adapter.go`). Public sources continue to work without credentials; TIDAL, Qobuz, Amazon Music, and Pandora integrations require provider-issued credentials or access.

---

## Adapter Summary

| Provider | Adapter Name | Version | Integration Method | API Keys | Rate Limits |
| --- | --- | --- | --- | --- | --- |
| **Spotify** | `spotify` | `spotify-web-v3` | Official Web API (optional keys) + anonymous Web API / oEmbed / embed scraping | **Optional** | Token Bucket (10 RPS) |
| **YouTube Music** | `ytmusic` | `ytmusic-innertube-v2` | InnerTube search & player (public API key) + oEmbed fallback | **Zero** | Token Bucket (20 RPS) |
| **Apple Music** | `applemusic` | `applemusic-v1` | iTunes Search & Lookup API | **Zero** | Keyless: approximately 20/minute, burst 1; token catalog: conservative 5 RPS |
| **Deezer** | `deezer` | `deezer-v1` | Public API (`api.deezer.com`) | **Zero** | Token Bucket (50 RPS) |
| **MusicBrainz** | `musicbrainz` | `musicbrainz-v2` | MusicBrainz REST API v2 | **Zero** | Token Bucket (1 RPS) |
| **SoundCloud** | `soundcloud` | `soundcloud-oembed-v1` | Public oEmbed endpoint | **Zero** | Token Bucket (10 RPS) |
| **JioSaavn** | `jiosaavn` | `jiosaavn-search-v1` | Public `api.php` search & getDetails endpoints | **Zero** | Token Bucket (5 RPS) |
| **Qobuz** | `qobuz` | `qobuz-api-v1` | Track metadata and search endpoints | Issued application ID; optional bearer token | Token Bucket (5 RPS) |
| **TIDAL** | `tidal` | `tidal-webapi-v3` | TIDAL Web API v2 track and search endpoints | Authorized OAuth token | Token Bucket (10 RPS) |
| **Amazon Music** | `amazonmusic` | `amazonmusic-webapi-v3` | Amazon Music Web API v2 catalog track search and lookup | Approved API access, OAuth token, security profile ID | Token Bucket (5 RPS) |
| **Pandora** | `pandora` | `pandora-graphql-v1` | Pandora GraphQL entity metadata lookup | Approved OAuth bearer token | Token Bucket (5 RPS) |

---

## Provider Specifics

### 1. Spotify Adapter (`internal/adapters/spotify/`)

Dual-mode adapter (`spotify-web-v3`):

- **Official mode**: When `OMMR_SPOTIFY_CLIENT_ID` / `OMMR_SPOTIFY_CLIENT_SECRET` are configured, it authenticates via the OAuth client-credentials flow (`/api/token`) and queries the Web API (`/v1/tracks/{id}`).
- Private Spotify browser sessions/cookies are supported subject to protected startup validation; see [PROVIDER_LIFECYCLE.md](PROVIDER_LIFECYCLE.md). Browser refresh remains challenge-dependent.
- **Zero-key fallback mode** (default): Attempts the current web-player token endpoint `/api/token?reason=transport&productType=web-player`. Anonymous access can be rejected (observed HTTP 400 on 2026-10-16); this produces an actionable error without echoing the response body. Text search has no embed fallback. Direct Spotify track-ID lookup additionally tries the public embed page and oEmbed. No challenge or access-control bypass is implemented.
- Accepts Spotify URLs (`https://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT`) or 22-character IDs (`4cOdK2wGLETKBW3PvgPWqT`).

### 2. YouTube Music Adapter (`internal/adapters/ytmusic/`)

- Artist/title search first uses YouTube Music's signed-out `WEB_REMIX` songs filter. Catalog audio rows supply separate artists and their channel IDs, album/browse ID, duration, explicit badges, artwork and additional display metadata. Only audio-track rows are accepted on this path; music-video rows are excluded. Empty/failed music search falls back to general YouTube video search. Player and direct-ID oEmbed lookup remain available. This is an unofficial interface and may change.
- Optional `OMMR_YTMUSIC_COOKIE` or owner-only `OMMR_YTMUSIC_COOKIE_FILE` signs these YouTube requests with SAPISIDHASH. This is an unofficial interface; the browser cookie may expire or the response shape may change. Do not interpret the video channel as a verified recording artist or credits.
- Falls back to YouTube oEmbed for direct video-ID lookups.
- Strips channel suffixes (`- Topic`, `VEVO`) and video noise tags (`Official Music Video`, `4K`, `Remastered`).

### 3. Apple Music Adapter (`internal/adapters/applemusic/`)
- Queries the iTunes Search & Lookup public endpoints with the configured `OMMR_APPLE_STOREFRONT` (default `us`). The 15-second HTTP safety cap is still bounded by the resolver's configured operation deadline (default 10 seconds), allowing public requests that exceed the former five-second cap.
- With `OMMR_APPLE_DEVELOPER_TOKEN`, queries the official Apple Music catalog at `https://api.music.apple.com/v1/catalog/{storefront}/songs/{id}` (or catalog search / exact ISRC filter). `OMMR_APPLE_STOREFRONT` defaults to `us`. The operator supplies and rotates an Apple-issued developer token. Composer is a display string, not a lyricist claim.
- Filters search results by ISRC when the query carries one.
- Returns high-resolution album artwork (`600x600bb.jpg`), primary genres, explicit flags, 30s preview URLs, track/disc numbers, and ISRC.

### 4. Deezer Adapter (`internal/adapters/deezer/`)
- Queries Deezer public REST endpoints.
- Normalizes ISRC/ISWC codes when supplied, album metadata, duration, track & disc positions, barcode, label, preview URL, and fan counts.

### 5. MusicBrainz Adapter (`internal/adapters/musicbrainz/`)
- After strict candidate acceptance, resolution looks up the accepted recording MBID for credited recording/work relationships. Identity disagreement or lookup failure skips enrichment without replacing the accepted recording. ISRC search/lookup retains the requested code even when a recording reports several codes in a different order.
- Queries MusicBrainz v2 Lucene search API (`musicbrainz.org/ws/2/recording`).
- Escapes special Lucene syntax characters (`$`, `&`, `/`, `+`, `-`, `!`, `(`, `)`, `{`, `}`, `[`, `]`, `^`, `"`, `~`, `*`, `?`, `:`) to prevent 503 error status responses.
- Direct recording lookup requests work and artist relationships. Only explicitly linked roles such as lyricist, composer, producer, and performer are credited. An ambiguous ISRC with multiple recordings is not chosen arbitrarily.
- A work-linked Wikipedia or Wikidata entity with an English Wikipedia sitelink can add `extensions.wikipedia` with article URL, summary, and attribution source. Wikipedia context does not supply recording identity.

### 6. SoundCloud Adapter (`internal/adapters/soundcloud/`)
- Resolves a SoundCloud permalink (`artist/track` or full URL) via the public oEmbed endpoint (`https://soundcloud.com/oembed`).
- Accepts `soundcloud_id` queries; no key required. Text search is not supported in zero-key mode (SoundCloud's search API requires OAuth).

### 7. JioSaavn Adapter (`internal/adapters/jiosaavn/`)
- Queries JioSaavn's public `api.php` endpoints: `search.getResults` and `song.getDetails`.
- Returns song ID (from `perma_url`), album, label, language, play counts, duration, explicit flag, release date, and upscaled cover art.
- The `JSONString` model type tolerates JioSaavn's inconsistent field typing (e.g. `"duration": 367` vs `"duration": "367"`).

### 8. Qobuz Adapter (`internal/adapters/qobuz/`)
- Uses the Qobuz track lookup and search endpoints, configured with `OMMR_QOBUZ_APP_ID`; an issued application ID is required. Optional `OMMR_QOBUZ_TOKEN` is sent as a bearer token.
- `OMMR_QOBUZ_BASE_URL` can override the API host for approved deployments and fixture tests.

### 9. TIDAL Adapter (`internal/adapters/tidal/`)
- Uses TIDAL Web API v2 track lookup and search with `OMMR_TIDAL_TOKEN`.
- `OMMR_TIDAL_BASE_URL` can override the API host. The adapter does not scrape TIDAL pages or fetch audio.

### 10. Amazon Music Adapter (`internal/adapters/amazonmusic/`)
- Uses catalog track lookup and track search with `OMMR_AMAZON_MUSIC_TOKEN` and `OMMR_AMAZON_MUSIC_API_KEY` against the fixed official `https://api.music.amazon.com` host. The token is an approved Login with Amazon bearer token; the API key is its security profile ID.
- Amazon currently documents this Web API as a closed beta for approved developers; without access credentials the adapter returns a configuration error.

### 11. Pandora Adapter (`internal/adapters/pandora/`)
- Looks up Pandora `TR:` track entities using the official GraphQL metadata API and `OMMR_PANDORA_TOKEN`.
- Pandora's API is OAuth-protected. Direct track/entity lookup is supported; text search is not exposed by this adapter.

Configuration variables:

| Provider | Variables |
| --- | --- |
| Qobuz | `OMMR_QOBUZ_APP_ID`, optional `OMMR_QOBUZ_TOKEN`, optional `OMMR_QOBUZ_BASE_URL` |
| TIDAL | `OMMR_TIDAL_TOKEN`, optional `OMMR_TIDAL_BASE_URL` |
| Amazon Music | `OMMR_AMAZON_MUSIC_TOKEN`, `OMMR_AMAZON_MUSIC_API_KEY` |
| Apple Music | `OMMR_APPLE_DEVELOPER_TOKEN`, optional `OMMR_APPLE_STOREFRONT` |
| YouTube Music | Optional `OMMR_YTMUSIC_COOKIE` or `OMMR_YTMUSIC_COOKIE_FILE` |
| Spotify | `OMMR_SPOTIFY_CLIENT_ID`, `OMMR_SPOTIFY_CLIENT_SECRET` |
| Pandora | `OMMR_PANDORA_TOKEN`, optional `OMMR_PANDORA_BASE_URL` (GraphQL endpoint prefix) |

### Per-provider extraction and field coverage
- `GET /v1/extract?provider=deezer&id=...` extracts one source; `GET /v1/extract?artist=...&title=...&sources=...` returns source-specific results without merging them.
- Each result includes `thumbnail_url`, `year`, and `present_fields` / `missing_fields` for the [metadata catalog](METADATA_CATALOG.md), including artwork, release type/date, IDs, credits by role, and contextual enrichment. A false `explicit` is not counted as evidence that the source reports clean content. Missing roles are never filled from a guess.
- The adapters are tested against provider-shaped JSON fixtures. Live equivalence requires valid access to credential-gated services and can vary by market and catalog edition.

---

## Notes

- **Shazam**: A Shazam adapter was initially evaluated, but Shazam's public web search endpoint now returns HTTP 405 for non-browser clients, so it was replaced by JioSaavn.
- **Rate limits** are enforced via token buckets in `internal/ratelimit/provider_limiter.go`; MusicBrainz is kept at 1 RPS out of respect for the service.


## Recording links

Adapters retain validated provider share URLs when present. SoundCloud's public
oEmbed widget may expose a numeric track ID; JioSaavn's API song ID is retained
separately from its share token. Amazon requests `id` and `url` in sparse track
fieldsets. `/v1/links` and `extensions.platform_links` expose accepted IDs/URLs
with their identifier namespace. See [PLATFORM_LINKS.md](PLATFORM_LINKS.md).

## Response modes

Per-source execution/version/error and field coverage described above require `debug=true`. Default `/v1/extract` returns an independent flat `results` array of public tracks; it does not claim these records match each other. Default resolve/bulk responses omit provenance and matching diagnostics. See [API_SPEC.md](API_SPEC.md#production-and-debug-responses).

## Catalog enrichment services

Four post-merge services complement the 11 identity adapters: Cover Art Archive (keyless), MusicBrainz release details (keyless), Last.fm (`OMMR_LASTFM_API_KEY`) and Discogs (`OMMR_DISCOGS_TOKEN`). AcoustID (`OMMR_ACOUSTID_API_KEY`) is available through a separate fingerprint endpoint. These are not streaming adapter IDs and do not create unverified recording matches. See [ENRICHMENT.md](ENRICHMENT.md) for fields and access conditions.

Local limits are implementation budgets, not guaranteed upstream quotas: MusicBrainz 1 RPS with burst 1; keyless iTunes one request every 3 seconds; Apple catalog 5 RPS; Cover Art Archive/Last.fm/AcoustID 3 RPS; Discogs 1 RPS. Search now uses the same limiter as resolution/extraction. Each catalog enrichment request passes through its budget; multiple server processes do not share budgets. Existing adapter-internal fallback/detail requests do not yet all have transport-level accounting.

## Startup selection and session import

Spotify, Apple Music and YouTube Music can import private browser sessions/cookies, subject to the actual token requirements of each service. Validated modes are selected before registration; unsupported/failed credentials use public metadata fallbacks. Required-key providers and enrichers stay out of the active registry unless startup access checks succeed. Runtime circuit breakers remove repeatedly failing sources from selection. Cookie-only Spotify exchange is challenge-dependent; Apple user cookies require a developer/browser bearer. Full details and safe status fields: [PROVIDER_LIFECYCLE.md](PROVIDER_LIFECYCLE.md).

## Credential-free live regression

See [LIVE_TEST_BOYFRIEND.md](LIVE_TEST_BOYFRIEND.md) for the 2026-10-16 retry, before/after metadata, observed access failures and cache timings. Apple, Deezer, MusicBrainz and JioSaavn reject non-JSON bodies before decoding; HTML access-error pages cannot become empty catalog successes.

## Accuracy and detail update

Accepted Deezer search hits now hydrate exact track details; Spotify hydrates the known album with ID/title checks. Apple public track counts populate `album.total_tracks`. Structured collaborators and same-provider artist-ID ownership are preserved. TIDAL requests JSON:API track resources with related artists/albums/artwork; Amazon requests quality and parental fields. Authenticated paths remain conditional on validated access and matching catalog entities. See [ACCURACY_IMPROVEMENTS.md](ACCURACY_IMPROVEMENTS.md) and [ENRICHMENT.md](ENRICHMENT.md) for behavior, tests and access limits.
