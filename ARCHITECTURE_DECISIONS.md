# Architecture Decision Records (ADR)

## ADR-001: 6-Stage Music Identity Resolution Pipeline & Trust Weights
- **Status**: Approved
- **Context**: Resolving cross-platform identities (e.g. YouTube Video ID to Apple Music, Deezer, MusicBrainz, and ISRC) requires structured pipeline stages, provider trust coefficients, and method provenance to prevent weak matches from polluting identity graphs.
- **Decision**:
  1. Implement a 6-stage discovery pipeline (Direct Lookup -> ISRC Query -> Cleaned Metadata Search -> Album Verification -> Duration Delta Filter -> Final Merge).
  2. Assign provider trust weights: MusicBrainz (1.00), ISRC (1.00), Apple Music (0.95), Spotify (0.95), Deezer (0.90), YTMusic (0.80).
  3. Emit `IdentityMatch` objects (`provider`, `id`, `confidence`, `method`) and construct `IdentityGraph`.
  4. Use nullable `*float64` pointers for `MatchBreakdown` to distinguish `unknown` (null) from `mismatch` (score < 1.0).

## ADR-002: Zero-Key Provider Strategy
- **Status**: Approved
- **Context**: Requirements dictate self-hostable operation without mandatory developer credentials.
- **Decision**: Integrate unauthenticated web oEmbed and public endpoints for Spotify, YouTube, Deezer, Apple Music, and MusicBrainz with token-bucket rate limiting.

## ADR-003: Spotify Dual-Mode Operation (Official + Zero-Key Fallback Chain)
- **Status**: Approved
- **Context**: Spotify's Web API returns the richest metadata (genres, labels, ISRC), but requires OAuth credentials. The service must remain usable without keys.
- **Decision**: Implement a dual-mode Spotify adapter (`spotify-web-v2`):
  1. **Official mode**: when `OMMR_SPOTIFY_CLIENT_ID` / `OMMR_SPOTIFY_CLIENT_SECRET` are configured, use the OAuth client-credentials flow and the Web API.
  2. **Zero-key mode** (default): fetch an anonymous web-player token from `https://open.spotify.com/get_access_token`, call the Web API; if unavailable, fall back to scraping the embed page's `__NEXT_DATA__`, then the public oEmbed endpoint.
- **Consequence**: Metadata resolution never blocks on missing credentials; a key is an optional reliability/richness upgrade.

## ADR-004: Provider Expansion — SoundCloud & JioSaavn (Shazam Rejected)
- **Status**: Approved
- **Context**: More providers increase cross-platform identity consensus and metadata richness (labels, play counts, preview URLs).
- **Decision**:
  1. Add a **SoundCloud** adapter (`soundcloud-oembed-v1`) via the public oEmbed endpoint; accepts `soundcloud_id` permalinks. No text search in zero-key mode.
  2. Add a **JioSaavn** adapter (`jiosaavn-search-v1`) via the public `api.php` endpoints (`search.getResults`, `song.getDetails`), contributing song IDs, labels, play counts, and preview URLs.
  3. **Shazam was evaluated and rejected**: its public web search endpoint now returns HTTP 405 for non-browser clients, so it is not usable as a zero-key source.
  4. Extend provider trust weights: `jiosaavn: 0.85`, `soundcloud: 0.75`; add per-provider rate limits (soundcloud 10 RPS, jiosaavn 5 RPS).

## ADR-005: Strict Candidate Acceptance (Wrong-ID Fix)
- **Status**: Approved
- **Context**: Resolutions could return wrong track/video IDs and wrong metadata. Root causes: (a) the matcher compared the target title against candidate *album*; (b) identity matches were emitted for rejected candidates; (c) all sub-threshold candidates were force-merged when nothing matched; (d) a title-only match on a different artist (e.g. another "Get Lucky") leaked its IDs/label.
- **Decision**:
  1. Fix the matcher to compare `target.Album` vs candidate album and use composite artist/album scoring (`0.6*JaroWinkler + 0.4*TokenSetRatio`).
  2. Emit `IdentityMatch` only for accepted candidates.
  3. Never force-merge sub-threshold candidates; surface at most the single best candidate (minimum `0.30`), otherwise return no track. `rejectThreshold = 0.40`.
  4. Add an **artist gate**: when the query carries an explicit artist, candidates with primary-artist composite score `< 0.45` are rejected regardless of combined score.
  5. Deduplicate `sources` in the merger.

## ADR-006: Canonical Schema Extension
- **Status**: Approved
- **Context**: Richer providers expose fields not previously modeled (ISWC, preview URLs, track/disc numbers, labels, barcodes, copyrights, play counts).
- **Decision**: Extend the canonical `Track` schema with `ISWC`, `PreviewURL`, `TrackNumber`, `DiscNumber`, `Label`, `Barcode`, `Copyrights`, `PlayCount`; add a `Copyright` type; extend `Album` with `Label`, `UPC`, `Barcode`, `TotalTracks`, `Copyrights`; extend `IdentityGraph` with `SoundCloudID` and `JioSaavnID`. Re-balance `CalculateCompleteness` weights to sum to 1.0.

## ADR-007: YouTube Search via InnerTube (Zero-Key)
- **Status**: Approved
- **Context**: ytmusic's oEmbed-only flow could not search by artist/title and returned sparse player data.
- **Decision**: Use YouTube's public InnerTube endpoints (`/youtubei/v1/search` and `/youtubei/v1/player`) with the public web API key to enable zero-key artist/title search and rich player metadata, with oEmbed retained as fallback (`ytmusic-innertube-v1`).

## ADR-008: Trusted Seed for Cross-Provider Identity
- **Status**: Approved
- **Context**: When resolving a platform ID, cross-provider candidates could be merged before the comparison query was constructed. A higher-priority provider could replace the directly fetched track's title and artist, allowing unrelated platform IDs to appear as matches.
- **Decision**:
  1. For a platform-ID request, seed resolution only from the candidate returned by the matching provider with the exact requested ID. For an ISRC request, seed only from candidates with that exact ISRC.
  2. Build the cross-provider search and comparison query from this seed before evaluating secondary candidates.
  3. Require both title and artist component scores of at least `0.72` for cross-provider candidates when resolving from a platform ID; an exact seed ISRC is accepted as stronger identity evidence.
  4. Reject conflicting ISRCs and do not treat another provider's claimed ID namespace as a direct match.
- **Consequence**: A weak or unavailable seed can produce no cross-platform IDs, but a secondary provider cannot redefine the requested recording and contaminate its identity.

## ADR-009: Modular Provider Extraction and Credential-Gated Catalog Sources
- **Status**: Approved
- **Context**: A merged canonical record obscures what each catalog actually returned, while new commercial providers have different access requirements and metadata coverage.
- **Decision**:
  1. Add `/v1/extract` to return per-provider normalized track data, release year, thumbnail URL, and explicit present/missing field coverage without merging provider responses.
  2. Add separate Qobuz, TIDAL, Amazon Music, and Pandora adapters and keep source IDs namespaced to their provider.
  3. Use provider-issued credentials for gated APIs; do not scrape provider pages or invent missing credits. Test adapter requests and normalization with deterministic JSON fixtures.
- **Consequence**: Source comparison is observable and reproducible. Live tests against credential-gated services require valid access and catalog availability may differ by territory.

## ADR-010: Rich Catalog Credentials and Work-Linked Context
- **Status**: Approved
- **Context**: Catalog providers vary in documented fields and access; free-text matching can assign a video or article to the wrong recording.
- **Decision**: Keep keyless lookup available and add operator-supplied official Spotify client credentials and Apple Music developer token, optional YouTube browser cookie for its unofficial InnerTube interface, and Amazon closed-beta credentials. Read private env/config files only with owner-only permissions; restrict credentialed hosts and redirects. MusicBrainz work relations provide explicit lyricist/composer roles. Wikipedia context is fetched only from a linked work article and carried in `extensions.wikipedia`; it cannot assert an ID match. `/v1/extract` exposes a field inventory for each source.
- **Consequence**: Optional richer fields require authorized credentials and catalog availability. Context can be absent even for a known recording; no inferred lyricist or song history is fabricated.


## ADR-011: Enrich Only Accepted Recording Identities
- **Status**: Implemented
- **Context**: MusicBrainz search responses omit work relationships, so normal resolution could not provide the credits and article context available from direct recording extraction.
- **Decision**: Perform a rate-limited, time-bounded recording lookup only for an accepted MusicBrainz candidate. Require the same MBID, title, primary artist and known ISRC before attaching credits, ISWC, language or extensions. Do not replace recording identity or add IDs through enrichment. Report enrichment outcome separately from lookup success. Preserve the requested ISRC when a recording reports multiple codes. Fetch article context only for a single linked work; reject disambiguation summaries and conflicting Wikidata IDs. Retain revision/entity attribution when available. Use a new resolved-cache namespace for enriched results.
- **Consequence**: Resolution adds a bounded lookup for accepted MusicBrainz records. Lookup failure preserves the accepted result with sparse metadata. Multiple-work recordings retain linked credits but omit ambiguous singular composition fields and context. Fixture tests verify requests and identity constraints; live credentialed service equivalence remains unverified.


## ADR-012: Provider-Owned Recording Links
- **Status**: Implemented
- **Context**: IDs alone do not tell callers how to open a matched song, and lookup permalinks/tokens may differ from native catalog IDs. Generic search links could falsely imply a recording match.
- **Decision**: Add a pure platform-link formatter and `/v1/links`, sharing the resolver's strict acceptance pipeline. Keep the Track schema immutable and attach the catalog in `extensions.platform_links`; per-source extraction exposes only its own links. Merge only provider-owned ID namespaces and count only primary IDs as IdentityMatches. Label ID types, preserve native IDs alongside lookup identifiers, validate provider URLs against the exact recording and host, remove known tracking parameters, and omit unknown URLs. Report missing platforms and cross-search failures explicitly. Version resolved cache keys.
- **Consequence**: Catalog coverage and authorized access determine which links can be returned. Format-valid ID-derived URLs are not live playback guarantees. Pandora and JioSaavn can return IDs without fabricated slug-based URLs.


## ADR-013: Compact Production Responses with Opt-In Diagnostics
- **Status**: Implemented
- **Context**: Returning execution statuses, field provenance and identity scoring with every metadata response increases payload size and couples application consumers to resolver internals.
- **Decision**: Parse an optional boolean `debug` query parameter on data endpoints. Default responses use explicit public DTOs; `debug=true` retains the detailed canonical output. Project data only after resolution, retaining a single complete cache entry for both modes. Keep useful catalog IDs/links and contextual attribution. Apply the flag to resolve, links, bulk, search and independent extraction. Reject invalid/ambiguous flags before provider work. Do not alter matching gates or introduce hosting policy settings in this change.
- **Consequence**: Production payloads are smaller while debug cache hits retain diagnostics. Existing consumers of diagnostic fields and the extraction source envelope must opt into debug. Cache storage size and cache-miss provider work do not decrease. Debug remains caller-selectable until a later hosting configuration feature.


## ADR-014: Preserve Source Metadata and Separate Recording Identity from Context
- **Status**: Implemented
- **Decision**: Keep accepted provider normalized alternatives and exact-entity public extras in `extensions.provider_data` (promoted in production). Remove credentials, tracking, player and unrelated search data. Default scalar merging to deterministic one-provider/one-vote consensus, with explicit configurable field trust. Retain an `explicit_status` tri-state without changing the legacy boolean. Album IDs are combined only for same-title accepted album records.
- **Consequence**: Extra fields survive even when they lose canonical selection; context sources cannot assert new recording IDs. Production payloads can grow with useful data while diagnostics remain opt-in.

## ADR-015: Bounded Catalog Enrichment and Shared Resolution Work
- **Status**: Implemented
- **Decision**: Add Cover Art Archive, MusicBrainz release, optional Last.fm and Discogs post-merge services; expose AcoustID fingerprint candidates separately. Enforce exact identity/scope checks, provider budgets, bounded bodies/timeouts, credential isolation and restricted artwork redirects. Cache catalog GET JSON and healthy empty resolutions. Share in-flight cache-enabled resolutions and clone results per caller. Include policy/capabilities in cache namespaces. Correct the iTunes budget and route search through rate limiting.
- **Consequence**: Repeated upstream work is reduced within one process. Access permissions, source coverage, different editions, distributed budgeting and upstream pagination remain real constraints. No credentialed live equivalence is claimed without operator keys.


## ADR-016: Private Session Import and Access-Gated Provider Bootstrap
- **Status**: Implemented
- **Decision**: Discover only documented owner-only credential files, support browser-session imports for Spotify/Apple/YouTube, and validate actual source access before selecting supplied modes. Try supported public fallbacks without equating them to authentication. Omit missing/failed required-key sources. Keep request probes bounded/uncached and lifecycle output secret-free.
- **Consequence**: Credentialed source registration reflects tested access. Public modes are monitored during use. Spotify web challenges and Apple's bearer requirement remain external constraints; there is no browser login UI or live credential reload.

## ADR-017: Provider Circuits and Startup API Policy
- **Status**: Implemented
- **Decision**: Suspend repeatedly failing sources, admit one cooldown recovery probe, and ignore stale-generation results. Exclude unavailable sources from selection and isolate cache partitions across health changes. Auto-load private YAML, allow explicit --config, apply API defaults/locks to all routes, and provide a JSON gateway plus redacted provider status.
- **Consequence**: Clients can use configured behavior without repeated URL controls. Source objects remain resident while suspended; no dynamic code unloading occurs. Source policy and status are local to each process.


## ADR-018: Prefer Music Catalog Search and Reject Non-JSON Access Pages
- **Status**: Implemented
- **Context**: The credential-free Boyfriend live test accepted a noisy video title while the public Apple request expired at five seconds. Other origins returned HTTP 200 HTML access-error pages.
- **Decision**: Prefer signed-out YouTube Music song-filter results, parse artist/album navigation evidence and audio-track types, and retain same-entity catalog extras. Keep general video search as a fallback. Raise five-second Apple/Deezer/MusicBrainz HTTP safety caps to fifteen seconds while retaining the configured resolver deadline. Apply storefront to public Apple requests. Reject non-JSON/oversized original adapter responses with body-free diagnostics. Use Spotify's current token path and report anonymous rejection without circumventing access controls. Bump resolved-cache namespace to rich-v3.
- **Consequence**: Keyless metadata quality improves where origins are accessible. HTTP 200 does not imply catalog success. Spotify text search can still require validated access; host/network restrictions remain visible. Identity gates are unchanged, and cached weak video results do not mask the new song search.


## ADR-019: Evidence-Backed Artists and Guarded Detail Hydration
- **Status**: Implemented
- **Context**: Combined artist displays discarded structured collaborator IDs; search-only responses omitted detail fields. A live Daft Punk Get Lucky query accepted an unrelated Funk Punk recording through fuzzy name similarity. Authenticated catalog envelopes and duration formats also differed from the generic parser assumptions.
- **Decision**: Require stronger normalized artist-name evidence on pure text inputs without weakening existing thresholds/direct-seed/ISRC rules. Score all structured contributors. Reconcile compound displays only against accepted structured lists; transfer native IDs only between exact individual names. Hydrate accepted Deezer tracks and Spotify albums concurrently with strict recording/edition checks, preserving source-only fields and original identity scores. Parse scoped JSON:API resources/includes and fractional/ISO durations without numeric ID loss; request Amazon extras. Expose comparisons in debug only. Cap failed enrichment caching at 30 seconds and use rich-v4 resolved keys.
- **Consequence**: Wrong-artist covers are rejected, structured metadata is more useful, and temporary detail failures retain a retry path. Stronger artist evidence can reject unsupported aliases. Corrected artist representation can change metadata-derived canonical IDs. Fixture-tested authenticated paths still require real access before live equivalence can be claimed.

## ADR-020: Preserve recording versions and deadline headroom (2026-10-17)

Expanded keyless testing selected a labelled remix for an unqualified Tum Hi Ho query and exposed a deadline race in shared resolution. Acceptance now requires matching explicit version qualifier categories unless exact native-ID/ISRC evidence establishes identity; the existing numeric acceptance and artist thresholds remain unchanged. Upstream discovery/detail/catalog operations share a deadline ending up to 500 ms (20% on short budgets) before the response deadline, leaving room to merge and return available evidence. A slow source cannot consume the complete serialization budget. Tests cover both regressions. Resolved cache namespace advances to rich-v5 so old accepted variants are not reused.

## ADR-021: Native public catalogs and bounded language workers (2026-10-18)

Bugs, Genie and Melon use bounded native Go HTML/JSON-LD adapters with accepted-candidate detail hydration, numeric-ID verification, native artist ownership, per-source rate limits and existing circuits. The optional ytmusicapi integration runs in persistent JSON-lines processes behind the Go API, with bounded workers/queue, deadlines, response caps, coalescing and Go caching. Successful partial context survives sibling failures and is retried after a short cache TTL. Artist/album context remains provider data, never proof of another recording ID. The optional `/v1/catalog` route and JSON operation share endpoint policy. Python public mode does not claim authenticated capability. Resolved cache advances to rich-v6 and includes enabled Python/profile capability to prevent sparse caches masking richer results. CPU/throughput gains are unmeasured.

## ADR-022: Recover catalog HTML without weakening identity (2026-10-18)

JioSaavn, Deezer and MusicBrainz accept known native JSON envelopes embedded in HTML, provider-owned JSON-LD music records and scoped MusicBrainz search rows through a shared bounded Go decoder. Normal API JSON remains native; credential probes remain strict. API HTML failure can use a bounded official public-page fallback with shared source budgets and a failed-route cooldown. Data-free error pages remain failures rather than empty successes or invented metadata. Recovered records retain source extras and pass unchanged identity gates. Unknown native entity IDs are omitted. Resolved cache advances to rich-v7. Live access remained unavailable in this environment; supported parsing was verified using fixtures.


## ADR 023: JioSaavn v4 protocol compatibility (2026-10-18)

Implement the public request protocol used by Python JioSaavn clients directly in Go. Accept legacy flat details and v4 nested search records, retaining structured artists, credits and accepted catalog extras. A language/runtime switch cannot repair an identical empty upstream page. Keep connectivity diagnostics separate from claims of successful provider recovery; no proxy fallback is introduced. Resolved cache advances to rich-v8.


## ADR 024: Isolated provider diagnostics and JioSaavn identifier routing (2026-10-18)

Add a standalone Go providercheck command that observes real public adapters without bootstrap credentials, resolver acceptance or caches. Reports distinguish decoded candidates from HTTP success, record bounded/redacted HTTP evidence, and optionally compare fixed source-specific routes with a shared one-request-per-second service budget. The production server does not acquire a network-probe endpoint.

JioSaavn native IDs use song.getDetails; public share tokens use webapi.get with type=song. Support keyed and songs-array detail envelopes, require one matching identity, and hydrate accepted records using explicitly typed IDs. GET extraction and the JSON gateway accept the same validated native/share selector. Keep search/identity thresholds unchanged and advance resolved caching to rich-v9. Neither a passing fixture nor a decoded search candidate proves cross-provider identity or restores blocked live access.
