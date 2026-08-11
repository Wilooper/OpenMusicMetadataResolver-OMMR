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
