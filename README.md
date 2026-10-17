# 🎵 OMMR — Open Music Metadata Resolver

> **Self-hostable Go music metadata resolver with source-by-source extraction and guarded cross-platform matching.**

[![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=for-the-badge&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge)](LICENSE)
[![Release](https://img.shields.io/badge/Release-v1.0.0-green?style=for-the-badge)](https://github.com/Wilooper/OpenMusicMetadataResolver-OMMR-/releases)
[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen?style=for-the-badge)](https://github.com/Wilooper/OpenMusicMetadataResolver-OMMR-/actions)
[![Zero Key](https://img.shields.io/badge/Zero--Key-First-orange?style=for-the-badge)](#no-api-keys-required)
[![Docker Ready](https://img.shields.io/badge/Docker-Ready-2496ED?style=for-the-badge&logo=docker)](docker-compose.yml)

---

## 📌 Why OMMR?

### The Problem
Music metadata across streaming services is heavily fragmented:
* A YouTube video ID (`x18b0D8sTwo`) cannot be directly mapped to Apple Music (`1534955145`), Deezer (`108990152`), or MusicBrainz (`b068febb...`).
* Streaming providers format artist and song titles inconsistently (e.g. `AP Dhillon - Topic`, `Excuses (Official Video)`, `Excuses (Remastered)`).
* Commercial metadata resolution APIs require expensive developer credentials, secret keys, or restrictive API quotas for every service.

### The OMMR Solution
OMMR accepts a platform identifier or artist and title and returns a canonical record with field provenance and identity confidence. Public paths require no keys; richer official catalog modes and some partner APIs require credentials and may have account restrictions. See [metadata fields and provider evidence](docs/METADATA_CATALOG.md).

### Private configuration

Cross-platform identifiers and song URLs are available at
`/v1/links?youtube_id=<video-id>` (or another supported ID, ISRC, artist/title).
Resolved tracks also include `extensions.platform_links`. See
[platform IDs, URLs and availability](docs/PLATFORM_LINKS.md).

Configuration precedence is process environment (including explicitly empty
values), YAML configuration, env-file values, then built-in defaults. Spotify
official mode requires both client credentials. YouTube cookies must be a single
header value containing `SAPISID` or `__Secure-3PAPISID`; invalid configuration
fails at startup without echoing the cookie. Apple and Spotify credentialed
clients refuse HTTP redirects.

Copy `.env.example` to `.env`, fill only the credentials you have, run `chmod 600 .env`, and start with `OMMR_ENV_FILE=.env ./bin/ommr-server`. Alternatively copy `ommr.example.yaml` to `ommr.yaml`, run `chmod 600 ommr.yaml`, and set `OMMR_CONFIG_FILE=ommr.yaml`. Process environment overrides the file. Neither file is committed. Never put cookies or tokens in API URLs or logs. `OMMR_YTMUSIC_COOKIE_FILE` points to a private file containing a browser Cookie header; the optional cookie is used only on YouTube InnerTube calls. Apple uses an operator-generated **developer token** and storefront for the official catalog endpoint; the public iTunes lookup remains the keyless fallback. Spotify supports official credentials and imported browser sessions; current Spotify Development Mode requires the app owner to have Premium. Amazon Music V2 requires approved beta access. Optional credentialed modes have fixture tests, but live provider calls require your own authorized credentials.

---

## ✨ Key Features

* **🌐 Keyless-first resolution**: Public Apple and YouTube Music catalog search plus optional public lookup adapters; source access and supported query types vary. Spotify anonymous search can be rejected, and SoundCloud keyless mode supports permalinks. Authorized sources are optional upgrades.
* **🔗 6-Stage Cross-Platform Identity Pipeline**: Translates input IDs into cross-platform identifiers (`spotify`, `youtube`, `applemusic`, `deezer`, `soundcloud`, `jiosaavn`, `musicbrainz`, `isrc`).
* **🆔 Path-Independent Canonical Track IDs**: Generates deterministic, collision-resistant track identifiers (`ommr_track_<hash>`) from normalized metadata regardless of query entry point; identically named recording versions can still share a seed.
* **📊 Multi-Source Metadata Merging**: Prioritizes higher-quality metadata fields across providers (e.g. MusicBrainz credits, Spotify genres, Apple Music cover art) with deduplicated source attribution.
* **🕵️ Source Transparency & Field Provenance**: With `debug=true`, returns `metadata_sources`, `provider_status` diagnostics (latency, cached status, rejection reasons), and `field_sources` (attributing every field to its source provider).
* **🧠 Composite Matching & Weighted Scores**: Combines Jaro-Winkler + Token Set Ratio composites, strict candidate acceptance (score threshold + artist gate), and provider trust coefficients (`MusicBrainz: 1.00`, `Apple: 0.95`, `Spotify: 0.95`, `Deezer: 0.90`, `JioSaavn: 0.85`, `YTMusic: 0.80`, `SoundCloud: 0.75`).
* **🎯 Structured Identity Verification**: Assigns `identity_status` (`verified`, `strong`, `probable`, `weak`) and `identity_reasons` (`isrc_match`, `musicbrainz_match`, `exact_title_artist_match`).
* **⚡ Dual-Tier Caching**: Sub-divided local disk cache (`raw/`, `normalized/`, `resolved/`) and Redis caching.
* **📦 Operations**: Includes Prometheus metrics (`/metrics`), request IDs, provider rate limiting, bulk resolution, and Docker Compose deployment. Credentialed integrations still require live validation with your accounts.

---

## 🏗️ Architecture Overview

```mermaid
flowchart TD
    Client[HTTP Client / API Consumer] --> API[HTTP Router & Handler /v1/resolve]
    API --> Res[Resolver Engine]
    
    subgraph Execution Pipeline
        Res --> S1[Stage 1: Direct Provider Lookup]
        S1 --> S2[Stage 2: Parallel Adapter Execution]
        S2 --> S3[Stage 3: Noise Cleaning & ISRC Discovery]
        S3 --> S4[Stage 4: Composite String & Trust Matching]
        S4 --> S5[Stage 5: Multi-Source Priority Merger]
        S5 --> S6[Stage 6: Identity Graph & Canonical ID Assignment]
    end

    S2 --> Adapters[Provider Adapters]
    Adapters --> Spotify[Spotify Adapter]
    Adapters --> YTMusic[YouTube Music Adapter]
    Adapters --> Apple[Apple Music Adapter]
    Adapters --> Deezer[Deezer Adapter]
    Adapters --> MB[MusicBrainz Adapter]
    Adapters --> SC[SoundCloud Adapter]
    Adapters --> JS[JioSaavn Adapter]
    
    S6 --> Cache[(Disk / Redis Cache)]
    S6 --> Response[Canonical Track Response]
    Response --> Client
```

---

## 🚀 Quick Start

### 1. Build and Run Binary

```bash
# Clone repository
git clone https://github.com/Wilooper/OpenMusicMetadataResolver-OMMR-.git
cd OpenMusicMetadataResolver-OMMR-

# Build server binary
go build -o bin/ommr-server.exe ./cmd/server

# Run server
./bin/ommr-server.exe
```

Server starts listening on `http://localhost:8080`.

### 2. Prebuilt Docker Container (GHCR)

```bash
# Pull and run prebuilt container from GitHub Container Registry
docker run -d \
  --name ommr-server \
  -p 8080:8080 \
  ghcr.io/wilooper/ommr:latest
```

### 3. Docker Compose (Recommended for Self-Hosting)

```bash
# Start OMMR with Redis cache via Docker Compose
docker compose up -d
```

Verify service readiness:

```bash
curl http://localhost:8080/v1/health
```

Output:
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime_seconds": 42,
  "cache_provider": "redis"
}
```

---

## 📡 API Usage & Examples

### Resolve by YouTube Video ID

```bash
curl "http://localhost:8080/v1/resolve?youtube_id=x18b0D8sTwo"
```

### Resolve by Artist + Song Title

```bash
curl "http://localhost:8080/v1/resolve?artist=Talwiinder&title=Tu"
```

### Resolve by SoundCloud Permalink

```bash
curl "http://localhost:8080/v1/resolve?soundcloud_id=m83/midnight-city"
```

### Lightweight Track Search

```bash
curl "http://localhost:8080/v1/search?q=boom%20shaka&page=1&limit=5"
```

---

## 📋 Example Response Payload (`/v1/resolve`)

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
    "genres": ["Punjabi Pop", "Indian Pop"],
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

## ⚙️ Configuration

OMMR is configured via environment variables or a `.env` file:

| Variable | Type | Default | Description |
| --- | --- | --- | --- |
| `OMMR_PORT` | Integer | `8080` | HTTP listening port |
| `OMMR_CACHE_PROVIDER` | String | `disk` | `disk` or `redis` |
| `OMMR_CACHE_DIR` | String | `./cache_data` | Disk storage |
| `OMMR_REDIS_URL` | String | `redis://localhost:6379/0` | Redis URL |
| `OMMR_CACHE_TTL` | Duration | `24h` | Positive resolution TTL |
| `OMMR_RATE_LIMIT_RPS` | Float | `50` | Process-wide API request budget |
| `OMMR_PROVIDER_TIMEOUT` | Duration | `10s` | Entire resolution/search operation budget |
| `OMMR_BULK_MAX_ITEMS` | Integer | `100` | Max queries per batch |
| `OMMR_BULK_WORKERS` | Integer | `10` | Bulk workers |
| `OMMR_ENABLE_ENRICHMENT` | Boolean | `true` | Post-merge catalog context/artwork |
| `OMMR_METADATA_POLICY` | JSON string | consensus | Field voting and trust overrides |
| `OMMR_LASTFM_API_KEY` | String | empty | Optional Last.fm context |
| `OMMR_DISCOGS_TOKEN` | String | empty | Optional Discogs edition context |
| `OMMR_ACOUSTID_API_KEY` | String | empty | Optional fingerprint lookup |
| `OMMR_SPOTIFY_CLIENT_ID` / `OMMR_SPOTIFY_CLIENT_SECRET` | String | empty | Optional official Spotify mode |


---

## 🗄️ Caching Architecture

OMMR implements two interchangeable caching providers implementing the `cache.Cache` interface:

* **Disk Cache (`internal/cache/disk/`)**: Stores entries in sub-directories (`raw/{provider}/`, `normalized/`, `resolved/`) for inspection and zero-dependency deployments.
* **Redis Cache (`internal/cache/redis/`)**: Uses `go-redis/v9` with automatic key TTLs and connection pooling for high-concurrency production deployments.

Bypass cache per request using `GET /v1/resolve?...&bypass_cache=true`.

---

## 📚 Technical Documentation Index

Detailed technical specifications are located in the `docs/` directory:

* 📖 **[API Specification](docs/API_SPEC.md)** — Complete REST endpoint documentation and DTO schemas.
* 🏗️ **[Architecture Guide](docs/ARCHITECTURE.md)** — Clean architecture design, 6-stage pipeline, and concurrency models.
* 📐 **[Domain Schema](docs/SCHEMA.md)** — In-depth domain model definitions (`Track`, `IdentityGraph`, `IdentityMatch`).
* 🎯 **[Matching Engine](docs/MATCHING_ENGINE.md)** — String similarity algorithms, provider trust weights, and scoring matrices.
* 💾 **[Caching System](docs/CACHING.md)** — Disk and Redis caching implementations and TTL strategies.
* 🔑 **[Identity Engine](docs/IDENTITY_ENGINE.md)** — Recording hash generation, identity verification rules, and graph storage.
* 🔌 **[Provider Adapters](docs/PROVIDERS.md)** — Zero-key provider integration details, Lucene escaping, and fallback strategies.
* 🤝 **[Contributing Guide](docs/CONTRIBUTING.md)** — Developer setup, Go conventions, skill library integration, and PR rules.
* ❓ **[Frequently Asked Questions](docs/FAQ.md)** — Common questions regarding zero-key operation, rate limiting, and accuracy.

---

## 🗺️ Roadmap

### Version 1.0 (Current)
- [x] Zero-key provider adapters (Spotify dual-mode, YouTube Music InnerTube, Apple Music, Deezer, MusicBrainz, SoundCloud, JioSaavn)
- [x] 6-stage cross-platform ID discovery pipeline
- [x] Path-independent canonical track IDs (`ommr_track_<hash>`)
- [x] Calibrated composite matching engine, strict candidate acceptance & provider trust weights
- [x] Extended canonical schema (ISWC, preview URLs, track/disc numbers, labels, barcodes, copyrights, play counts)
- [x] Source attribution & provenance diagnostics (`field_sources`, `provider_status`, `resolver_stats`)
- [x] Disk and Redis dual caching backends

### Future Releases
- [ ] **v1.1**: Expanded lyrics metadata extraction & sync timecodes
- [ ] **v1.2**: Canonical Artist Identity Resolver (`ommr_artist_<hash>`)
- [ ] **v1.3**: Canonical Album & Release Identity Resolver (`ommr_album_<hash>`)
- [ ] **v1.4**: Playlist cross-platform conversion endpoint (`POST /v1/playlist/convert`)
- [ ] **v2.0**: GraphQL API query interface & automatic OpenAPI client SDK generation

---

## 👥 Contributing

Contributions are warmly welcome! Please read [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) for details on our code style, local skill library integration (`cc-skills-golang`), and pull request submission process.

---

## 📄 License

OMMR is open-source software licensed under the [MIT License](LICENSE).

hi

### Compact API output

Data endpoints return production metadata by default. Add `&debug=true` to a resolve/search/links/extract query, or use `POST /v1/bulk?debug=true`, for provider execution details and matching diagnostics. Use `?debug=true` when it is the first query parameter. `debug=false` keeps the compact format; malformed or repeated values return 400. Both modes reuse the same complete resolver cache. Existing clients that consume provider diagnostics or per-source extraction envelopes should explicitly enable debug. See [response modes](docs/API_SPEC.md#production-and-debug-responses).

### Rich provider metadata

Accepted provider alternatives and extra catalog fields are now retained in production `track.provider_data`. Canonical fields use deterministic consensus or configured trust; diagnostics remain opt-in. The new catalog services add Cover Art Archive artwork, MusicBrainz release details, optional Last.fm and Discogs context, and a separate AcoustID fingerprint endpoint. See [ENRICHMENT.md](docs/ENRICHMENT.md) for the exact schema, access settings, identity checks and remaining limitations. [CACHING.md](docs/CACHING.md) describes actual cache integration.

### Startup sessions, health and API policy

A private `ommr.yaml` now loads automatically; `--config` selects another file. Named files in an owner-only credential directory are discovered at boot. Supplied credentials must pass source-specific probes; public-capable sources fall back, and required-key sources stay unloaded when access is absent or fails. Runtime failures trigger a configurable circuit and controlled recovery. `POST /v1/query` uses configured operation/source/debug defaults, and `/v1/providers` shows redacted lifecycle status. See [PROVIDER_LIFECYCLE.md](docs/PROVIDER_LIFECYCLE.md) for formats, session limits and the full sample configuration.

Accuracy improvements: structured collaborator records with owned artist IDs, stricter artist evidence for text queries, guarded accepted-record detail hydration, and debug field comparison. See [ACCURACY_IMPROVEMENTS.md](docs/ACCURACY_IMPROVEMENTS.md) for accuracy behavior and remaining access limits. See [the 12-song live matrix](docs/LIVE_SONG_MATRIX.md) and [credential checklist](docs/CREDENTIALS_CHECKLIST.md).
