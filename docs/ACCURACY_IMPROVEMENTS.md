# Accuracy and richer metadata follow-up — 2026-10-16

## Behavior

- Pure artist/title queries require complete normalized artist-name evidence or a close spelling variant, in addition to unchanged existing thresholds. A shared word cannot establish artist identity. Structured collaborators participate in scoring rather than only the first artist.
- Default consensus uses separate artist records only when accepted source evidence supplies an equivalent unambiguous list. Exact individual-name matches can contribute their own native IDs. Compound display IDs are not assigned to both people. Configured artist priorities remain respected.
- Accepted Deezer search candidates fetch exact track details. Spotify candidates with a known album ID fetch exact album details for label, UPC, total tracks, copyright and safe extras. Detail failures and mismatches retain the search candidate. Native IDs, titles, primary artists, known ISRCs, durations and known album IDs are checked before hydration. Original scoring is retained.
- Hydration runs per provider concurrently, bounded to five seconds and the remaining API deadline. Search-only normalized/catalog fields survive detail responses. Failed detail or catalog enrichment caps resolved caching at 30 seconds, including when initial search succeeded.
- TIDAL searches the tracks relationship and requests supported artist/album/artwork includes. Scoped JSON:API decoding follows type-and-ID linkage, not arbitrary included entities. Fractional seconds and ISO-8601 music durations preserve milliseconds. Numeric IDs do not pass through floating point. Amazon requests quality/media/global-ID/eligibility/parental extras; known song ratings remain independent from album ratings.
- Apple public `trackCount` populates `album.total_tracks` (as reported by the source). Spotify, Deezer and YouTube structured direct lookups reject a returned ID that differs from the requested one. Empty Spotify embed metadata fails safely.
- Debug adds `track.extensions.metadata_comparison` with chosen values, evidence sources and alternatives. Production keeps artist/album improvements and provider data but omits comparison diagnostics.

## Live keyless validation

Started a fresh server with empty disk cache, all inherited `OMMR_*` values removed, no env/session files, credential discovery disabled, 10-second API deadline and US Apple storefront. All lifecycle entries reported absent credentials. No recording-specific results are hardcoded.

| Query | Canonical performers | Apple song ID | YouTube song ID | Apple / YouTube duration | Identity | Request seconds |
| --- | --- | --- | --- | --- | --- | --- |
| Boyfriend — Karan Aujla | Karan Aujla; Ikky | 1829591986 | tIjUfo30r4M | 160796 / 161000 ms | strong | 7.115 |
| Get Lucky — Daft Punk | Daft Punk; Pharrell Williams; Nile Rodgers | 617154366 | 4D7u5KF7SP8 | 369629 / 370000 ms | probable | 0.700 |
| Tu — Talwiinder | Talwiinder; Sanjoy | 1755529616 | uOQG38DriDM | 132086 / 133000 ms | strong | 2.327 |

Before the stronger artist gate, the broader test selected `Get Lucky` by **Funk Punk**, ID `2KPIShmFouY` (250000 ms), alongside Daft Punk's 369629 ms album track. After the fix, the result selects the **Daft Punk** catalog audio with matching collaborators and 370000 ms duration. A regression fixture ensures Funk Punk alone cannot produce a track/ID or trigger hydration for a Daft Punk query. This is a correctness improvement, not a change that forces higher confidence labels.

Boyfriend retains album P-POP CULTURE, 2025-08-22, Pop, preview and artwork. Canonical performers are now separate and retain their YouTube artist IDs; the combined Apple display and its ID remain only in Apple's own provider record. Apple reports an album track count of 5 for this returned entity; that is source/edition metadata, not a verified total across every edition. Its cached production request took 1 ms. Debug payload was 12083 bytes; production 8400 bytes in the observed run.

Tu preserves the Apple album label `Tu - Single` and YouTube's `Tu` alternative. No attempt is made to invent an album ID match solely by stripping suffixes. Separate performers are corroborated by accepted source structure.

These are individual request measurements, not throughput benchmarks. Spotify anonymous search remains rejected; Deezer/MusicBrainz/JioSaavn still returned HTTP 200 HTML access-error pages in this environment. Those services are reported failed and can be suspended after repeated failures. The added authenticated paths were not live-tested with operator credentials. No ISRC, language, credits or label is invented where available sources omit it.

## Checks and regression coverage

`go build ./...`, `go vet ./...`, `go test ./...` and `go test -race ./...` passed. Tests cover:

- shared-name false positives; collaborator scoring; unchanged direct-ID/ISRC gates;
- compound display reconciliation, unsplit band names, ID ownership, source immutability, deterministic ordering and trust overrides;
- detail ID/title/artist/ISRC/duration/album mismatches and unavailable details, preserved original scores/IDs and search-only fields;
- album detail ID/title checks, returned copyright/UPC/unknown extras and no duplicate Deezer direct-detail request;
- JSON:API reference-only search results, type/ID collisions, unrelated included records, precise large IDs and fractional/ISO duration rejection cases;
- current TIDAL track-search path/includes, Amazon song-versus-album parental flags, no-op detail health and short partial-enrichment cache TTL;
- debug-only field comparisons and unchanged production omission of diagnostics.

## Compatibility and remaining limits

Resolved cache namespace is `rich-v7`. Metadata-derived canonical IDs can change when an artist display becomes a corrected structured list. Existing IDs are not a durable provider-independent database key across normalization releases. Field priorities still control canonical selection; source alternatives remain available.

Artist aliases and transliterations without strong evidence can be rejected. Same-title/same-artist recording versions remain ambiguous without stronger identifiers. Public access, regional catalog coverage and authorized app access still determine upstream metadata availability. TIDAL internal-only lyrics/credits are not enabled by this change. Qobuz/Pandora/SoundCloud authenticated coverage is not claimed complete. Extra source data is retained only when associated with the accepted entity.

References: [TIDAL official OpenAPI](https://tidal-music.github.io/tidal-api-reference/tidal-api-oas.json), [Amazon track schema](https://developer.amazon.com/docs/music/API_web_track_v2.html), [ENRICHMENT.md](ENRICHMENT.md), [MATCHING_ENGINE.md](MATCHING_ENGINE.md).

## Expanded validation — 2026-10-17

The [12-song live matrix](LIVE_SONG_MATRIX.md) found and fixed selection of an unrequested remix and a shared-request deadline race that discarded usable results. Explicit version qualifiers now participate in acceptance unless an exact native ID/ISRC establishes identity. Upstream operations leave up to 500 ms for final assembly. Build, vet, full tests and race checks passed again after these changes. Confidence levels are recomputed against the ISRC sources selected by the final metadata policy.
