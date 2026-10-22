# Full OMMR verification — 2026-10-19

Fresh checks cover the current recording/edition/credit graphs, native albums, share links, metadata resolution, API policies, provider lifecycle and optional Python worker. Two verified defects were fixed during this run. The final Go suite passes; live source limitations remain listed below.

## Results

| Check | Result |
| --- | --- |
| `go build ./...` | Passed |
| `go vet ./...` | Passed |
| `go test -race -count=1 -coverprofile=... -json ./...` | Passed: **246 top-level tests; 331 cases including subtests; zero failures/skips** |
| Go statement coverage | **75.0%**; coverage is not a guarantee of correctness or live source reachability |
| Python worker unit tests | **8/8 passed**, pinned ytmusicapi 1.12.3, Python 3.12 |
| Running API contract smoke | **58/58 checks passed** |
| Credential-free live song matrix | **12/12 returned accepted metadata; production/debug IDs and provider data matched** |
| Graph audit of all 12 live public responses | **12/12 passed**: roots present, no dangling edges, bounded graphs, no root execution diagnostics |
| Native Apple album | P-POP CULTURE: **11/11 tracks, two discs (5+6)**; limited responses keep totals and limit graph entries |
| Cached concurrency smoke | 32 requests, 16 clients: all equal responses; median 8.39 ms, p95 14.82 ms, maximum 20.83 ms, 0.039 s wall time |
| Disk cache persistence | Album cache and observation timestamp survived a real server restart |
| Android ARM64 | `GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build` passed; binary was **not executed on Android** |

The cache timing is a small localhost observation with warm data, not sustained-load capacity, remote latency, cold-cache performance or a production throughput guarantee. No claim of million-user scalability is made. The ordinary song matrix's median cold response was 2.832 seconds and median warm response 0.003 seconds; Get Lucky needed a roughly 2.87-second warm-path response, so not every repeat was an immediate cache hit.

## Live provider outcomes

| Source/path | Observation |
| --- | --- |
| Apple public/iTunes | Accepted on 12/12 songs; exact extraction, track links, artist profile, native album and graph delivery passed |
| Bugs public | Accepted on 8/12 songs; direct Dynamite extraction also succeeded |
| Genie public | Accepted on 9/12 songs; direct Dynamite succeeded initially, timed out on a later attempt; Korean-name search still returned it |
| Melon public | Accepted on 10/12 songs; direct Dynamite succeeded initially, timed out later; Korean-name search still returned it |
| YouTube Go adapter | Accepted on 11/12 songs |
| YouTube optional Python | After correcting the test interpreter path, the worker loaded; artist profile succeeded; Known Unknowns limit=2 returned 2 tracks, source total 10, and truncation; other calls intermittently returned 502/timeouts |
| YouTube public credits | HTTP worker attempt failed; a separate direct-worker retry succeeded with performed_by, written_by, produced_by and metadata-provider sections. This does not prove stable HTTP endpoint availability |
| Spotify public | Anonymous text search failed with HTTP 400; exact public ID `4uLU6hMCjMI75M1A2tKUQC` returned Never Gonna Give You Up in a fresh server |
| SoundCloud public | Selected by the live resolver but no accepted recording returned in this corpus; no successful live extraction is claimed |
| JioSaavn | Homepage, API and page fallback returned the same HTTP-200 195-byte Site Unavailable HTML; no live recovery |
| Deezer | Homepage, API and page fallback returned the same HTTP-200 195-byte Site Unavailable HTML; no live recovery |
| MusicBrainz | Homepage, API and page fallback returned the same HTTP-200 195-byte Site Unavailable HTML; graph relationships remain fixture-verified here |
| Qobuz, TIDAL, Amazon Music, Pandora | Correctly unloaded with credentials absent; no protected live calls tested |
| Last.fm, Discogs, AcoustID | Correctly disabled without keys; AcoustID endpoint returned 503 as expected |
| Cover Art Archive / MB release enrichment | Lifecycle startup paths exercised; no successful live enrichment is claimed for this corpus without accepted MB identities |

For JioSaavn, Deezer and MusicBrainz, the unavailable bodies share SHA-256 `5b131ca14aa96311d3432b0062c7443d3b0e6346ec279bd376a75f0a76bcd5d7`. This establishes identical observed responses, not whether the origin is the service, proxy or execution network. No VPN/region/root-cause fix is claimed.

Live metadata was obtained without account login or operator credentials. Accepted source IDs passed current resolver gates; the run did not independently fingerprint audio to prove every returned platform ID corresponds to identical audio.

## Song matrix

| Song | Artist | Accepted sources |
| --- | --- | --- |
| Boyfriend | Karan Aujla | Apple, Bugs, Genie, Melon, YouTube |
| Tu | Talwiinder | Apple, Melon, YouTube |
| Get Lucky | Daft Punk | Apple, Genie |
| Blinding Lights | The Weeknd | Apple, Bugs, Genie, Melon, YouTube |
| Hello | Adele | Apple, Bugs, Genie, Melon, YouTube |
| Dynamite | BTS | Apple, YouTube |
| Bohemian Rhapsody | Queen | Apple, Bugs, Genie, Melon, YouTube |
| Tum Hi Ho | Arijit Singh | Apple, Genie, Melon, YouTube |
| Despacito | Luis Fonsi | Apple, Bugs, Melon, YouTube |
| Idol | YOASOBI | Apple, Bugs, Genie, Melon, YouTube |
| Anti-Hero | Taylor Swift | Apple, Bugs, Genie, Melon, YouTube |
| bad guy | Billie Eilish | Apple, Bugs, Genie, Melon, YouTube |

The separate Korean artist-name query `방탄소년단` / Dynamite returned all three Korean catalog IDs. Therefore missing source acceptance in an English-name query must not be described as global provider unavailability. Query spelling and source search results affect coverage.

## API and regression scope

The running-server contract checks exercise REST/JSON capability discovery; exact share-link normalization; cached production/debug graph parity; foreign and mixed input rejection; repeated/invalid debug fields; wrong methods; scalar/unknown gateway fields; bulk limits and independent failures; exact extraction; links; search and huge-page safety; exact artist lookup; complete/limited albums; provider/worker/key absence; endpoint flags; blocked debug; locked operation defaults; metrics disabling; disk restart; and concurrent warm requests. The test uses isolated configuration/cache directories and no inherited OMMR environment or credentials.

The full Go suite additionally checks candidate acceptance, title/artist/version gates, wrong same-album IDs, ISRC handling, detail identity, metadata comparison/trust policy, HTML decoding, cookies/private files, credential fallback/probing, health cooldown/cancellation, worker queue/process limits, graph scope/direction and album pagination. Fixture success is kept separate from live authentication or service recovery.

## Fixes made

1. `tools/live_rich_catalogs.py` previously resolved a virtualenv Python symlink to the base interpreter. That interpreter did not have ytmusicapi, so startup incorrectly appeared as worker unavailable. The tool now preserves the absolute virtualenv executable path. Initial 503 results and corrected runs are both retained.
2. Structured JioSaavn graph contributor names could retain HTML entities, e.g. `Composer &amp; Player`, while the flat credit had `Composer & Player`. This could create a duplicate name-only credit. Names are decoded before graph creation and fallback deduplication. A regression verifies the contributor's native ID is retained and the duplicate is absent. An additional test checks exact recording membership before release-enrichment credits enter a graph. Resolved/album cache namespaces advance to `rich-v14` / `albums:v3` so old escaped-name graphs cannot survive the fix.

The initial combined coverage attempt hit a runner issue: the coverage subprocess could not find `go` on PATH. Adding the toolchain directory to PATH fixed it. Final evidence is from a fresh successful full run after the product fixes.

## Remaining verification and next priority

Real authenticated Spotify/Apple/YouTube/partner modes were not exercised because no credentials were supplied and login remains stopped. Redis server/container tools were unavailable, so no live Redis or Docker integration was run. Android compilation passed, but actual Termux execution, Android race testing, memory, battery and sustained cold-load behavior remain unmeasured.

Current health circuits apply to whole providers, not separate search/detail operations. Spotify's observed search failure and successful fresh exact lookup illustrate why capability-specific health is a useful next improvement; a search failure can temporarily suppress another working operation until recovery. The isolated direct success does not prove lookup works through an already-suspended provider.

Suggested next work: run this same suite from Termux/VPN, then address operation-specific health and YouTube timeout reliability before adding more sources. Keep authenticated results separate from public fallback results.

## Reproduce

```sh
# From the repository, with the installed Go toolchain on PATH:
go build ./...
go vet ./...
go test -race -count=1 ./...  # On supported desktop hosts.
go test -count=1 ./...        # Native Termux: race mode may be unsupported.
python -m unittest discover -s workers/ytmusic -p 'test_*.py' -v

go build -o bin/server ./cmd/server
python tools/verify_api.py --server bin/server --output /tmp/ommr-api-checks
python tools/live_song_matrix.py --server bin/server --output /tmp/ommr-song-checks
python tools/live_rich_catalogs.py --server bin/server \
  --python "$PWD/.venv/bin/python" --output /tmp/ommr-rich-checks
```

Use an interpreter with `workers/ytmusic/requirements.txt` installed. In Termux replace `/tmp/...` with writable paths under `$TMPDIR`. The smoke corpus contains known catalog IDs/totals which can change upstream; a future mismatch requires investigation rather than automatically weakening an assertion. Standard-library verification scripts require no additional Python dependencies; the optional YouTube worker does.

Evidence: [checks](test-results/full-verification-2026-10-19/checks.json), [API cases](test-results/full-verification-2026-10-19/api-contracts.json), [song results](test-results/full-verification-2026-10-19/songs.json), [graph audit](test-results/full-verification-2026-10-19/graph-audit.json), [full Boyfriend debug sample](test-results/full-verification-2026-10-19/boyfriend-debug.json), [provider observations and exclusions](test-results/full-verification-2026-10-19/summary.json).
