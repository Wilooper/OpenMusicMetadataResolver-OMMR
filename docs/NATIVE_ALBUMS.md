# Native albums and complete track lists

`GET /v1/albums` returns one exact source-owned album or MusicBrainz release. It uses native Go HTTP clients and public metadata; no Python process, cookies or keys are required. It does not merge album editions across services or change the canonical track/album schema.

```sh
curl 'http://localhost:8080/v1/albums?provider=applemusic&id=1829591921'
curl 'http://localhost:8080/v1/albums?provider=deezer&id=6575789&debug=true'
curl -X POST http://localhost:8080/v1/query \
  -H 'Content-Type: application/json' \
  -d '{"operation":"albums","provider":"applemusic","id":"1829591921"}'
```

Use the configured port if different from 8080. The gateway also accepts the fields inside `query`.

| Parameter | Behavior |
| --- | --- |
| `provider` | Required: `applemusic`, `deezer`, `jiosaavn` or `musicbrainz` |
| `id` | Required native **album** ID; MusicBrainz requires an exact **release UUID**, not a recording or release-group UUID |
| `limit` | Maximum returned track entries, 1–1000; defaults to 1000 |
| `debug` | Boolean; adds source execution status/cache information; follows server debug policy |
| `bypass_cache` | Boolean; fetch fresh data; follows server override policy |

IDs must be supplied as IDs, not arbitrary URLs. Leading/trailing whitespace is normalized; MusicBrainz UUID case is normalized by the HTTP handler. Unknown or repeated query fields return 400. Track resolution's `album` title filter remains a separate feature. Album native IDs are available in source catalog records returned by track resolution/extraction; IDs from another source cannot be substituted.

## Data retained

The response has an `album` object containing native provider/album identity, title, artists, public catalog URL, release date at the source's precision, label, UPC/barcode, copyright, language, artwork and source-owned extra fields when available. Missing metadata is omitted, not guessed. `observed_at` is the actual fetch time and stays unchanged on cache hits.

Each track includes its native ID, title, position, source-reported disc number and printed track number, duration, artists, ISRC when reported, and sanitized `provider_data`. MusicBrainz track-entry IDs and recording IDs remain separate: the same recording may legitimately occur on multiple discs. MusicBrainz printed numbers such as `A1` remain strings. The recording title does not overwrite a release-specific track title. Apple tracks are sorted by disc and track number. Deezer falls back to the source list position when its response omits track-position fields; it does not invent a disc number. JioSaavn preserves the album's list order.

Album-level and track-level `provider_data` preserve additional catalog fields, including structured credits, genres, explicitness, price/currency, availability, labels/catalog numbers, release-group context and all reported ISRCs where present. Credentials, encrypted media URLs, playback and tracking fields are filtered recursively. Album track containers are removed from album `provider_data` so they cannot bypass the requested track limit. These records are catalog data, not execution diagnostics, and remain available in production.

`discs` contains source-declared disc metadata for Apple and MusicBrainz when available. MusicBrainz disc extra metadata retains pregap/data-track information when reported; `tracks` and its total refer to the normal medium track list. Stream/playback requests and per-track detail fan-out are not performed. A provider's album-list response may expose fewer fields than its individual track-detail endpoint.

## Completion is explicit

```json
{"album":{"provider":"applemusic","id":"1829591921","title":"P-POP CULTURE","track_list":{"returned":11,"total":11,"complete":true},"tracks":[]}}
```

The example abbreviates `tracks`; the live response includes all 11 entries across two discs. Always use `track_list.complete` before presenting a list as complete.

| Field/reason | Meaning |
| --- | --- |
| `returned` | Number of actual track entries in this response |
| `total` | Provider-reported album total; omitted when unknown; never derived from a sample |
| `complete: true` | Valid returned entries match the declared album total; MusicBrainz medium counts must also match |
| `reason: limit` | The caller/server track limit stopped the list |
| `reason: unknown_total` | The source did not report a usable total; completeness is unproven |
| `reason: incomplete_response` | Missing, invalid, duplicate or inconsistent entries/counts prevent completeness |
| `reason: upstream_error` | Album metadata survived, but a subsequent page failed |

Deezer pages are fetched sequentially on the fixed API host, up to 100 entries per page and 20 pages, stopping at the requested limit. Returned `next` URLs are never followed. Repeated pages cannot inflate completeness. iTunes uses its public album/song lookup with `limit=200`; there is no invented offset fallback. Albums exceeding the public response's coverage remain explicitly incomplete even if the requested limit is 1000. JioSaavn decodes both flat `songs` and v4 `list`/`more_info` forms. MusicBrainz uses the exact release lookup with recordings, artist credits, labels, release groups, genres, ISRCs and URL relationships.

A partial album returns HTTP 200 with explicit completion information. Invalid input returns 400, a missing album 404, upstream failure before album metadata 502, disabled/unavailable service 503, and a deadline before album metadata 504. Upstream URLs/bodies/errors are not echoed to clients. Debug adds only `source.status`, `source.cached` and a sanitized page-error code when applicable.

## Runtime policy and caching

Albums share the existing per-provider physical request limiter, source enable flags and provider health circuits. Invalid album identity cannot become a successful response. Local request-budget failures and caller cancellation do not trip source health; an upstream page failure does. No authentication process is started.

The independent `albums:v2` cache includes provider, exact ID, track limit, Apple storefront and runtime health signature. Complete entries cache for five minutes; partial entries for 30 seconds; failures without album metadata are not cached. Debug and production share entries. A different track limit has a separate entry and can cause a new upstream lookup. Every upstream body is limited to 4 MiB and redirects are refused.

```yaml
api:
  default_operation: albums
  allow_debug: false
  endpoints:
    albums: true
```

Restart after configuration changes. The same endpoint flag applies to REST and JSON requests. Track source defaults do not select or override exact album providers. `/v1/capabilities` advertises eligible album sources and `album_max_tracks`. Eligibility is local readiness, not a guarantee of live access.

YouTube's existing optional Python album path remains `/v1/catalog?provider=ytmusic&type=album&id=<MPRE...>&limit=100`; see [YouTube album verification](YOUTUBE_KNOWN_UNKNOWNS_VERIFICATION.md).

## Verification on 2026-10-19

The running API fetched Apple's P-POP CULTURE album: **11/11 tracks, two discs (5 + 6)**. Boyfriend is disc 1, track 4. REST and JSON gateway responses matched; production omitted execution diagnostics; a two-track limit retained the total of 11 and reported incomplete. Warm gateway cache response took about 2 ms in this run; this is an observation, not a throughput benchmark. Public response: [smoke test results](test-results/2026-10-19-native-albums-smoke.json).

Deezer, JioSaavn and MusicBrainz album requests each returned HTTP 200 with the same 195-byte “Site Unavailable” HTML from this remote environment. No successful live album extraction is claimed for those sources here. Constructed protocol tests cover pagination, exact identity, flat/v4 decoding, multi-disc repeated recordings, partial failures, sanitization and request policy. Recheck those three sources from the user's Termux/VPN network before claiming live recovery.

Protocol references: [Apple's album/song lookup examples](https://developer.apple.com/library/archive/documentation/AudioVideo/Conceptual/iTuneSearchAPI/LookupExamples.html), [MusicBrainz API](https://musicbrainz.org/doc/MusicBrainz_API), and [MusicBrainz release model](https://musicbrainz.org/doc/Release).

## Recording and credit graph

Album responses also include `music_graph` for release relationships, contributors and returned track entries. MusicBrainz supplies release-group, recording, composition and scoped credit links where reported. Only returned tracks enter the graph. Album cache is now `albums:v2`; see [graph documentation](MUSIC_GRAPH.md).
