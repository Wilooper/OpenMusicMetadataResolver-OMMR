# OMMR limitations and metadata options

Review date: 2026-10-16. Findings below are research only; no new providers, rate-limit changes, or merger/cache policies are implemented by the compact-response change.

## Verified in the current code

| Finding | Evidence | Impact / possible later work |
| --- | --- | --- |
| iTunes budget mismatch | `internal/ratelimit/provider_limiter.go` configures `applemusic` at 20 requests/second, burst 5; Apple documents approximately 20 Search API calls/minute, subject to change [1] | Separate iTunes and token-backed Music catalog budgets; honor upstream throttling. Local settings are not evidence of provider-approved quotas. |
| MusicBrainz permits an initial burst | Same limiter uses 1 request/second, burst 2 | Compare exact request spacing with MusicBrainz's published rules [2]; multiple server instances need a shared budget. |
| Search bypasses provider limiter | `internal/api/handler.go` calls adapters directly in `HandleSearch` | Search traffic can exceed the intended local budgets; enrichment/internal adapter calls also need request-level accounting. |
| Source trust is not configurable | `internal/merger/merger.go` has package-level priority lists; `internal/config/config.go` has no per-field trust policy | The previously discussed environment-controlled trust/majority resolution is not implemented. Design consensus per field, with independent-source accounting. |
| Some merge outcomes depend on arrival order | Resolver appends concurrent candidates; merger chooses the first populated duration, release date and language and the first candidate's explicit flag | Conflicts can produce inconsistent metadata. Define deterministic policy before adding more sources. |
| Unknown and clean explicit values are conflated | Canonical `Track.Explicit` is a bool; missing and false both become false | A future optional/tri-state value would distinguish unknown from confirmed clean. Keep compatibility in mind. |
| Shared cache stores the complete result | Resolver caches the full `ResolveResponse` under a query/provider-set key | Compact HTTP output reduces transfer, not stored bytes. No request coalescing, negative caching or background refresh is present in the resolver. Repeated simultaneous misses may repeat upstream work. |
| Layered cache directories exceed actual resolver usage | Disk backend supports raw/normalized/resolved paths, but resolver integration reads/writes final resolved entries | A directory layout alone does not mean provider responses are cached independently. Investigate reusable per-provider cache entries later. |
| Search pagination is local | Search gathers each adapter's bounded results and then slices that finite pool | Later pages do not fetch the next upstream page; results are not deduplicated cross-provider recordings. |
| Credential-gated providers cannot guarantee keyless coverage | TIDAL, Qobuz, Amazon Music and Pandora adapters return access/configuration errors without their required credentials | Registering an adapter does not establish live access or matching catalog coverage. Current fixture tests do not prove authorized live equivalence. |
| Spotify cookie login is absent | Config/token manager supports official client credentials plus anonymous/embed fallbacks; no Spotify cookie config exists | The previously requested cookie mode is not in this branch. Anonymous web endpoints are fragile and cannot guarantee rich metadata. |
| SoundCloud keyless search is absent | `internal/adapters/soundcloud/provider.go` supports permalink oEmbed lookup; text search returns no candidates | Link lookup can provide basic metadata, but broad text resolution needs an authorized search integration. |
| Region and edition handling are limited | Apple catalog storefront is configurable but defaults to `us`; keyless lookup and generated song links do not provide a full territory-aware resolver | Availability, remaster/live/clean editions and release dates may differ by region. Provider names/IDs are not playback guarantees. |
| Extraction is independent, not identity verified | `/v1/extract` normalizes each provider response without the resolver's cross-provider acceptance pipeline | Production extraction gives a flat list of independent records; use debug to compare provenance and failures. Do not treat every extraction record as the same recording. |

## Ways to obtain richer metadata

These are options for discussion, not promised coverage or implemented integrations.

| Option | What it could add | Requirements and matching constraints |
| --- | --- | --- |
| Cover Art Archive [3] | Release/release-group cover images, image types and thumbnails | Join via verified MusicBrainz release IDs. A recording may belong to several releases with different artwork. Keyless read API; catalog gaps remain. |
| Deeper MusicBrainz release/work lookups [4] | Release-specific label/barcode/edition data and explicit work/composer/lyricist relationships | Build on existing recording/work enrichment. Preserve recording vs composition vs release identity; schedule lookups within the provider budget. |
| Discogs [5] | Release/artist/label catalog data; investigate edition-specific credits, genres/styles and packaging | Optional authenticated integration; establish recording-to-release/track mapping. This review verified the official API overview, but the detailed developers page returned 403, so field/limit details need further validation before design. |
| Last.fm `track.getInfo` [6] | Tags, listener/play counts and editorial/wiki context | API key required. User tags should be enrichment rather than proof of recording identity. API supports artist/title or MBID lookup; retain attribution. |
| AcoustID + Chromaprint [7] | Identify a recording from an audio fingerprint and retrieve MusicBrainz recording/release links | Requires audio/fingerprint plus duration and an application key; not a text-only replacement. Free service is non-commercial and documents a 3 requests/second ceiling. Fingerprint computation adds client/server work. |
| Better use of existing catalog detail endpoints | Fields omitted by search responses, such as album detail and contributor roles where supported | Fetch detail only after identity acceptance; measure request cost and verify exact fields against authorized live responses before adding normalization. |

Suggested discussion order: fix budgets and deterministic merging first; then provider cache reuse/request coalescing; then evaluate Cover Art Archive and richer release/work enrichment. Add sources only when their identity mapping and unique field coverage justify the request cost.

## Sources

1. Apple Search API: https://performance-partners.apple.com/search-api (approximately 20 calls/minute; subject to change).
2. MusicBrainz rate limiting: https://musicbrainz.org/doc/MusicBrainz_API/Rate_Limiting
3. Cover Art Archive API: https://musicbrainz.org/doc/Cover_Art_Archive/API
4. MusicBrainz API: https://musicbrainz.org/doc/MusicBrainz_API
5. Discogs API overview: https://www.discogs.com/developers/
6. Last.fm track.getInfo: https://www.last.fm/api/show/track.getInfo
7. AcoustID web service: https://acoustid.org/webservice

Code findings are from inspection of this checkout, not live provider load tests. External documentation was checked during this review; detailed Discogs documentation and the MusicBrainz rate-limit page could not be directly opened, so no new numerical quota is asserted for them here.
