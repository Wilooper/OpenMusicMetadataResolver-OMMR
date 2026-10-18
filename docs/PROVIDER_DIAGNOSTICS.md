# Isolated provider diagnosis — 2026-10-18

`providercheck` calls one actual public provider adapter per process, without server startup credential import, resolver scoring, shared caches or post-merge catalog services. It observes API and fallback requests rather than testing a substitute HTTP implementation. It does not claim search candidates are verified cross-platform matches.

```bash
go run ./cmd/providercheck -provider jiosaavn -routes
go run ./cmd/providercheck -provider deezer -artist 'Daft Punk' -title 'Get Lucky' -routes
go run ./cmd/providercheck -provider musicbrainz -artist 'Daft Punk' -title 'Get Lucky' -routes

go run ./cmd/providercheck -provider jiosaavn -id OQsEfQFVUXk -id-type jiosaavn_share
```

Run providers separately. Each process shares one physical request per second across a provider's API and public website; separate processes do not coordinate limits. Reports go to stdout as JSON; save with shell redirection. Exit 0 means the adapter returned a named song and artists; exit 1 means error/empty, and exit 2 means invalid CLI input. `go run` may also print its child exit status to stderr. Building the command once avoids repeated compilation, including on Termux with Go installed:

```bash
go build -o providercheck ./cmd/providercheck
./providercheck -provider jiosaavn -routes > jiosaavn-check.json
```

Reports include operation, public route/method, HTTP status/protocol, bounded Content-Type/Server headers, elapsed time, body size/hash and response classification. They distinguish a JSON response, API error, HTML page, HTTP 405, rate limiting, timeout and cancellation. The observer replays the exact bytes to the adapter's ordinary decoder. Candidate summaries contain names and provider IDs, not complete raw responses. A normal JSON envelope without candidates is not success.

Requests are bounded to 8 MiB, with per-operation deadlines and a 65-second command deadline. Fixed comparison routes cannot be replaced with arbitrary destinations. Redirects for comparison routes must remain HTTPS on the same host. No cookie jar, configuration file, credential scanner, account login, TLS weakening, proxy selection, third-party gateway or script execution is added. Exception messages, response bodies, request headers and unrecognized query parameters are omitted. Public JioSaavn song-link tokens are retained as catalog identifiers only for `webapi.get&type=song` requests. Query text and returned public artist/track identifiers are intentional report data. This is a local diagnostic tool, not a public HTTP probing endpoint.

## Fresh individual results

All runs below used Karan Aujla / Boyfriend. Each provider was run in a separate process; provider request budgets remain separate.

| Provider | Browser observation | Go protocol comparison | Result |
| --- | --- | --- | --- |
| JioSaavn | Homepage, v4 search and public song-link API each displayed Site Unavailable | Actual v4 adapter + official-page fallback, homepage, `webapi.get` link lookup, Python-style autocomplete, legacy search | All completed responses were HTTP 200, HTML, 195 bytes with the same hash; no candidates |
| Deezer | Homepage and `api.deezer.com/search` displayed Site Unavailable | Actual structured-query adapter + official-page fallback, homepage, plain text search | All completed responses had the same unavailable body; no candidates |
| MusicBrainz | Homepage and recording search displayed Site Unavailable | Actual Lucene-query adapter + official-page fallback, homepage, plain text search in JSON and XML | All completed responses had the same unavailable body; no candidates |

Completed Go responses reported `Server: openresty/1.27.1.1` and SHA-256 `5b131ca14aa96311d3432b0062c7443d3b0e6346ec279bd376a75f0a76bcd5d7`. Browser observations establish visible page state only; no browser body hash or response status was captured. Browser navigation to the Apple API control reported `ERR_BLOCKED_BY_CLIENT`, which is a browser error, not evidence of an Apple response. Earlier Python/Go Apple controls returned valid catalog JSON; see [prior route comparison](test-results/2026-10-18-python-client-routes.json).

Homepage failures are independent of song IDs, query syntax and the Go parser. The matching bodies across unrelated services and browser/server clients support a shared access/intermediary hypothesis, but do not identify its origin. They do not prove a global source outage, incorrect credentials, bot detection, or that no endpoint bug exists. The test did not receive HTTP 405 or a source-specific JSON validation error that would identify an incorrect method. MusicBrainz's [API documentation](https://musicbrainz.org/doc/MusicBrainz_API) confirms GET lookup/search and JSON/XML formats; its [rate guidance](https://musicbrainz.org/doc/MusicBrainz_API/Rate_Limiting) requires identification and at most one call per second.

## A confirmed JioSaavn protocol bug and its fix

The earlier adapter accepted both IDs but sent every value to `song.getDetails&pids=...`. Our output's public song-link token can differ from the native song ID. The current [yt-dlp extractor](https://github.com/yt-dlp/yt-dlp/blob/master/yt_dlp/extractor/jiosaavn.py), reviewed blob `3760e9abeaba54edeff83de9b79e585a8737c958`, uses `webapi.get&type=song&token=...` for public links and reads the `songs` response array. Its Leja Re fixture distinguishes native ID `IcoLuefJ` from share token `OQsEfQFVUXk`.

OMMR now supports both protocols and both keyed/array native detail shapes. Lookup requires one exact matching native ID or provider-owned share permalink; foreign permalinks, contradictory IDs, duplicate matches and non-song types fail. Known embedded HTML `songs` arrays can recover only that uniquely requested record. Accepted search records hydrate using their explicit native ID when present, otherwise their official public share token. Missing native IDs remain absent.

Use explicit types in the existing source extraction API, also supported through the JSON gateway:

```text
GET /v1/extract?provider=jiosaavn&id=IcoLuefJ&id_type=native&debug=true
GET /v1/extract?provider=jiosaavn&id=OQsEfQFVUXk&id_type=share&debug=true
```

```json
{"operation":"extract","provider":"jiosaavn","id":"OQsEfQFVUXk","id_type":"share","debug":true}
```

Without `id_type`, eight-character IDs retain native lookup compatibility, while other lengths select public-token lookup. An eight-character share token requires the explicit `share` type. Returned catalog extras still pass through metadata sanitization. Native IDs and share tokens are not interchangeable, and no new cross-provider identity is inferred.

This fixes an actual routing/decoding defect, verified with protocol/identity regression fixtures. Live tests of the corrected public lookup still received the unavailable page in both browser and Go. The generic search/homepage access failure therefore remains unresolved. Adding more retries, a new language or a duplicate server endpoint has not been demonstrated to repair it. Resolved cache namespace is `rich-v9` so old results do not hide new detail behavior.

Evidence: [JioSaavn](test-results/2026-10-18-providercheck-jiosaavn.json), [Deezer](test-results/2026-10-18-providercheck-deezer.json), [MusicBrainz](test-results/2026-10-18-providercheck-musicbrainz.json), [JioSaavn exact link lookup](test-results/2026-10-18-providercheck-jiosaavn-link.json), [browser observations](test-results/2026-10-18-browser-provider-checks.json).

Validation: Go build, vet and the full test suite passed. Focused race checks passed for diagnostics, JioSaavn, HTML decoding and API handlers; the earlier broader check also covered bootstrap, resolver and provider health. Regressions cover exact lookup method/parameters, keyed and array identities, foreign/duplicate records, legacy artist preservation, GET/JSON ID-type parity, response observation, redaction, limits and cancellation.
