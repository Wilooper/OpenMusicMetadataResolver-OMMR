# HTML catalog recovery and “Site Unavailable” — 2026-10-18

## Findings

Fresh credential-free probes checked both API and official public-search pages for JioSaavn, Deezer and MusicBrainz. All six returned HTTP 200 with an identical 195-byte HTML “Site Unavailable” page. It contained no embedded JSON, structured music record, recording ID, artist, album or duration. The matching bodies suggest a shared access/intermediary failure, but do not establish its origin. They do not demonstrate a global source outage or a requirement for credentials. A second API probe with explicit `Accept: application/json` returned the same 195-byte HTML for all three; content negotiation did not resolve the access failure.

The updated server's live extraction retry still reported source errors: JioSaavn/MusicBrainz returned the empty unavailable page; Deezer's public-page request failed. No fake track was emitted. A separate Boyfriend control request returned Apple, YouTube, Bugs, Genie and Melon IDs normally.

## Implemented recovery

The three affected adapters now use a shared bounded Go HTML decoder while preserving normal JSON responses:

| Response format | Recovery |
| --- | --- |
| Normal API JSON | Keep original payload; explicit API error envelopes remain errors |
| Sole HTML `<pre>` containing escaped JSON | Decode HTML entities and parse the provider's catalog envelope |
| `application/json` or `__NEXT_DATA__` app state | Accept recognized provider envelopes inside known state containers |
| Known initial-state assignments | Decode the JSON value only; no JavaScript is executed |
| JSON-LD `MusicRecording` / `ItemList` / `@graph` | Convert provider-owned song records into each adapter's existing JSON shape |
| MusicBrainz search table | Convert recording-name and adjacent artist-credit cells into recording JSON |
| Error page without catalog records | Return an explicit unavailable/no-data error |
| Open Graph title alone, arbitrary JSON or recommendations | Do not guess a recording |

Recovered records pass through the same provider normalizer, artist/version/score gates and merger as API records. Direct lookups still require the requested ID. Schema URLs must identify the correct provider; contradictory entity IDs and ambiguous app envelopes fail. Unknown structured entity fields survive in `provider_data.<provider>.catalog.structured_metadata`, subject to existing secret/tracking sanitization. Raw HTML is not returned.

Missing artist/album IDs remain absent, including Deezer's previously fabricated zero IDs when HTML provides names without native entity IDs. Structured-image dimensions are not guessed. JioSaavn share tokens remain distinct from native lookup IDs: a JSON-LD permalink does not establish a native song ID. ISRCs are not inferred from titles, links or arbitrary identifiers.

## Official public-page fallback

When an API returns unrecognized/unavailable HTML, a bounded official-page attempt can recover a usable catalog:

- Deezer: public song search, or exact numeric track page.
- JioSaavn: public song search. Short native IDs do not reveal a public title slug, so no direct page URL is invented.
- MusicBrainz: public recording search or exact recording page. A visible search row cannot substitute for exact recording detail; HTML does not establish an ISRC lookup.

Fallback uses a five-second HTTP timeout, the remaining request context, an 8 MiB body cap, same-host HTTPS redirects only and an additional conservative request budget. Production fallback requests consume the shared provider limiter; MusicBrainz API/public requests also share a physical HTTP limiter. A failed public route backs off for one minute before another attempt. Existing provider circuits still apply. No third-party mirror, proxy, credential collection, script execution or challenge bypass is introduced. Authentication probes keep their strict JSON/protected-endpoint behavior.

Resolved cache namespace is `rich-v7`; old sparse output does not mask a recovered source. Public/debug response structure remains unchanged. Use `debug=true` to distinguish source failures from genuine empty results; HTTP 200 for an extraction envelope alone is not source success.

## Validation and limits

Build, vet, full Go tests and race checks passed. Fixtures cover normal JSON, escaped preformatted JSON, native app envelopes, JSON-LD, scoped MusicBrainz rows, foreign/wrong IDs, missing native IDs, extra fields, API error redaction, oversized bodies, ambiguous payloads, public-fallback host restrictions and cooldown/cancellation. Adapter tests verify HTML becomes ordinary Deezer/JioSaavn/MusicBrainz candidates and that a valid public page can recover a failed API.

These passing fixtures establish implemented parsing behavior, not a successful live recovery of the three currently unavailable services. Website formats can change; unsupported layouts fail safely. Pages requiring client-side execution or a challenge are not rendered. A page with no catalog content cannot be converted into real music data.

Observations: [public probes](test-results/2026-10-18-html-probes.json), [updated-server live results](test-results/2026-10-18-html-live.json), [explicit JSON-header probes](test-results/2026-10-18-json-negotiation.json).
