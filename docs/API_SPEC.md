# OMMR API Specification

Base Path: `/v1`

---

## Endpoints

| Method | Route | Description |
| --- | --- | --- |
| `GET` | `/v1/resolve` | Resolves unified track metadata and cross-platform identities by ID or Query |
| `POST` | `/v1/bulk` | Batch resolves up to 100 queries in a single request |
| `GET` | `/v1/search` | Performs raw search across metadata providers returning lightweight `SearchResult` objects |
| `GET` | `/v1/health` | System health check and uptime probe |
| `GET` | `/metrics` | Prometheus metrics endpoint |

---

## 1. Single Resolution Endpoint

`GET /v1/resolve`

### Query Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `spotify_id` | string | Optional | Spotify track ID or full track URL |
| `youtube_id` | string | Optional | YouTube video ID or full video URL |
| `deezer_id` | string | Optional | Deezer track ID or full track URL |
| `apple_id` | string | Optional | Apple Music track ID or full track URL |
| `soundcloud_id` | string | Optional | SoundCloud permalink (`artist/track`) or full track URL |
| `artist` | string | Optional | Artist name (used together with `title`) |
| `title` | string | Optional | Track title (used together with `artist`) |
| `album` | string | Optional | Album title (used together with `artist`/`title` for disambiguation) |
| `isrc` | string | Optional | International Standard Recording Code |
| `sources` | string | Optional | Comma-separated adapter filter (`spotify,ytmusic,applemusic,deezer,musicbrainz,soundcloud,jiosaavn`) |
| `bypass_cache` | boolean | Optional | If `true`, forces live provider fetch bypassing cache |

### Response 200 OK

> Note: The payload below is illustrative. Track/video IDs shown are example values and do not represent a specific real release.

```json
{
  "track": {
    "canonical_id": "ommr_track_2c15190166124384",
    "identity_status": "verified",
    "identity_reasons": [
      "isrc_match",
      "musicbrainz_match",
      "exact_title_artist_match"
    ],
    "title": "Tu",
    "artists": [
      { "name": "Talwiinder", "role": "main" }
    ],
    "album": {
      "title": "Tu - Single",
      "release_date": "2024-06-21"
    },
    "duration_ms": 218400,
    "release_date": "2024-06-21",
    "explicit": false,
    "isrc": "QZRP52317311",
    "genres": ["Punjabi Pop"],
    "credits": [
      { "name": "Talwiinder", "roles": ["Artist", "Composer"] }
    ],
    "images": [
      { "url": "https://is1-ssl.mzstatic.com/.../100x100bb.jpg", "width": 100, "height": 100, "type": "cover" },
      { "url": "https://is1-ssl.mzstatic.com/.../600x600bb.jpg", "width": 600, "height": 600, "type": "cover" }
    ],
    "ids": {
      "ytmusic": "FVNSACXFAy0",
      "applemusic": "1749982113",
      "musicbrainz": "400436a5-99fa-44d9-b51f-5c63be233454"
    },
    "identity_matches": [
      { "provider": "applemusic", "id": "1749982113", "confidence": 0.94, "method": "title_artist_duration" },
      { "provider": "musicbrainz", "id": "400436a5-99fa-44d9-b51f-5c63be233454", "confidence": 1.0, "method": "isrc" }
    ],
    "sources": ["ytmusic", "applemusic", "musicbrainz"],
    "field_sources": {
      "album": ["applemusic"],
      "artists": ["applemusic"],
      "credits": ["musicbrainz"],
      "duration": ["applemusic"],
      "genres": ["applemusic"],
      "images": ["ytmusic", "applemusic"],
      "isrc": ["musicbrainz"],
      "release_date": ["applemusic"],
      "title": ["applemusic"]
    },
    "match_score": 0.96,
    "match_breakdown": {
      "title": 1.0,
      "artists": 1.0,
      "album": 0.88,
      "duration": 1.0,
      "release_date": 1.0,
      "isrc": 1.0
    },
    "confidence": {
      "level": "exact",
      "score": 0.96
    },
    "completeness": 1.0
  },
  "metadata_sources": ["ytmusic", "applemusic", "musicbrainz"],
  "provider_status": [
    { "name": "applemusic", "success": true, "matched": true, "contributed": true, "cached": false, "latency_ms": 61, "version": "applemusic-v1" },
    { "name": "ytmusic", "success": true, "matched": true, "contributed": true, "cached": false, "latency_ms": 153, "version": "ytmusic-innertube-v1" },
    { "name": "musicbrainz", "success": true, "matched": true, "contributed": true, "cached": false, "latency_ms": 340, "version": "musicbrainz-v2" },
    { "name": "deezer", "success": true, "matched": false, "contributed": false, "cached": false, "latency_ms": 286, "rejection_reason": "no_results", "version": "deezer-v1" },
    { "name": "spotify", "success": true, "matched": false, "contributed": false, "cached": false, "latency_ms": 0, "rejection_reason": "no_results", "version": "spotify-web-v2" }
  ],
  "resolution_strategy": {
    "input_type": "youtube_id",
    "primary_source": "ytmusic",
    "cross_resolved": true
  },
  "resolver_stats": {
    "providers_queried": 7,
    "providers_matched": 3,
    "providers_contributed": 3,
    "candidates_evaluated": 6
  }
}
```

---

## 2. Bulk Resolution Endpoint

`POST /v1/bulk`

### Constraints
* Max array length: **100 items**
* Concurrency: Configurable worker pool (default: 10 workers)

### Request Payload

```json
{
  "queries": [
    { "youtube_id": "FVNSACXFAy0" },
    { "artist": "Talwiinder", "title": "Wishes" }
  ],
  "sources": ["applemusic", "deezer", "musicbrainz"]
}
```

---

## 3. Search Endpoint

`GET /v1/search`

### Query Parameters
* `q` (string, required): Raw search term
* `page` (integer, default: 1)
* `limit` (integer, default: 10, max: 50)

### Response 200 OK

```json
{
  "query": "boom shaka",
  "results": [
    {
      "canonical_id": "ommr_track_3dbef6050029a577",
      "title": "Boom Shaka",
      "artists": [
        { "name": "Muzi", "role": "main" }
      ],
      "album": "Boom Shaka",
      "images": [
        { "url": "https://is1-ssl.mzstatic.com/.../600x600bb.jpg", "width": 600, "height": 600 }
      ],
      "match_score": 0.91,
      "confidence": { "level": "high", "score": 0.91 },
      "identity_status": "strong",
      "identity_reasons": ["exact_title_artist_match"],
      "source_count": 1,
      "available_on": ["applemusic"]
    }
  ],
  "page": 1,
  "limit": 10,
  "total": 5
}
```
