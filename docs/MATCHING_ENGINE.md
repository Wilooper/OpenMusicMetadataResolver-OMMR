# OMMR Matching Engine Architecture

## String Similarity Algorithms (`internal/matcher/string_sim.go`)

The matching engine uses a composite string similarity score combining two string metrics:

1. **Jaro-Winkler Distance**: Evaluates character transposition and prefix agreement.
2. **Token Set Ratio**: Tokenizes title/artist strings into sorted sets, ignoring word order variations (e.g. `AP Dhillon, Gurinder Gill` vs `Gurinder Gill & AP Dhillon`).

```go
componentScore = 0.6 * JaroWinkler(target, candidate) + 0.4 * TokenSetRatio(target, candidate)
```

Composite field scoring avoids the pitfall where Jaro-Winkler alone over-scores unrelated strings (e.g. `test song` vs `totally unrelated` = 0.517).

---

## Field Weight Distribution

Weights are normalized depending on album context availability:

| Context | Title | Artist | Album |
| --- | --- | --- | --- |
| Query has album | **0.50** | **0.40** | **0.10** |
| Query without album | **0.55** | **0.45** | — (weight re-normalized) |

```go
if albumAvailable {
    rawScore = 0.50*title + 0.40*artist + 0.10*album
} else {
    rawScore = 0.55*title + 0.45*artist
}
```

*Non-ISRC exact matches are capped at `0.96` to reserve `1.00` for verified ISRC matches and direct platform-ID matches.*

### Direct Platform-ID Match

When the query carries a platform ID (e.g. `youtube_id`) and a candidate's provider ID matches it exactly, the candidate is scored at `1.0` with full `MatchBreakdown` — a verified direct lookup.

---

## Strict Candidate Acceptance

The resolver (`internal/resolver/resolver.go`) gates candidate acceptance beyond the raw score:

1. **Score threshold**: `match_score < 0.40` → rejected (`rejectThreshold`).
2. **Artist gate**: when the query carries an explicit artist, candidates whose primary-artist composite score is `< 0.45` are rejected, even if the overall score passes. This blocks wrong-artist title matches (e.g. a different artist's "Get Lucky" scoring 0.55 on title alone).
3. **No forced merge**: sub-threshold candidates are never merged; if nothing clears the threshold only the single best candidate is surfaced (minimum `0.30`), otherwise the request returns no track.
4. **ID-seeded resolution**: the provider result whose provider name and ID exactly match the requested platform ID supplies the trusted title, artist, album, and ISRC. Cross-provider results cannot redefine that comparison target. Their title and artist scores must each reach `0.72` to contribute IDs, unless they have an exact matching ISRC.
5. **ISRC conflict**: when both the trusted query and a candidate have an ISRC, a mismatch rejects the candidate. A direct-ID match is valid only when the ID belongs to that candidate's own provider namespace.

---

## Provider Trust Weights

Match candidate identity confidence is scaled by provider trust coefficients:

| Provider Adapter | Trust Weight | Rationale |
| --- | --- | --- |
| `MusicBrainz` | **1.00** | Open community music encyclopedia with strict metadata moderation |
| `ISRC` | **1.00** | International Standard Recording Code authority |
| `Apple Music` | **0.95** | High-quality curated catalog metadata |
| `Spotify` | **0.95** | Official commercial streaming metadata |
| `Deezer` | **0.90** | Commercial streaming catalog metadata |
| `JioSaavn` | **0.85** | Commercial streaming catalog metadata (search-sourced) |
| `YouTube Music` | **0.80** | User-uploaded content and audio/video title noise |
| `SoundCloud` | **0.75** | Community uploads; titles/artists frequently non-canonical |

```go
IdentityConfidence = CandidateMatchScore * ProviderTrustWeight[provider]
```

---

## Nullable `MatchBreakdown` (`unknown != mismatch`)

Un-evaluated fields in `MatchBreakdown` serialize as `null` / omitted pointers (`*float64`). A `null` value indicates that the field was not available in the candidate, distinguishing missing data from active field mismatches.

## Metadata conflict selection

Acceptance thresholds and artist/direct-ID/ISRC gates are unchanged. **After acceptance**, one candidate per provider contributes a vote; metadata consensus does not admit rejected candidates or weaken identity checks. Explicit per-field trust overrides and all alternate provider values are described in [ENRICHMENT.md](ENRICHMENT.md). Deterministic source ordering removes arrival-order-dependent metadata selection.

## Artist evidence for text queries

Pure artist/title queries now require artist-name evidence in addition to the existing score gates: a complete normalized artist token sequence in the supplied artist record(s), or a very close spelling variant (Jaro-Winkler at least 0.94 and TokenSetRatio at least 0.85). A shared name word is insufficient: `Daft Punk` must not accept `Funk Punk` merely because the title is `Get Lucky`. Structured credited collaborators are considered by artist scoring instead of only the first artist. Direct-seed and exact-ISRC rules retain their existing constraints. The 0.40 rejection, 0.30 reporting and 0.45 artist-score constants are unchanged.

These checks prefer missing data over unsupported artist aliases. Transliterated/alternate artist names without sufficient evidence can be rejected; a correct platform ID or known ISRC provides stronger input. They do not prove recording-version identity for every same-title/same-artist edition.

Direct Spotify/Deezer/YouTube structured metadata must return the requested native recording ID. Spotify embed IDs/URIs are checked and empty embed responses fail safely rather than dereferencing missing metadata.

Explicit bracketed or dash-separated recording-version qualifiers (remix, live, acoustic, instrumental, karaoke, sped-up, slowed, radio edit) must agree before a text-derived candidate is accepted. Direct native-ID and exact-ISRC matches retain their precedence. Unlabelled versions remain ambiguous. See [LIVE_SONG_MATRIX.md](LIVE_SONG_MATRIX.md).
