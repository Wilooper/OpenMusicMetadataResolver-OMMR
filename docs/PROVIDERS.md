# OMMR Provider Adapters

OMMR integrates 5 zero-key metadata provider adapters implementing the `adapters.ProviderAdapter` interface (`internal/adapters/adapter.go`).

---

## Adapter Summary

| Provider | Adapter Name | Version | Integration Method | API Keys | Rate Limits |
| --- | --- | --- | --- | --- | --- |
| **Spotify** | `spotify` | `spotify-web-oembed` | Public oEmbed & Open Graph HTML Scraping | **Zero** | Token Bucket (30 RPS) |
| **YouTube Music** | `ytmusic` | `ytmusic-oembed-v1` | Public oEmbed & YouTube API v3 Web | **Zero** | Token Bucket (20 RPS) |
| **Apple Music** | `applemusic` | `applemusic-v1` | iTunes Search & Public Web API | **Zero** | Token Bucket (20 RPS) |
| **Deezer** | `deezer` | `deezer-v1` | Public API (`api.deezer.com`) | **Zero** | Token Bucket (50 RPS) |
| **MusicBrainz** | `musicbrainz` | `musicbrainz-v2` | MusicBrainz REST API v2 | **Zero** | Token Bucket (1 RPS) |

---

## Provider Specifics

### 1. Spotify Adapter (`internal/adapters/spotify/`)
- Extracts metadata and high-res cover art from Spotify web oEmbed endpoints and Open Graph tags (`og:title`, `og:image`, `music:duration`).
- Accepts Spotify URLs (`https://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT`) or 22-character IDs (`4cOdK2wGLETKBW3PvgPWqT`).

### 2. YouTube Music Adapter (`internal/adapters/ytmusic/`)
- Resolves video titles, thumbnails, and channel names via YouTube oEmbed (`https://www.youtube.com/oembed`).
- Strips channel suffixes (`- Topic`, `VEVO`) and video noise tags (`Official Music Video`, `4K`, `Remastered`).

### 3. Apple Music Adapter (`internal/adapters/applemusic/`)
- Queries Apple Music iTunes public search endpoints.
- Returns high-resolution album artwork (`600x600bb.jpg`), primary genres, and explicit content flags.

### 4. Deezer Adapter (`internal/adapters/deezer/`)
- Queries Deezer public REST endpoints.
- Provides ISRC codes, album metadata, duration in seconds, and track IDs.

### 5. MusicBrainz Adapter (`internal/adapters/musicbrainz/`)
- Queries MusicBrainz v2 Lucene search API (`musicbrainz.org/ws/2/recording`).
- Escapes special Lucene syntax characters (`$`, `&`, `/`, `+`, `-`, `!`, `(`, `)`, `{`, `}`, `[`, `]`, `^`, `"`, `~`, `*`, `?`, `:`) to prevent 503 error status responses.
- Provides contributor credits (`Composer`, `Producer`, `Artist`) and recording MBIDs.
