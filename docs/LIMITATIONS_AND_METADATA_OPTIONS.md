# Implementation status and remaining limits

Updated 2026-10-16. This replaces the earlier research-only assessment with the actual implementation status. See [ENRICHMENT.md](ENRICHMENT.md) for exact source scopes and [CACHING.md](CACHING.md) for storage behavior.

## Implemented

| Earlier limitation | Current behavior |
| --- | --- |
| Incorrect iTunes request budget | Keyless Apple calls are budgeted at one every 3 seconds; catalog-token mode has a separate conservative budget |
| MusicBrainz burst and search limiter bypass | Burst is 1; public search now goes through the resolver's provider limiter |
| Hardcoded-only trust | `OMMR_METADATA_POLICY` selects consensus/priority and explicit per-field provider preferences |
| Arrival-order-dependent merge | Stable source ordering, one accepted candidate per provider, deterministic scalar choices and collection ordering |
| Unknown explicit conflated with clean | Compatibility bool plus `explicit_status`: unknown/clean/explicit; missing flags do not vote |
| Extra provider fields discarded | Accepted normalized alternatives and exact-entity public catalog extras are retained in provider data |
| Simultaneous identical misses duplicate work | Cache-enabled resolution shares one operation per normalized query/source set within a process; canceled waiters do not cancel it |
| No-result queries repeatedly hit providers | Healthy empty results are cached for 30 seconds; failed provider responses are not treated as healthy negatives |
| Raw cache directories only aspirational | New catalog GET services actually cache JSON for 24h; general normalized/provider adapter caching is still absent |
| No additional artwork/context | Cover Art Archive and MusicBrainz release enrichment; optional Last.fm/Discogs context; AcoustID fingerprint candidates |
| Config docs differ from code | Correct `OMMR_` names, 24h TTL, cache_data directory, 50 RPS API budget, 10s operation timeout, and explicit file selection; credentialed adapters now receive values loaded from those files |
| Unsafe large-page arithmetic | Local search pagination now avoids integer overflow and sorts ties deterministically |

## Real remaining constraints

- Credentialed catalog access still requires provider-issued keys/tokens. Optional adapters do not imply live access to every catalog. Spotify now supports imported browser sessions and a bounded cookie exchange; challenge-dependent refresh is not guaranteed. Apple session cookies still need a developer/browser bearer. See PROVIDER_LIFECYCLE.md.
- SoundCloud's keyless integration is permalink lookup, not broad text search. Search still paginates a finite per-provider result pool and can contain the same recording from several providers; it does not fetch arbitrary upstream pages.
- ISRC/title/artist evidence is imperfect for live, remastered, clean and regional editions. Existing identity gates remain; adding context does not weaken them. Canonical IDs use the existing metadata-based identity scheme and can still conflate versions with identical cleaned title/artist.
- Source consensus counts catalogs, not guaranteed independent editorial origins. Durations vote exactly, so small timing differences use deterministic tie-breaking rather than clustered voting. Caller trust overrides and preserved alternatives make disagreements inspectable.
- Local provider budgets and in-flight sharing are not distributed across replicas. Some existing adapter-internal fallback/detail calls still need transport-level quota accounting. No automatic Retry-After backoff/circuit breaker is implemented.
- `bypass_cache` refreshes resolved entries but does not bypass the 24h catalog GET cache. Normalized candidate caching and reusable raw caching for the original identity adapters are not implemented. Cache TTLs are not a guarantee that deleted provider data disappears immediately from deployments.
- Broader country/region selection is incomplete. Apple storefront is configurable; other provider markets may be fixed or inferred. Country/territory fields remain source data rather than a single inferred root value.
- Provider data is intentionally richer and can enlarge production responses. Diagnostics remain hidden by default, but no field-selection API or HTTP compression was added in this change.
- Sources can omit metadata, fail, or return no matches. Extra data is preserved only when associated with an accepted entity; unrelated search results/player/auth payloads are not echoed. Body/depth/time limits bound resource use.
- No operator credentials were available for live Last.fm, Discogs search, or AcoustID matching checks. Live probes verified endpoint behavior, while complete source/identity/error paths are fixture-tested. The Cover Art Archive redirect was observed; Archive.org retrieval timed out in this environment.

## Official references

- [Apple Search API](https://performance-partners.apple.com/search-api)
- [MusicBrainz API](https://musicbrainz.org/doc/MusicBrainz_API)
- [Cover Art Archive API](https://musicbrainz.org/doc/Cover_Art_Archive/API)
- [Last.fm track.getInfo](https://www.last.fm/api/show/track.getInfo)
- [Discogs developers](https://www.discogs.com/developers/)
- [AcoustID web service](https://acoustid.org/webservice)

## Startup/lifecycle update

Implemented credential discovery, actual boot probes, public fallback, required-source exclusion, runtime suspension/recovery, endpoint/default API policy and a configured JSON gateway. Unavailable optional sources no longer incur ordinary resolution attempts. Runtime circuits do not solve distributed budgeting or guarantee that browser session refresh remains compatible with private web interfaces. Documentation: [PROVIDER_LIFECYCLE.md](PROVIDER_LIFECYCLE.md).
