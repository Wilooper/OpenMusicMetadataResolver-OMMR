# Detailed credits, edition placement and smaller production packets

## Exact album placement

A recording can occur on an original release, a deluxe edition and a compilation at different positions. OMMR now selects the album, disc number and track number together from one accepted provider observation. Direct song-ID/share-link resolution anchors that tuple to the requested provider's album when the accepted record supplies a native album ID and title. Text queries retain configured album selection. Missing disc/track numbers remain unknown; they are never borrowed from another edition or defaulted to one.

Production track responses include `album_placements`; debug tracks retain it under `extensions.album_placements`. Each entry is owned by its provider and keeps its native song and album IDs together. `selected: true` identifies the source observation used for top-level positions. Other accepted placements remain available even when they belong to different editions. Graph `appears_on` edges also retain the observation's numeric position and disc number.

```json
{
  "title": "Flip-Side (Sandy's Interlude)",
  "disc_number": 2,
  "track_number": 1,
  "album_placements": [{
    "provider": "applemusic",
    "album_id": "1829591921",
    "track_id": "1829591990",
    "disc_number": 2,
    "track_number": 1,
    "selected": true
  }]
}
```

This excerpt abbreviates the response. Resolve the native song ID to preserve a played edition; an artist/title search alone cannot establish which edition the player selected. If the original source supplies no native album reference, no exact source-edition guarantee is made. Use the selected placement's `provider` and `album_id` with `/v1/albums` for the edition's full list, reported total and completion state. That endpoint supports Apple, Deezer, JioSaavn and MusicBrainz release IDs; other providers' IDs remain references, not guaranteed native-album endpoint support.

Same-title album IDs in the legacy `album.ids` view are not independent proof of edition equivalence. Position-sensitive clients should use `album_placements`. Edition-specific totals, UPC, barcode, label, type and date are no longer filled from another provider solely because the album title matches. Unknown source totals remain unknown. Existing flat track-level label/barcode fields keep their recording metadata behavior.

Configure the `album` metadata priority to choose the tuple for text queries. Independent `track_number` / `disc_number` priority configuration now returns a validation error directing the operator to `album`; separate field votes cannot produce a trustworthy edition position. A direct provider ID takes precedence over configured album-source preference when its exact native album reference is available. Other metadata policy fields and candidate identity gates are unchanged.

## Detailed source credits

`credit_details` is an additive index of the existing source-scoped music graph, available on production tracks and native albums. Debug tracks keep it in `extensions.credit_details`. It contains:

- `contributors`: each referenced contributor once, with graph ID, source provider, native ID when present, display name and an explicit `local` flag for name-only identities.
- `credits`: credit edges whose `from` references a contributor and whose `to` references a subject node in `music_graph.nodes`. The subject node's type distinguishes a recording, composition/work, release edition or another explicitly returned entity.
- Original source role, normalized role, credited name, instruments/other relationship attributes, attribute credit text, dates, direction and relationship type ID, wherever supplied. Missing values stay absent.
- `truncated: true` when the graph's existing bounds were reached. The index does not pretend truncated metadata is exhaustive.

Do not apply every indexed credit to the currently playing recording: follow its subject ID. Composition authors remain on their work, recording performers/producers remain on their recording, and release artwork remains on the release. Same-name people with distinct source IDs are not merged in the detailed graph/index. Name-only people stay local to their source and subject. The legacy flat `credits` array remains a name/role convenience view and cannot establish contributor identity or credit scope.

The index reuses graph IDs instead of repeating a contributor's complete identity for every role. The full graph and sanitized source catalog metadata remain available; compression reduces their repeated strings on the wire without dropping fields.

### New extraction coverage

Melon's actual public Boyfriend page puts its composer list in `section_prdcr`, outside the main song-info section. The adapter now reads only that explicit contributor section after validating the requested song identity. It retains source artist IDs where present, decodes HTML entities, preserves entire names (including commas), preserves Korean role labels and normalizes explicit composer/lyricist/arranger labels. It does not collect unrelated sidebar artists, fabricate missing IDs, or infer that a performer with the same name is the contributor.

MusicBrainz's flat credit extraction now recognizes specialized roles already supported by its graph: instruments, vocals, mixing/mastering/recording engineering, conductor, orchestrator, co/executive producer, programming, remixer and translator, alongside existing roles. Unknown relationship types are not invented as credits. Rich MusicBrainz relationship behavior is fixture-tested here; live source access remains a separate limitation.

## Lossless production transport

Send `Accept-Encoding: gzip` or use a client that negotiates gzip. JSON responses of at least 1024 bytes under `/v1/` stream through pooled Go `gzip.BestSpeed` writers. Only a small prefix is buffered. Smaller responses remain plain. The resolver cache remains format-independent; compression runs after metadata selection and caching.

The middleware sets `Content-Encoding: gzip` and `Vary: Accept-Encoding`, removes stale representation lengths/ETags when transforming, honors explicit `gzip;q=0` and wildcard negotiation, and avoids double compression. HEAD, range requests/responses, non-JSON, existing encodings and `Cache-Control: no-transform` responses are left uncompressed. Metrics retain their existing handler. No source fields, credits, graph edges or provider records are removed to achieve the measured reduction.

```sh
curl --compressed 'http://localhost:8080/v1/resolve?apple_id=1829591990'
```

This measures packet bytes, not a CPU/latency improvement. Compression does consume CPU; the implementation uses the standard library's speed-oriented level and pools/reset writers to avoid retaining response references between requests.

## Live verification, 2026-10-21

| Public-only API case | Plain JSON bytes | Gzip bytes | Reduction | Source result |
| --- | ---: | ---: | ---: | --- |
| Boyfriend, Apple song ID | 6,261 | 1,562 | 75.05% | P-POP CULTURE, disc 1 track 4 |
| Flip-Side, Apple song ID | 6,496 | 1,600 | 75.37% | P-POP CULTURE, disc 2 track 1 |
| For Youth, Apple song ID | 6,138 | 1,536 | 74.98% | Proof, disc 3 track 1 |
| Boyfriend, Melon song ID | 6,774 | 1,699 | 74.92% | Three source-declared composers |
| Proof, native Apple album | 86,287 | 8,362 | 90.31% | Complete 35-track album |

All five final comparisons matched byte-for-byte after decompression, as well as matching parsed JSON. Counts are the compact API payloads; pretty-printed evidence files are larger. Byte sizes vary with source metadata/timestamps.

Melon returned composer assertions for **Ikwinder Sahota**, **Karan Aujla** and **Milan D'Agostini**, retaining the original `작곡` role label. Their source page did not expose usable native artist IDs in those composer links, so they remain local identities. These are attributed catalog assertions, not independently verified authorship claims. The initial Melon request returned `track: null`; the next request succeeded, so the first comparison could not establish compression parity. The final fresh run resolved both variants from cache and passed. The initial empty result is retained alongside the successful results.

Full build, vet and race/coverage checks passed: **267 top-level tests, 384 cases, zero failures/skips**. Results are in [checks](test-results/detailed-production-2026-10-21/checks.json). Live responses, exact placements and size comparisons are in [live evidence](test-results/detailed-production-2026-10-21/live-summary.json). Regression tests cover mixed editions, unknown positions, direct-ID anchoring, policy validation, outside-scope contributors, same-name identities, relationship attributes/scope, encoding negotiation, byte preservation and concurrent compressor reuse.

```sh
go build -o bin/server ./cmd/server
python tools/live_production_metadata.py --server bin/server --output /tmp/ommr-production-metadata
```

Resolved song cache advances to `rich-v16`; native albums to `albums:v5`, invalidating old mixed positions and missing credit indexes. Korean provider versions advance to `public-v2-credits`. Existing canonical Go models stay unchanged; production additions are response projections of cached extension metadata.

References: [MusicBrainz relationship scopes](https://musicbrainz.org/doc/Style/Relationships), [relationship API examples](https://musicbrainz.org/doc/MusicBrainz_API/Examples), and [HTTP content negotiation, RFC 9110](https://www.rfc-editor.org/rfc/rfc9110.html).
