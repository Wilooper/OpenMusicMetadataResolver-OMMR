# OMMR Matching Engine Architecture

## String Similarity Algorithms (`internal/matcher/string_sim.go`)

The matching engine uses a composite string similarity score combining two string metrics:

1. **Jaro-Winkler Distance**: Evaluates character transposition and prefix agreement.
2. **Token Set Ratio**: Tokenizes title/artist strings into sorted sets, ignoring word order variations (e.g. `AP Dhillon, Gurinder Gill` vs `Gurinder Gill & AP Dhillon`).

```go
titleScore = 0.6 * JaroWinkler(target, candidate) + 0.4 * TokenSetRatio(target, candidate)
```

---

## Field Weight Distribution

| Field | Weight | Description |
| --- | --- | --- |
| `Title` | **0.50** | Cleaned track title similarity |
| `Artist` | **0.40** | Cleaned primary artist similarity |
| `Album` | **0.10** | Cleaned album title similarity |

*Note: Non-ISRC exact matches are capped at `0.96` to reserve `1.00` for verified ISRC matches.*

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
| `YouTube Music` | **0.80** | User-uploaded content and audio/video title noise |

```go
IdentityConfidence = CandidateMatchScore * ProviderTrustWeight[provider]
```

---

## Nullable `MatchBreakdown` (`unknown != mismatch`)

Un-evaluated fields in `MatchBreakdown` serialize as `null` / omitted pointers (`*float64`). A `null` value indicates that the field was not available in the candidate, distinguishing missing data from active field mismatches.
