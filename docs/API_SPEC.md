# OMMR API Specification

Base Path: `/v1`

---

## Production and debug responses

All data endpoints (`resolve`, `links`, `bulk`, `search`, `extract`) default to compact production JSON. Append `debug=true` to request the detailed response. `debug=false` is identical to omitting it. The flag uses Go boolean syntax (`true`/`false`, `1`/`0`, `t`/`f`, and supported case variants); empty, invalid, or repeated values return HTTP 400 before provider calls. It is a URL query parameter, including for `POST /v1/bulk?debug=true`.

```text
/v1/resolve?artist=Example&title=Song
/v1/resolve?artist=Example&title=Song&debug=true
```

| Endpoint | Default | With `debug=true` |
| --- | --- | --- |
| `/resolve` | `{ "track": ... }`, or `{ "track": null }` when no match passes the gates | Full canonical track, provider statuses, metadata sources, strategy, resolver statistics |
| `/bulk` | Item index, success/error, public track; batch totals | Full canonical track and per-item provider statuses (`sources`) |
| `/links` | Track identity and accepted platform IDs/URLs | Also provider statuses, unavailable platforms, link match methods/confidence and URL provenance |
| `/search` | Title, artists, album, images, canonical ID, available platforms; pagination | Also matching scores, confidence, identity diagnostics, source count |
| `/extract` | `{ "results": [ ...public tracks ] }` (independent records, never merged) | Existing per-source `query`/`sources` response, execution status/error, version and field coverage |

Production tracks retain music metadata, credits, artwork, accepted provider IDs, artist/album IDs, identity status, and compact `extensions.platform_links`. They omit matching scores/breakdowns, confidence calculations, completeness, identity reasons/methods, field provenance, contributing sources and internal relation graphs. Provider metadata is exposed in `track.provider_data`; only the public extensions `platform_links` and `wikipedia` remain under `extensions`; Wikipedia context keeps its attribution. Platform names in IDs/links represent usable catalog identifiers, not fetch diagnostics. Optional empty metadata arrays are omitted. No country code is added to the unified root; source-reported territory/country information is retained within provider data.

Production tracks also include `explicit_status` (`explicit`, `clean`, `unknown`). The internal canonical schema and shared disk/Redis cache remain complete. The presentation flag is not part of the resolver request/cache key: switching modes reuses the same cached resolution without changing its diagnostics or repeating provider work. It reduces transmitted bytes; it does not reduce stored cache-entry bytes or upstream work on a cache miss. Debug is an output mode, not an authorization boundary. A hosting-level switch to disable it is deferred.

**Migration:** clients relying on the previous diagnostic fields or extraction source envelopes must add `debug=true`. Detailed response examples below show debug mode unless otherwise noted. Default extraction returns an empty results array if all providers fail or find nothing; use debug mode to distinguish those outcomes.

---

## Endpoints

| Method | Route | Description |
| --- | --- | --- |
| `GET` | `/v1/artists` | Public source-owned artist profiles, audience metrics, biographies and catalog context |
| `GET` | `/v1/resolve` | Resolves unified track metadata and cross-platform identities by ID or Query |
| `GET` | `/v1/links` | Returns accepted recording IDs and links |
| `GET` | `/v1/extract` | Returns independent normalized metadata and field coverage from one or more providers |
| `POST` | `/v1/fingerprint` | AcoustID candidate lookup from a client-supplied fingerprint and duration |
| `POST` | `/v1/bulk` | Batch resolves up to 100 queries in a single request |
| `GET` | `/v1/search` | Performs raw search across metadata providers returning lightweight `SearchResult` objects |
| `GET` | `/v1/health` | System health check and uptime probe |
| `GET` | `/metrics` | Prometheus metrics endpoint |

---

## 1. Provider Extraction Endpoint

`GET /v1/extract`

Pass `provider` and `id` to fetch a single provider's track, or pass `artist` and `title` to compare provider search results independently. For title/artist extraction, `sources` can limit the comma-separated provider list. For JioSaavn exact lookups, optional `id_type=native` or `id_type=share` selects the identifier protocol explicitly. Without it, eight-character identifiers use native lookup and other identifiers use share lookup for compatibility. Explicit kinds are recommended when the caller knows the ID type; share tokens and native IDs are different identifiers. `id_type` is also accepted by `POST /v1/query` with `operation: extract`; invalid, repeated, or non-JioSaavn ID kinds are rejected.

```text
/v1/extract?artist=Example%20artist&title=Example%20track&sources=deezer,tidal,qobuz
/v1/extract?provider=deezer&id=123456
/v1/extract?provider=jiosaavn&id=OQsEfQFVUXk&id_type=share
```

With `debug=true`, each source result includes the normalized `track`, `year`, `thumbnail_url`, and `present_fields` / `missing_fields` arrays. Coverage fields include title, artists, album, year, artwork, duration, ISRC, credits, and lyricist. Missing optional fields are reported as missing; OMMR does not infer or synthesize credits. Provider failures are isolated to that source's `status` and `error`.

## 2. Single Resolution Endpoint

`GET /v1/resolve`

### Query Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `spotify_id` | string | Optional | Spotify track ID or full track URL |
| `youtube_id` | string | Optional | YouTube video ID or full video URL |
| `deezer_id` | string | Optional | Deezer track ID or full track URL |
| `apple_id` | string | Optional | Apple Music track ID or full track URL |
| `soundcloud_id` | string | Optional | SoundCloud permalink (`artist/track`) or full track URL |
| `qobuz_id` | string | Optional | Qobuz track ID |
| `tidal_id` | string | Optional | TIDAL track ID |
| `amazonmusic_id` | string | Optional | Amazon Music track ID (also accepts `amazon_music_id`) |
| `pandora_id` | string | Optional | Pandora track entity ID (with or without `TR:` prefix) |
| `artist` | string | Optional | Artist name (used together with `title`) |
| `title` | string | Optional | Track title (used together with `artist`) |
| `album` | string | Optional | Album title (used together with `artist`/`title` for disambiguation) |
| `isrc` | string | Optional | International Standard Recording Code |
| `sources` | string | Optional | Comma-separated adapter filter (`spotify,ytmusic,applemusic,deezer,musicbrainz,soundcloud,jiosaavn,qobuz,tidal,amazonmusic,pandora`) |
| `bypass_cache` | boolean | Optional | If `true`, forces live provider fetch bypassing cache |

For ID-based resolution, providers may report `rejection_reason: "unverified_candidate"` when their result cannot be tied to the exact source-ID result, or `"identity_mismatch"` when its metadata fails the source-track identity checks.

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
    "title": "Example Song",
    "artists": [
      { "name": "Example Artist", "role": "main" }
    ],
    "album": {
      "title": "Example Album",
      "release_date": "2024-06-21"
    },
    "duration_ms": 218400,
    "release_date": "2024-06-21",
    "explicit": false,
    "isrc": "XX-XXX-24-00001",
    "genres": ["Punjabi Pop"],
    "credits": [
      { "name": "Talwiinder", "roles": ["Artist", "Composer"] }
    ],
    "images": [
      { "url": "https://is1-ssl.mzstatic.com/.../100x100bb.jpg", "width": 100, "height": 100, "type": "cover" },
      { "url": "https://is1-ssl.mzstatic.com/.../600x600bb.jpg", "width": 600, "height": 600, "type": "cover" }
    ],
    "ids": {
      "ytmusic": "example-video-id",
      "applemusic": "example-track-id",
      "musicbrainz": "00000000-0000-0000-0000-000000000000"
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

## 3. Bulk Resolution Endpoint

`POST /v1/bulk`

### Constraints
* Max array length: **100 items**
* Concurrency: Configurable worker pool (default: 10 workers)
* Request body: **1 MiB maximum** and must contain exactly one JSON object

### Request Payload

```json
{
  "queries": [
    { "youtube_id": "example-video-id" },
    { "artist": "Example Artist", "title": "Example Song" }
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


## GET /v1/links

Uses the same query parameters and identity gates as `/v1/resolve`. Returns
`links` (OMMR ID, ISRC, accepted native IDs and song URLs, unavailable platforms),
title, artists, identity status and provider status. Unknown IDs/URLs are omitted.
See [PLATFORM_LINKS.md](PLATFORM_LINKS.md) for field definitions and supported services.
Resolved and bulk tracks include the same catalog in `extensions.platform_links`;
per-source extraction adds `links` without asserting cross-provider equivalence.

## Catalog enrichment, provider extras and fingerprint lookup

See [ENRICHMENT.md](ENRICHMENT.md) for `provider_data`, field trust/consensus, exact source activation and the fingerprint request/response contract. Debug responses store extra data inside `track.extensions.provider_data`; production promotes it to `track.provider_data`. Search remains a lightweight provider result pool; retrieve a full resolution for all accepted provider metadata. `sources` filters identity adapters; it does not select post-merge catalog services.

## Configured JSON gateway and lifecycle

`POST /v1/query` accepts an optional `operation` plus the selected operation's input fields. A configured default removes the need to select operation/sources/debug per request. Startup YAML controls enabled endpoints, permitted overrides and debug access. `GET /v1/providers` returns redacted lifecycle status; no token/cookie/profile data is included. Required-credential sources are not registered until a startup probe succeeds, and runtime-suspended sources are skipped. See [PROVIDER_LIFECYCLE.md](PROVIDER_LIFECYCLE.md).

When the configured/default source selection has no available identity adapters, resolve/links return HTTP 503. Unknown source names return 400. Existing endpoint routes remain available unless disabled in configuration.

## Rich public catalogs

See [RICH_CATALOGS.md](RICH_CATALOGS.md) for native Bugs/Genie/Melon song adapters, `bugs_id`/`genie_id`/`melon_id`, accepted YouTube artist/album context in provider data, optional Python pooling and `/v1/catalog` (also `operation: catalog` through the JSON gateway). These Korean adapters require no credentials; the optional Python path currently uses public YouTube access.


### JioSaavn public song-page input

For source extraction, `provider=jiosaavn&url=<percent-encoded HTTPS song URL>` reads the exact official song page before falling back to `webapi.get&type=song`. The JSON gateway accepts `{"operation":"extract","provider":"jiosaavn","url":"https://www.jiosaavn.com/song/mera-mann/ERIkRAwJf38"}`. URL input cannot be combined with `id` or `id_type`; repeated URLs, other providers, foreign hosts, credentials, ports, query strings, fragments and non-song paths are rejected. Only the fixed official JioSaavn host is fetched. Existing endpoint/debug policy applies. See [supported structures and live limits](JIOSAAVN_PUBLIC_PAGES.md).

YouTube catalog also supports `type=credits&id=MPTC<11-character identifier>` through GET `/v1/catalog` or POST `/v1/query` with `operation: catalog`. It returns bounded public credit sections as source context. See [catalog operations](RICH_CATALOGS.md).

## Artist profiles

`GET /v1/artists?artist=Gur%20Sidhu&limit=10` and `POST /v1/query` with `operation: artists` return independent public artist profiles. Optional `sources` filters supported providers; exact lookups use `provider` and a native artist `id`. Debug adds source status, cache and partial/error details. Bios, artist images, top/recent/catalog tracks and precise source-scoped audience/count semantics are described in [ARTIST_PROFILES.md](ARTIST_PROFILES.md). No song-list length is presented as a total track count, and no monthly/yearly plays are inferred from listeners or views.
