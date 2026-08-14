# OMMR Provider Adapters

OMMR integrates 7 metadata provider adapters implementing the `adapters.ProviderAdapter` interface (`internal/adapters/adapter.go`). Every adapter works **zero-key by default**; only the Spotify official path is optional and key-based.

---

## Adapter Summary

| Provider | Adapter Name | Version | Integration Method | API Keys | Rate Limits |
| --- | --- | --- | --- | --- | --- |
| **Spotify** | `spotify` | `spotify-web-v2` | Official Web API (optional keys) + anonymous Web API / oEmbed / embed scraping | **Optional** | Token Bucket (10 RPS) |
| **YouTube Music** | `ytmusic` | `ytmusic-innertube-v1` | InnerTube search & player (public API key) + oEmbed fallback | **Zero** | Token Bucket (20 RPS) |
| **Apple Music** | `applemusic` | `applemusic-v1` | iTunes Search & Lookup API | **Zero** | Token Bucket (20 RPS) |
| **Deezer** | `deezer` | `deezer-v1` | Public API (`api.deezer.com`) | **Zero** | Token Bucket (50 RPS) |
| **MusicBrainz** | `musicbrainz` | `musicbrainz-v2` | MusicBrainz REST API v2 | **Zero** | Token Bucket (1 RPS) |
| **SoundCloud** | `soundcloud` | `soundcloud-oembed-v1` | Public oEmbed endpoint | **Zero** | Token Bucket (10 RPS) |
| **JioSaavn** | `jiosaavn` | `jiosaavn-search-v1` | Public `api.php` search & getDetails endpoints | **Zero** | Token Bucket (5 RPS) |

---

## Provider Specifics

### 1. Spotify Adapter (`internal/adapters/spotify/`)

Dual-mode adapter (`spotify-web-v2`):

- **Official mode**: When `OMMR_SPOTIFY_CLIENT_ID` / `OMMR_SPOTIFY_CLIENT_SECRET` are configured, it authenticates via the OAuth client-credentials flow (`/api/token`) and queries the Web API (`/v1/tracks/{id}`).
- **Zero-key fallback mode** (default): Fetches an anonymous web-player access token from `https://open.spotify.com/get_access_token`, then calls the Web API. If the anonymous token is unavailable, it falls back to scraping the embed page's `__NEXT_DATA__` (`ParseEmbedNextData` in `internal/adapters/spotify/parser.go`), and finally to the public oEmbed endpoint.
- Accepts Spotify URLs (`https://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT`) or 22-character IDs (`4cOdK2wGLETKBW3PvgPWqT`).

### 2. YouTube Music Adapter (`internal/adapters/ytmusic/`)

- Uses the public InnerTube API (search & player endpoints, public web API key) to search by artist/title and fetch full player metadata for a video ID, including duration and rich track details (`internal/adapters/ytmusic/innertube.go`).
- Falls back to YouTube oEmbed for direct video-ID lookups.
- Strips channel suffixes (`- Topic`, `VEVO`) and video noise tags (`Official Music Video`, `4K`, `Remastered`).

### 3. Apple Music Adapter (`internal/adapters/applemusic/`)
- Queries the iTunes Search & Lookup public endpoints.
- Filters search results by ISRC when the query carries one.
- Returns high-resolution album artwork (`600x600bb.jpg`), primary genres, explicit flags, 30s preview URLs, track/disc numbers, and ISRC.

### 4. Deezer Adapter (`internal/adapters/deezer/`)
- Queries Deezer public REST endpoints.
- Provides ISRC/ISWC codes, album metadata, duration, track & disc positions, barcode, label, preview URL, and fan counts.

### 5. MusicBrainz Adapter (`internal/adapters/musicbrainz/`)
- Queries MusicBrainz v2 Lucene search API (`musicbrainz.org/ws/2/recording`).
- Escapes special Lucene syntax characters (`$`, `&`, `/`, `+`, `-`, `!`, `(`, `)`, `{`, `}`, `[`, `]`, `^`, `"`, `~`, `*`, `?`, `:`) to prevent 503 error status responses.
- Provides contributor credits (`Composer`, `Producer`, `Artist`), recording MBIDs, and ISRCs.

### 6. SoundCloud Adapter (`internal/adapters/soundcloud/`)
- Resolves a SoundCloud permalink (`artist/track` or full URL) via the public oEmbed endpoint (`https://soundcloud.com/oembed`).
- Accepts `soundcloud_id` queries; no key required. Text search is not supported in zero-key mode (SoundCloud's search API requires OAuth).

### 7. JioSaavn Adapter (`internal/adapters/jiosaavn/`)
- Queries JioSaavn's public `api.php` endpoints: `search.getResults` and `song.getDetails`.
- Returns song ID (from `perma_url`), album, label, language, play counts, duration, explicit flag, release date, and upscaled cover art.
- The `JSONString` model type tolerates JioSaavn's inconsistent field typing (e.g. `"duration": 367` vs `"duration": "367"`).

---

## Notes

- **Shazam**: A Shazam adapter was initially evaluated, but Shazam's public web search endpoint now returns HTTP 405 for non-browser clients, so it was replaced by JioSaavn.
- **Rate limits** are enforced via token buckets in `internal/ratelimit/provider_limiter.go`; MusicBrainz is kept at 1 RPS out of respect for the service.
