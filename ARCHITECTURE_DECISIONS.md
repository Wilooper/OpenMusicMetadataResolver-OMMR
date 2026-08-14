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
