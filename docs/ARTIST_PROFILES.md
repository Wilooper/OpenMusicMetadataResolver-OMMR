# Public artist profiles

`GET /v1/artists` returns independent artist profiles from Deezer, JioSaavn, Apple Music's public iTunes API, MusicBrainz and optional YouTube Music. It uses public metadata without credentials. The native Go sources work without the Python extractor; YouTube uses the existing optional, pinned ytmusicapi worker. Missing sources do not prevent other profiles from returning.

```text
GET /v1/artists?artist=Gur%20Sidhu&limit=10
GET /v1/artists?artist=Daft%20Punk&sources=deezer,jiosaavn,musicbrainz,ytmusic&debug=true
GET /v1/artists?provider=deezer&id=5810099&limit=5
GET /v1/artists?provider=ytmusic&id=UC5RFfy546sgeEdO03RBqKyg&limit=5
GET /v1/artists?provider=applemusic&id=862626825&debug=true
```

```json
{"operation":"artists","artist":"Gur Sidhu","sources":["deezer","jiosaavn","musicbrainz"],"limit":5,"debug":true}
```

Use an artist name or an exact `provider`/native artist `id` pair. A name may additionally constrain an exact lookup; disagreement is rejected. IDs are numeric for Deezer/JioSaavn/Apple, a MusicBrainz artist UUID, or a YouTube artist/channel `UC...` ID. Song IDs and JioSaavn share tokens are not artist IDs. `limit` is 1–50 and defaults to 10, bounding each returned track list. `sources` cannot accompany `provider`/`id`. Unsupported/repeated parameters, foreign URLs and invalid IDs return HTTP 400 before network access.

## Identity and ambiguity

Name searches consider at most 20 source candidates and require one unique exact normalized artist name among those candidates. Accent/whitespace/case normalization follows the existing query normalizer; broad fuzzy matches and combined artist entries are not promoted. Ambiguous sources return `candidates` with provider, native ID, name, page URL and available disambiguation text. The caller can retry an exact ID. The live Gur Sidhu search returned three matching Apple IDs and two matching YouTube IDs; neither source was chosen arbitrarily. Daft Punk and Arijit Singh also demonstrated source-specific duplicates.

Profiles remain separate by provider. Same-name profiles are not merged into a globally verified artist or a single cross-service audience total. `identity_method` describes source selection (`unique_exact_name` within the bounded search or `exact_id`), not proof of global cross-provider identity. An exact response must report the requested artist ID. YouTube Topic IDs can resolve to an official channel: `id` remains the requested browse ID, while the reported `channelId` remains in provider data. Recommendations and related artists do not supply the requested artist's audience counts.

## Fields

Every profile has `provider`, native `id`, `name`, artist page `url`, `identity_method`, and an `observed_at` timestamp when actual fetch time is known. Available fields include:

- `biography`: source prose, retaining attribution/license text. YouTube biographies can be attributed to Wikipedia. Empty JioSaavn `bio: "[]"` is not emitted as a biography.
- `images`: artist-page images only, with dimensions when supplied. Album covers are not substituted for missing artist portraits.
- `metrics`: typed source audience/play counters with exact `value` only when the source reports a full integer, `display_value`, original/semantic `label`, `period`, `scope` and `source_field`.
- `counts.total_albums`, `counts.total_tracks`, `counts.total_recordings`: only explicit source totals. Unknown counts are omitted, not set to zero or inferred from the returned list length.
- `top_tracks`: source-owned top-song listings from Deezer, JioSaavn and YouTube. Collaborations can appear under another lead performer; these are artist-page context, not cross-resolved canonical identities.
- `recent_tracks`: Apple's explicitly requested recent songs. They are not relabeled as popularity-ranked top tracks.
- `catalog_tracks`: a bounded MusicBrainz recording sample. Its recording count is a catalog total, not stream plays or an exact count of streaming-service tracks.
- `provider_data`: safe selected source fields such as languages, verified badge, country/area/life span, aliases, genres, social/reference URLs, annotation and biography attribution runs.

Track context includes source-native IDs/page URLs, reported artists/album, artwork, duration and per-track play/view metrics where available. Play counts for collaborations and recommendations are never summed into an artist total.

### Metric precision and time windows

| Source field | Returned concept | Period/scope |
| --- | --- | --- |
| Deezer `nb_fan` | fans | snapshot / artist |
| JioSaavn `follower_count` | followers | snapshot / artist |
| JioSaavn subtitle “Listeners” + `fan_count` | listeners | unspecified / artist |
| YouTube `subscribers` | subscribers | snapshot / artist channel |
| YouTube parser `monthlyListeners` | **monthly audience** (observed UI label) | monthly / artist |
| YouTube `views` | views | unspecified / artist channel |
| Song `play_count` / `views` | plays / views | unspecified / that track |

The pinned YouTube parser strips the text “monthly audience” into a field named `monthlyListeners`; the endpoint preserves the actual audience concept rather than asserting monthly listeners or plays. `162K` and `105M` stay display text with no exact numeric `value`; `172,808,309 views` can retain exact integer 172808309 and the original display string. Known zero is represented as zero; absence stays absent. The monthly label does not establish calendar-month boundaries or an exact rolling window. These providers do not currently supply a verified annual play history through these endpoints. Monthly/annual plays, listening histories and historical counter deltas are not fabricated or extrapolated.

## Execution, caching and settings

Each native request uses HTTPS on a fixed provider host, refuses redirects, caps JSON to 4 MiB, uses a meaningful application/contact User-Agent, and passes through the shared provider limiter. MusicBrainz artist API requests use the same service budget as existing resolver entry requests. Existing adapter-internal transport budgets still apply; limits do not represent guaranteed upstream capacity. Failed/empty HTML or mismatched IDs cannot become profiles merely because HTTP status was 200.

Artist profiles have a separate `artists:v1` cache namespace. Successful source results cache for five minutes; ambiguous/no-result/partial results for at most 30 seconds; errors are not cached. Cache keys include source, native ID/name, limit, storefront and health signature. `bypass_cache=true` bypasses these results and worker lookup caches. YouTube audience profile cache misses always use a fresh worker request so a general six-hour worker cache cannot make audience data appear freshly observed. The worker records the actual fetch time; an unknown fetch time is omitted. Cached output preserves the original observation time.

All source requests share the HTTP operation deadline and fail independently. If the primary profile is valid but its top-track/catalog request fails, the profile is retained with `partial` in debug status. Existing health circuits and source-enable flags apply. Local worker queue saturation or request-budget exhaustion releases a probe lease without counting as an upstream failure. Debug mode adds per-source `status`, sanitized `error`, `cached` and `partial`; production retains music/profile data and ambiguity candidates. Use debug to distinguish unavailable, ambiguous, missing and failed sources. Source errors do not expose upstream bodies, headers, credentials or internal paths.

`api.endpoints.artists: false` disables GET and POST gateway access. `api.default_operation: artists` is supported. Artist searches default to the five supported artist sources, independently of the track-specific `api.default_sources`; explicit `sources` works when API overrides are allowed. Provider disable flags apply equally to all paths. YouTube requires `extractor.enabled: true`; its absence does not disable the native Go profiles. Existing source registration and track matching thresholds are unchanged.

## Live Termux verification

See [sanitized reports](test-results/artist-profiles-2026-10-18/README.md) for Gur Sidhu, Daft Punk and Arijit Singh, name ambiguity, exact IDs, GET/POST parity, cache reuse, bypass, invalid inputs and a credential-free Go-only run. MusicBrainz intermittently returned HTTP 503; missing/partial outcomes remain visible. Values are observations from that run, not hardcoded runtime metadata.

The Apple recent-song lookup follows the official [lookup examples](https://developer.apple.com/library/archive/documentation/AudioVideo/Conceptual/iTuneSearchAPI/LookupExamples.html). MusicBrainz's [API](https://musicbrainz.org/doc/MusicBrainz_API) and [rate policy](https://musicbrainz.org/doc/MusicBrainz_API/Rate_Limiting) describe its public catalog semantics and request requirements. JioSaavn and YouTube web metadata interfaces are unofficial and can change.

Final verification passed Go build, vet, serial and default full tests and all eight worker tests. Focused race attempts returned `-race is not supported on android/arm64`. Deterministic regressions cover ambiguous names, wrong IDs, counter precision/periods, bounded lists versus totals, related-artist isolation, timestamp retention, cache bypass, cancellation, capacity/circuit behavior, URL policy and GET/POST endpoint policy.
