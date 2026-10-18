# Rich public catalogs and optional Python extraction

## Public access is not universal availability

On 2026-10-18, JioSaavn, MusicBrainz and Deezer public JSON endpoints returned HTTP 200 with the same 195-byte HTML “Site Unavailable” page in this environment. This is an access failure, not an empty catalog or evidence that authentication is required. Existing response validation rejects that body and source circuits reduce repeated failed work. No proxy, challenge bypass or substitute service is used. Apple and YouTube remain public-capable; their responses can also vary or time out.

Bugs, Genie and Melon returned real public pages. New native Go adapters support artist/title searches and exact numeric song IDs, then hydrate accepted candidates with song detail fields. HTML parsing is bounded to 4 MiB; links/callbacks are parsed as data, never executed. Structured identity must match the requested song. Unrecognized pages fail explicitly. Unknown catalog detail labels remain in `provider_data` alongside normalized fields. These sources use one request per second each and the existing health circuits.

```http
GET /v1/resolve?artist=방탄소년단&title=Dynamite&sources=bugs,genie,melon&debug=true
GET /v1/resolve?bugs_id=31999479
GET /v1/resolve?genie_id=90705547
GET /v1/resolve?melon_id=32872978
GET /v1/extract?provider=genie&id=90705547&debug=true
```

The new fields also work in bulk and the JSON query gateway. Metadata trust configuration accepts `bugs`, `genie` and `melon`. Correct native IDs appear in links and the identity graph. Korean and English artist names are not assumed equivalent: a Korean-only result can be rejected for an English alias unless other accepted identity evidence establishes the match. Existing artist/score/direct-ID/ISRC gates remain intact. These adapters currently extract songs, not full Korean artist catalogs or charts.

## Python where the library adds value

The Go service owns HTTP handling, native adapters, matching, trust, rate limits, health and cache. The optional `ytmusicapi` worker adds song/player context, album tracks, public song credits and alternate editions, artist descriptions, artwork, audience counts, related artists and release sections when upstream supplies them. It does not convert a biography into verified per-song credits or invent a release date. Context is retained in `provider_data.ytmusic.extra.youtube_catalog` for accepted songs; canonical recording fields/IDs remain guarded.

Each worker is a persistent Python process with one in-flight JSON-lines operation. There are 1–8 workers, a queue of `workers * 4` jobs, per-job deadlines, 4 MiB upstream/protocol response caps, coalescing for identical misses and Go-backed caching. Slow/malformed processes are killed and replaced. Valid upstream errors do not restart healthy workers. Cancellation of a waiting HTTP request does not corrupt the next worker response. Shutdown terminates workers. Queue saturation returns 503 through the catalog endpoint rather than spawning additional Python processes.

A job cache normally lasts six hours; partial song data is cached for at most 30 seconds. Accepted-record enrichment retains successful context when sibling requests fail, reports `partial` in debug provider status and caps resolved caching to 30 seconds. Public entity responses may carry `partial: true`; private cookies, account state, feedback/tracking/streaming tokens and audio payloads are excluded. No browser sessions are harvested. The Python implementation currently uses public mode even when the native Go YouTube adapter has validated credentials.

This design bounds resources. It does not establish lower CPU use or higher throughput than the Go-only baseline; those require deployment benchmarks. Python is optional and unavailable workers leave ordinary Go resolution usable.

## Install and enable

From the repository directory:

```sh
python3 -m venv .venv
.venv/bin/pip install -r workers/ytmusic/requirements.txt
```

In private `ommr.yaml`:

```yaml
extractor:
  enabled: true
  python: /absolute/path/to/ommr/.venv/bin/python
  script: /absolute/path/to/ommr/workers/ytmusic/worker.py
  workers: 2
  timeout: 5s
  ttl: 6h
  profiles: true
```

Paths should be absolute when starting from another working directory. Environment alternatives include `OMMR_EXTRACTOR_ENABLED`, `OMMR_EXTRACTOR_PYTHON`, `OMMR_EXTRACTOR_SCRIPT`, `OMMR_EXTRACTOR_WORKERS`, `OMMR_EXTRACTOR_TIMEOUT`, `OMMR_EXTRACTOR_TTL` and `OMMR_EXTRACTOR_PROFILES`. Restart after changes. Worker startup validates the local protocol/dependency capability, not upstream authentication or all remote operations. `/v1/providers` exposes `ytmusic_python` health. For Docker, build `--target with-python` and enable the extractor in mounted configuration; default builds remain Go-only.

## Go catalog endpoint

```http
GET /v1/catalog?provider=ytmusic&type=song&id=tIjUfo30r4M
GET /v1/catalog?provider=ytmusic&type=credits&id=MPTCjO_BW28XPKk
GET /v1/catalog?provider=ytmusic&type=album&id=MPREb_qTDpBqltt6c&limit=20
GET /v1/catalog?provider=ytmusic&type=artist&id=UCSmK5WX5U4gdtebWjoL81og&limit=20
GET /v1/catalog?provider=ytmusic&type=artist_albums&id=UCSmK5WX5U4gdtebWjoL81og&limit=50
GET /v1/catalog?provider=ytmusic&type=artist_singles&id=UCSmK5WX5U4gdtebWjoL81og&limit=50
```

`playlist` is also supported for valid `OLAK5uy_`, `PL` or `RD` IDs. Limits are 1–100. Artist profiles return top sections, while `artist_albums`/`artist_singles` use the library's bounded continuation traversal. Results indicate a possible further catalog when the limit is reached; unlimited scraping is not provided. Album tracks/playlist tracks are source context, not additional accepted recording identities. IDs are validated and no caller-selected upstream URL is accepted.

```http
POST /v1/query
Content-Type: application/json

{"operation":"catalog","provider":"ytmusic","type":"artist","id":"UCSmK5WX5U4gdtebWjoL81og","limit":10}
```

Set `api.endpoints.catalog: false` to disable both access paths. Set `providers.enabled.ytmusic: false` to prohibit YouTube extraction. Missing/offline worker: 503; invalid input: 400; unavailable upstream metadata: 502; API deadline: 504. A separate missing-data result is never fabricated as success. Profiles may resolve a YouTube Topic browse ID to an official channel ID; these remain separate source references and do not overwrite canonical artist ID ownership.

## Reference research

[SpotiFLAC extension registry](https://github.com/spotiflacapp/SpotiFLAC-Extension) lists Bugs/Genie/Melon packages. Their public-page formats informed independent Go adapters; extension JavaScript is not executed or bundled. [ytmusicapi documentation](https://ytmusicapi.readthedocs.io/en/stable/reference/api/ytmusicapi.mixins.html) describes album/artist/watch browsing and continuation support. The optional dependency is pinned to 1.12.3. See [LIVE_RICH_CATALOGS.md](LIVE_RICH_CATALOGS.md) for observed successes and limitations.

Public song credits use a source-provided `MPTC` browse ID plus 11-character recording identifier. Credit section names remain source-specific; Written by is not automatically composer or lyricist. See [Known Unknowns album verification](YOUTUBE_KNOWN_UNKNOWNS_VERIFICATION.md) for all 10 live tracks, video/audio differences and the stricter title acceptance regression.
