# OMMR caching

## Backends and configuration

The default disk backend stores files in `OMMR_CACHE_DIR` (default `./cache_data`). Optional Redis uses `OMMR_CACHE_PROVIDER=redis` and `OMMR_REDIS_URL` (default `redis://localhost:6379/0`). A Redis initialization failure falls back to disk. Positive resolved entries use `OMMR_CACHE_TTL`, default **24h**, subject to shorter partial/error and source freshness caps. Accepted YouTube/SponsorBlock attempts cap resolved entries to one hour with POI, five minutes without POI, or 30 seconds when unavailable. Shorter existing caps still apply; raw and resolved lifetimes can compound.

Config is loaded from process variables prefixed `OMMR_`, an explicitly selected private `OMMR_ENV_FILE`, or a private YAML `OMMR_CONFIG_FILE`. A plain `.env` file is not loaded automatically. A local owner-only `ommr.yaml` is now auto-discovered unless explicitly overridden. Process variables take precedence over the selected YAML file; YAML takes precedence over defaults read from the env file. Prefer selecting one file format.

## What is cached

| Layer | Actual integration | Lifetime |
| --- | --- | --- |
| Resolved responses | Complete internal response, including provider metadata and diagnostics | `OMMR_CACHE_TTL` (24h default) |
| Healthy no-result responses | Empty resolutions where every attempted provider succeeded | 30 seconds |
| Catalog enrichment GET JSON | Cover Art Archive, release details, Last.fm and Discogs requests through the catalog client | 24 hours |
| Normalized candidates | Backend directory exists, but provider candidate caching is not implemented | None |
| Native albums | Exact source edition, bounded track list, graph | Complete 5 minutes; partial 30 seconds |
| Artist profiles | Independent source profiles and ambiguity | Success 5 minutes; partial/ambiguous/no-result at most 30 seconds |
| YouTube worker | Bounded public entity jobs | Normally 6 hours; partial song at most 30 seconds |
| SponsorBlock raw buckets | Highlight point hash-prefix GET | Success 1 hour; 404 emptiness 5 minutes |
| Fingerprint POST | Not cached | None |

Provider errors are not negative-cached. Catalog JSON with top-level `error`/`errors` or `status=error` is not cached, including HTTP 200 error envelopes. Source-specific identity checks are repeated after cache reads.

## Keys and isolation

Resolved keys use `resolved:rich-v17:` plus a policy/enrichment-capability digest and a hash of the normalized query and unique sorted source set. Changing merge policy or enabled enrichment names invalidates the previous presentation. Earlier `links-v1` entries expire naturally.

Raw catalog keys use `raw:catalog:` plus a SHA-256 digest of provider, endpoint and authorization value. Keys contain no plaintext API keys or tokens. Cached JSON contains the provider's response, never request headers. Operators should secure disk/Redis access as metadata responses can be large and include provider context.

`debug` is an HTTP output mode and does not fragment the cache. Production and debug requests reuse the same complete result. Every caller receives independently decoded maps/slices; response projection does not mutate cached data.

## Concurrent requests and cancellation

Within one server process, identical cache-enabled resolutions share one in-flight operation, including identical items in bulk requests. Canceling one waiter returns immediately without canceling the shared provider work. Shared work has the initiating request's remaining deadline, or 15 seconds if it has none. Different queries and different processes do not share this in-flight group.

`bypass_cache=true` skips resolved-cache reads, refreshes the resolved entry, and uses a separate in-flight group from ordinary requests. It does not bypass the catalog GET cache. Data remains in the raw catalog cache for its TTL; a full purge currently requires clearing the configured backend. No stale-while-refresh, distributed locking or adaptive retry is implemented. HTTP gzip is a presentation/transport step after caching; it does not change the cache representation. See [production metadata transport](DETAILED_PRODUCTION_METADATA.md#lossless-production-transport).

Provider budgets are local to each process. Deployments with multiple replicas need an aggregate budget or shared upstream gateway.

## Provider lifecycle partitions

Resolved-cache keys additionally include the provider-health state/mode signature, including recovery eligibility. A circuit transition prevents writes into the old partition. Provider-failure partial results use at most 30 seconds of resolved TTL. Startup probes use uncached source clients. Safe health status describes selection/operation, not credential contents. See [PROVIDER_LIFECYCLE.md](PROVIDER_LIFECYCLE.md).

Accepted-provider detail failures, detail identity mismatches and unavailable post-merge catalog enrichments also cap positive resolved caching at 30 seconds. A temporarily sparse result does not freeze missing detail fields for the configured full TTL. No-op detail lookups do not reset provider health. Artist representation changes may change metadata-derived canonical IDs; The current rich-v17 namespace also invalidates older combined-artist, mixed-edition and pre-POI results.

Artist profile source results use `artists:v1` keys: five-minute successes, at most 30-second partial/ambiguity/no-result entries, no error caching. Source availability/health is checked before serving cached data; observation times remain unchanged on hits. Explicit bypass refreshes worker and native lookups; YouTube audience profile misses bypass the worker entity cache to avoid relabeling stale counters.
