# Credential-free song matrix — 2026-10-17

Run with a fresh isolated server, empty cache, no inherited OMMR configuration or credentials, US Apple storefront and a 10-second API deadline. `tools/live_song_matrix.py` launches and stops the server and records full debug/public responses plus startup/final provider health. Python standard library only.

```sh
go build -o bin/ommr-server ./cmd/server
python3 tools/live_song_matrix.py --server bin/ommr-server --output /private/ommr-live-results
```

The first pass revealed an unrequested remix for Tum Hi Ho and one initial Boyfriend timeout. The fix rejects conflicting explicit version qualifiers unless an exact native ID or ISRC establishes identity, and reserves up to 500 ms (20% for short deadlines) for merging/caching/serialization after upstream work expires. A deterministic slow-provider regression verifies available data survives the provider deadline. The final live pass below returned HTTP 200 and nonempty tracks for all 12 queries; both response modes preserved identical recording IDs and provider data. Eleven identity labels were strong; Get Lucky remained probable. These labels are resolver assessments, not independent recording verification.

| Query | Returned performers | Apple ID | YouTube ID | Duration ms | Identity | Cold seconds |
| --- | --- | --- | --- | --- | --- | --- |
| Boyfriend — Karan Aujla | Karan Aujla; Ikky | 1829591986 | tIjUfo30r4M | 160796 | strong | 7.807 |
| Tu — Talwiinder | Talwiinder; Sanjoy | 1755529616 | uOQG38DriDM | 132086 | strong | 0.599 |
| Get Lucky — Daft Punk | Daft Punk; Pharrell Williams; Nile Rodgers | 617154366 | 4D7u5KF7SP8 | 369629 | probable | 2.476 |
| Blinding Lights — The Weeknd | The Weeknd | 1499378607 | J7p4bzqLvCw | 200046 | strong | 3.005 |
| Hello — Adele | Adele | 1544494392 | yjo_aXygRDI | 295502 | strong | 2.996 |
| Dynamite — BTS | BTS | 1596532400 | HaEYUJ2aRHs | 199054 | strong | 2.993 |
| Bohemian Rhapsody — Queen | Queen | 6781024437 | BSTsnWoslP4 | 355155 | strong | 2.995 |
| Tum Hi Ho — Arijit Singh | Arijit Singh | 1193103463 | fsiPzT50ZiM | 261974 | strong | 3.003 |
| Despacito — Luis Fonsi | Luis Fonsi; Daddy Yankee | 1447401620 | FXovf5dsRTw | 229387 | strong | 2.989 |
| Idol — YOASOBI | YOASOBI | 1679278167 | IzJ_ioBHrPU | 213234 | strong | 3.004 |
| Anti-Hero — Taylor Swift | Taylor Swift | 1649434293 | 3YgtjHZyCIQ | 200690 | strong | 2.993 |
| bad guy — Billie Eilish | Billie Eilish | 1450695739 | ZD6rXLXZOEI | 194088 | strong | 3.003 |

Tum Hi Ho changed from Apple remix ID `1073359675` (237338 ms) to original-recording compilation ID `1193103463` (261974 ms), alongside YouTube `fsiPzT50ZiM` (262000 ms). The canonical album remains the Apple compilation; the YouTube soundtrack album remains in provider alternatives. Original-album/earliest-release selection is not established by this change.

Production omitted provider execution diagnostics and debug field comparisons. Timings are individual observations, not a throughput benchmark. The nominal warm Get Lucky request re-resolved after provider health changed; most other warm requests were 1–3 ms. Production bodies were smaller while retaining accepted entity metadata.

Spotify anonymous search and Deezer/JioSaavn/MusicBrainz public access failed in this environment. Their circuits suspended after repeated failures, reducing later requests to currently selectable sources. This is not evidence these services are globally unavailable. All accepted recording IDs in this run came from Apple and YouTube Music. Required-key sources remained unloaded. Authenticated upstream coverage has not been live-tested.

Version checks cover explicit bracketed or dash-separated remix/live/acoustic/instrumental/karaoke/sped-up/slowed/radio-edit qualifiers. They do not solve every unlabelled version, alternate mix or translation. Exact IDs/ISRCs remain preferable. Alias/transliteration limits and cache migration are described in [ACCURACY_IMPROVEMENTS.md](ACCURACY_IMPROVEMENTS.md). Credentials accepted by the server are listed in [CREDENTIALS_CHECKLIST.md](CREDENTIALS_CHECKLIST.md).
