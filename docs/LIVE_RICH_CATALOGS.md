# Live rich catalog validation — 2026-10-18

All runs were credential-free with isolated configuration/cache and US Apple storefront. Browser/account setup stayed stopped. Observations describe this environment, not global provider uptime.

## Twelve-song Go regression run

`tools/live_song_matrix.py` used the normal 10-second API deadline with the Python extractor disabled. All 12 queries returned HTTP 200/nonempty tracks; subsequent production responses preserved the same recording IDs and provider data while omitting debug execution/comparison fields. Accepted IDs were present from Apple and YouTube on all 12, Bugs on 9, Genie on 10 and Melon on 11. Artist/score/ISRC gates were unchanged. These are accepted matches, not independently verified ISRC identities.

| Query | Accepted Korean sources | Duration ms | Identity | Cold seconds |
| --- | --- | --- | --- | --- |
| Boyfriend — Karan Aujla | bugs, genie, melon | 161000 | strong | 9.506 |
| Tu — Talwiinder | genie, melon | 132086 | strong | 3.329 |
| Get Lucky — Daft Punk | bugs, genie, melon | 370000 | probable | 4.294 |
| Blinding Lights — The Weeknd | bugs, genie, melon | 202000 | strong | 5.688 |
| Hello — Adele | bugs, genie, melon | 296000 | strong | 4.495 |
| Dynamite — BTS | none | 199054 | strong | 2.053 |
| Bohemian Rhapsody — Queen | bugs, genie, melon | 355000 | strong | 4.793 |
| Tum Hi Ho — Arijit Singh | genie, melon | 262000 | strong | 4.173 |
| Despacito — Luis Fonsi | bugs, melon | 229387 | strong | 5.009 |
| Idol — YOASOBI | bugs, genie, melon | 213000 | strong | 3.191 |
| Anti-Hero — Taylor Swift | bugs, genie, melon | 201000 | strong | 4.465 |
| bad guy — Billie Eilish | bugs, genie, melon | 194000 | strong | 5.073 |

Boyfriend now includes Bugs `126998566`, Genie `111431603` and Melon `39664161`, alongside the previous Apple/YouTube matches. Most added catalogs use English artist names for international songs. BTS is an exception: Korean `방탄소년단` results were rejected for the English BTS query, and were accepted for an explicit Korean artist query. This preserves the alias safety rule rather than hardcoding a band-name mapping.

More source votes changed canonical durations, for example Blinding Lights from Apple 200046 ms to the 202000 ms reported by several added sources. Repeated catalog values are not proof of independent correctness; per-source alternatives remain exposed. Get Lucky remains probable. No verified ISRC was manufactured. Timings are individual requests; most warm requests were 1–4 ms, but a provider-health change caused Get Lucky to resolve again.

## Korean exact detail and additional credits

A separate Korean artist query for Dynamite accepted Bugs `31999479`, Genie `90705547` and Melon `32872978`, with 199000 ms, 2020-08-21 and native artist IDs `80079394`, `80221635`, `672375`. Genie supplied composer and lyricist credits for David Stewart and Jessica Agombar. Genre labels were retained in their source language.

Initial exact-detail requests were variable: Bugs succeeded in 6.137 seconds; Genie and Melon hit the 8-second transport timeout and returned explicit source errors inside the debug extraction envelope. A later request in the same isolated run succeeded for all three: Bugs 1.410 s, Genie 1.509 s, Melon 1.914 s. The successful Korean multi-source resolution took 5.115 s. These failures remain in the saved smoke summary; HTTP 200 on an extraction envelope alone does not mean its source succeeded.

## Python through the Go API

The richer smoke run used two Python workers, an eight-second worker job budget and a 15-second API budget (distinct from the normal matrix above). Observed:

- `/v1/catalog` song context succeeded; unavailable watch/player components were treated as partial data.
- The first artist request failed; a later explicit recheck succeeded in 0.387 seconds and included biography/description runs, audience metrics, artwork, songs, albums, singles, videos and related sections as supplied by YouTube.
- A direct album request timed out. The accepted Boyfriend resolution subsequently retained its album context and Karan Aujla profile, together with song context, while another artist request was unavailable. Debug status correctly reported `partial` and canonical IDs stayed Apple `1829591986` / YouTube `tIjUfo30r4M`.
- Album context includes album type/year, artwork, track count, audio-playlist reference, bounded track list, alternate versions and returned duration. Artist Topic browse references can resolve to official channel references without rewriting the canonical IDs.

Context is returned under `provider_data.ytmusic.extra.youtube_catalog`. The pipeline does not promise every section on every request. Failed sibling lookups retain successful context, and partial data uses short cache retention. Account/private-library fields and streaming data are excluded. Python currently uses public mode; authenticated Python behavior is not claimed.

## Checks and limitations

`go build ./...`, `go vet ./...`, `go test ./...`, `go test -race ./...` and five Python unit tests passed. Additional focused race tests cover shared work after caller cancellation, worker timeout/oversize recovery, queue/process limits, private error redaction, coalescing/cache reuse and partial-result TTL. Parser fixtures cover exact ID mismatch, credits, unknown extra fields, search parsing and HTML error pages; API tests cover catalog input validation and disabled-route policy through both routes.

The optional Docker target was updated but not built here because Docker is unavailable. CPU, memory and throughput comparisons against Go-only mode remain unmeasured. JioSaavn/MusicBrainz/Deezer still returned HTML access-error pages; no credentials were required or tried for those public paths. Korean album/artist catalog endpoints and charts are not implemented in this change. Unlimited YouTube catalogs and alias/transliteration resolution remain limited.

Machine-readable observations: [song matrix](test-results/2026-10-18-song-matrix.json), [public probes](test-results/2026-10-18-public-probes.json), [rich smoke including failures](test-results/2026-10-18-rich-smoke.json), [returned YouTube context field names](test-results/2026-10-18-youtube-context.json).

Repeat standard songs with `tools/live_song_matrix.py`. Repeat rich endpoint probes with `tools/live_rich_catalogs.py --server bin/ommr-server --python .venv/bin/python --output /private/ommr-rich-results`; omit `--python` for the Go-only probe. Live results can change by region, network and catalog edition.
