# Song, album and artist tests across styles — 2026-10-19

Tested 14 styles/version cases through the running API, following accepted source identifiers into native album and artist lookup. No credentials or login were used. The suite found and corrected three metadata/identity issues; the final results include targeted reruns after those fixes.

## Final results

- **14/14 song queries returned metadata**, with structurally valid recording graphs.
- **13 exact Apple album responses**, all containing their originating Apple track ID and valid album graphs: **11 complete, 2 explicitly partial**.
- **14 artist profiles**: 13 followed exact Apple artist IDs; Lana Del Rey was independently resolved by unique exact-name Apple artist search because the accepted remix sources did not provide an Apple track ID.
- Four optional public YouTube artist probes produced two richer profiles and two source failures. BTS and Glenn Gould returned bios, images, top tracks and audience metrics.
- No remaining **large reported duration contradictions** or mislabeled song-row album totals in the final matrix. Unknown durations remain unknown and are not audio-identity proof.

Categories are test labels, not an inferred genre taxonomy. Album titles identify the source edition actually returned; compilation/reissue context need not be the original album.

| Test type | Song / artist | Native Apple album | Returned / declared items | Artist profile |
| --- | --- | --- | --- | --- |
| Punjabi pop | Boyfriend — Karan Aujla | P-POP CULTURE | 11/11, 2 discs | Karan Aujla |
| Hindi soundtrack | Tum Hi Ho — Arijit Singh | Love Dose Arijit Singh | 10/10 | Arijit Singh |
| English pop | Blinding Lights — The Weeknd | After Hours | 14/14 | The Weeknd |
| Hip-hop | HUMBLE. — Kendrick Lamar | DAMN. | **14/15, partial** | Kendrick Lamar |
| K-pop | Dynamite — BTS | Proof | 35/35, 3 discs | BTS |
| Japanese pop | Idol — YOASOBI | アイドル - Single | 1/1 | YOASOBI |
| Latin pop | Despacito — Luis Fonsi | VIDA | 15/15 | Luis Fonsi |
| Rock | Bohemian Rhapsody — Queen | A Night At The Opera (Deluxe Edition) | 18/18, 2 discs | Queen |
| Electronic | One More Time — Daft Punk | Discovery | 14/14 | Daft Punk |
| Instrumental | Experience — Ludovico Einaudi | Cinema | 28/28 | Ludovico Einaudi |
| Classical | Goldberg Variations, BWV 988: Aria — Glenn Gould | The 1955 & 1981 Recordings | **64/65, partial** | Glenn Gould |
| Live | Hotel California (Live) — Eagles | Eagles Live | 15/15, 2 discs | Eagles |
| Acoustic | Everlong (Acoustic Version) — Foo Fighters | Greatest Hits | 16/16 | Foo Fighters |
| Remix | Summertime Sadness (Cedric Gervais Remix) — Lana Del Rey | No accepted source ID supported by native album service | Not looked up | Lana Del Rey, independent exact-name lookup |

The remix returned Bugs, Genie and Melon recording IDs. Native albums currently support Apple, Deezer, JioSaavn and MusicBrainz, so inventing an Apple album relationship to fill that cell would be incorrect. This is a coverage gap, not a failed exact album lookup.

DAMN. and the Goldberg collection report one more collection item than the public song lookup returns. Both remain `complete: false`, reason `incomplete_response`. No missing item is invented. This run did not establish whether the difference is bonus video, another media type, catalog omission or another cause.

## Defects found and fixed

### 1. A song-row count was mistaken for the whole album total

Apple's Boyfriend song row has `trackCount: 5`, `discCount: 2`; the native P-POP CULTURE collection has 11 tracks across two discs (5+6). BTS's Proof similarly has 19 on the song row but 35 on the collection. Single-disc rows can also differ from collection totals: HUMBLE. reports 14 while DAMN. declares 15 items. The original matrix found six such discrepancies.

The public Apple track adapter no longer assigns a song row's `trackCount` to canonical `album.total_tracks`. The unmodified source count remains in provider catalog data. Use `/v1/albums` for the collection's actual declared total and explicit completion. This corrects the meaning without adding a separate album fetch to every song request. Official authenticated Apple catalog album totals are unaffected. The public adapter version advances to `applemusic-v2-album-counts`.

### 2. Original Experience merged with a Reimagined recording

The initial query for Experience accepted:

| Source | Accepted title before fix | Duration |
| --- | --- | --- |
| Apple | Experience (Reimagined), ID 1750709920 | 380082 ms |
| YouTube | Experience (Reimagined), ID X0uOx8nj67M | 381000 ms |
| Genie | Experience, ID 81785038 | 316000 ms |
| Melon | Experience (Reimagined) | Unknown |

The missing `reimagined` version marker allowed a different recording to pass title/artist checks. Added reimagined/re-imagined/reimagining recognition in qualified titles. The original query now returns **Experience**, Apple ID **1559739399** and Genie ID **81785038**, approximately **316–317 seconds**. The rejected versions no longer contribute IDs or graph metadata. A separate live positive control explicitly requested Experience (Reimagined): it returned Apple ID 1750709920, Bugs 115360300 and YouTube X0uOx8nj67M at approximately 380–381 seconds, its complete four-track EP and the artist profile. This additional control is separate from the 14-row corpus.

### 3. Different Gould performances shared the same title/artist

The original Aria response merged Apple/Bugs/Genie observations around **112–113 seconds** with YouTube ID **43sTxRVpRBM** at **185 seconds**. Album metadata identifies the former as 1955 context and the latter as 1981 context. Title and performer agreement alone could not establish the same performance.

Added a conservative material-duration check: a text query requires at least two distinct provider observations in a strict majority cohort before rejecting known timing outliers. Each provider contributes at most one vote. The allowed difference is the larger of **10 seconds or 5%**; this targets large contradictions, not ordinary rounding/fades. Ties, insufficient evidence and missing durations do not manufacture a consensus. This is a metadata consistency rule, not fingerprinting or verified source independence.

For a direct-ID request without an exact ISRC anchor, the known direct recording duration anchors the check; a majority cannot replace the requested recording. Exact requested ISRC acceptance remains unchanged.

The check runs before identity-match emission when timings are already known and **again after accepted detail hydration**, because Korean search rows can omit durations that their detail pages supply. Post-hydration rejection also removes identity matches, matched status, provider data and graph membership before merging. The live final Aria result excludes the 185-second YouTube ID, while retaining the 112–113-second cohort. Existing title/artist/ISRC acceptance floors were not weakened.

Resolved caching advances to **rich-v15** so older mismatched recording IDs and incorrect album totals cannot survive deployment. Native album caching remains `albums:v3` because its collection totals were already correct.

## Artist information and remaining gaps

| Source | What was actually returned |
| --- | --- |
| Apple, 14 profiles | Native ID, display name, exact-ID or unique-name identity method, source genre/type metadata and five recent tracks. No biography, image or audience metric in these public responses; unknown counts remained absent |
| YouTube BTS | Biography, five images, three top tracks, subscribers, monthly audience and view-count observation |
| YouTube Glenn Gould | Biography, five images, three top tracks, subscribers, monthly audience and view-count observation |
| YouTube Karan Aujla / Ludovico Einaudi | Source errors; no successful profile claimed for these attempts |

Audience values retain their source label, period and precision. For example, abbreviated subscribers/monthly audience remain display strings rather than invented exact integer counts. Monthly audience is not monthly plays.

All 14 song responses lacked flat detailed contributor credits in this public-source corpus. The graphs retain source artist credit relationships, but no composer/lyricist/producer data is invented. Rich MusicBrainz credit relationships remain fixture-tested; the provider could not be reached here. Public iTunes artist profiles are intentionally limited, so profile HTTP success does not imply a full biography/discography response.

Source genre labels were retained, including Korean labels and differing capitalization; this run did not validate a universal genre taxonomy. The suite did not separately establish clean/explicit recording equivalence, authenticate any provider, fingerprint audio, or prove identity for sources with missing timing. Additional named remixes, live venues, remasters and sparse/tied timing cohorts can still require stronger evidence.

## Verification and evidence

The final full Go build/vet/race/coverage suite passed with **252 top-level tests / 337 cases**, zero failures/skips and **75.4% statement coverage**. Eight Python worker tests also passed. Results are recorded in [checks](test-results/entity-matrix-2026-10-19/checks.json). Regressions cover song-count semantics, unrequested/explicit Reimagined handling, material timing contradictions, duplicate-source votes, tied cohorts, post-hydration rejection, graph exclusion and direct-ID/ISRC preservation.

[Final matrix](test-results/entity-matrix-2026-10-19/final.json), [summary](test-results/entity-matrix-2026-10-19/summary.json), [initial observations](test-results/entity-matrix-2026-10-19/baseline.json), [final targeted rechecks](test-results/entity-matrix-2026-10-19/rechecks.json), and [YouTube artist probes](test-results/entity-matrix-2026-10-19/youtube-artists.json) retain outcomes rather than hiding missing data. Before/after full responses for Experience and Aria are saved alongside them. The intermediate run predates post-hydration filtering; the final targeted Aria rerun supersedes that row.

```sh
go build -o bin/server ./cmd/server
python tools/live_entity_matrix.py --server bin/server --output "$TMPDIR/ommr-entity-matrix"
```

The script uses only Python's standard library, starts an isolated public-only server, follows accepted source IDs, records partial/failed outcomes, and flags large reported timing conflicts. Native artist-ID lookup is preferred; a missing native ID may trigger a separate unique exact-name Apple artist search. Catalog IDs, totals and availability can change; saved evidence describes these runs. Recommended next work: native album support for the Korean sources, richer reliable credits/artist profiles, and additional version-specific identity tests.
