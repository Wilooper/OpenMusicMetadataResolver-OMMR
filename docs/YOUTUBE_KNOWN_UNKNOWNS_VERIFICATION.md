# Known Unknowns: live OMMR album verification

The supplied YouTube Music URL identifies **Known Unknowns**, an album by **Gur Sidhu and Kaptaan**, reported year **2026**, with **10 tracks**. OMMR extracted the complete album from public metadata in Termux, then checked every song sequentially using direct YouTube extraction, public song context, and artist/title/album searches across YouTube Music, Apple, JioSaavn, Deezer and MusicBrainz. No credentials, browser login, media download or authentication bypass was used.

The album reports 28 minutes and 1709 seconds (28:29 summed from its rounded track durations). Exact per-provider recording durations differ slightly; the table below reports resolved audio durations rounded to seconds. Album artwork and source-specific artist IDs, album/track IDs, language, dates, labels, credits, explicit evidence, ISRCs and available extra catalog metadata are retained in the [complete JSON export](test-results/youtube-album-MPREb_ToFqhXgz9A7/album-complete.json). Unknown fields remain missing. The export distinguishes the final canonical response from verified source observations gathered across the runs; it does not overwrite the canonical response with stale failed results.

## Findings and fixes

**Wrong song from the same artist and album:** Apple text search for FOLK MAFIA returned other songs, including ON MY MIND, and the resolver accepted that track despite a weak title score. The old merged result contained Apple ID `6815521808` and an incorrect 175500 ms duration. Matching artist/album evidence could overwhelm missing title identity. Text resolution now requires title component evidence at the existing identity component floor (0.72), as direct cross-resolution already did. Existing 0.40/0.30 score thresholds, 0.45 artist gate, version checks, exact-ID and exact-ISRC behavior remain intact. A source that omits a parenthesized featured artist from the title can pass only with an exact base title and that named artist explicitly present in its artist records. This preserves the verified HYPE/Jasmeen Akhtar record without admitting a different featured artist. Rejected candidates do not emit identity matches or provider data. Captured regression evidence is in `internal/resolver/testdata`.

The final text-query FOLK MAFIA result excludes the wrong Apple ID and has approximately 142–142.5 seconds according to accepted sources. Apple text search still omits its correct track; adding `explicit=Yes` did not fix the observed search ranking. Public album lookup identified the real Apple track `6815521939`, and OMMR's exact extraction/resolution verified FOLK MAFIA at 142500 ms and explicit status. This is reported as separate exact lookup, not a successful Apple text search. Resolved cache namespace is `rich-v12`, invalidating earlier wrong accepted matches while keeping cache backends and TTL behavior.

**Video versions substituted by YouTube:** The album itself points BREEZY to `vm92MBLZH4U` (official video, 158 seconds by direct extraction) and AFTERIMAGE to `W4MyRYw4i2o` (official video, 203 seconds). Raw public row navigation and playlist metadata agree with those IDs; this is not a demonstrated OMMR parser defect. Independent OMMR song searches identify the album audio as BREEZY `ba2rxdm4VcM` (~155.4 seconds) and AFTERIMAGE `fcweFZSTubY` (~153.8 seconds). Album source IDs are preserved. The export marks their difference from the independently accepted audio IDs; it never silently rewrites the supplied album rows or treats the longer video as the audio recording.

**Missing public credits operation:** The pinned existing ytmusicapi 1.12.3 exposes public `get_song_credits`, and all 10 album rows supply a credits browse ID. OMMR now supports `type=credits` with strictly validated `MPTC` plus an 11-character identifier, through GET catalog and equivalent POST query. It uses the existing bounded, optional worker, timeout/cache/health handling and private-field cleaning. Credit sections remain source data: performed_by, written_by, produced_by and music_metadata_provided_by. “Written by” is not guessed to mean lyricist or composer. The real COACH response is a labeled captured regression fixture. No new dependency was added beyond installing the existing pinned optional requirements locally.

**Availability and metadata gaps:** Deezer returned no intended recording for most album text searches; this is reported as no results, not global unavailability. MusicBrainz intermittently returned 503/errors or empty results; successful records from earlier and later requests are separately preserved. Release dates differ: Apple reports 2026-10-14 while JioSaavn/MusicBrainz report 2026-10-16. YouTube reports only 2026. The album-level YouTube explicit flag is false while its FOLK MAFIA row is explicit; source flags are preserved, and album false is not used to override the song. Apple combines the artist display name while YouTube/JioSaavn can report individual artists. Missing ISRCs, ISWC, UPC/barcode, detailed contributor roles and exact dates are not invented.

## Track results

All 10 tracks resolved after the fixes. Accepted-source membership can vary with upstream availability; it is not a promise that every provider contains every track.

| # | Track | Resolved duration | YouTube audio ID | Accepted text-search providers |
|---|---|---|---|---|
| 1 | COACH | 3:05 | `jO_BW28XPKk` | applemusic, jiosaavn, ytmusic |
| 2 | MANIFEST | 3:07 | `yfztrj4giCM` | applemusic, jiosaavn, musicbrainz, ytmusic |
| 3 | BREEZY | 2:35 | `ba2rxdm4VcM` | applemusic, jiosaavn, musicbrainz, ytmusic |
| 4 | NASHA | 2:55 | `5q3zCX8IFcI` | applemusic, jiosaavn, musicbrainz, ytmusic |
| 5 | ON MY MIND (feat. Real Boss) | 2:56 | `7NjGG0wHyA4` | applemusic, jiosaavn, ytmusic |
| 6 | KNOWN UNKNOWNS | 3:03 | `6hPeGHiiHdY` | applemusic, deezer, jiosaavn, musicbrainz, ytmusic |
| 7 | STREETS | 3:08 | `LE4kDhEyBhM` | applemusic, jiosaavn, musicbrainz, ytmusic |
| 8 | HYPE (feat. Jasmeen Akhtar) | 2:38 | `0mvmuQdxIWI` | applemusic, jiosaavn, ytmusic |
| 9 | FOLK MAFIA | 2:22 | `6xlBQWabV9Q` | jiosaavn, ytmusic |
| 10 | AFTERIMAGE | 2:34 | `fcweFZSTubY` | applemusic, jiosaavn, ytmusic |

Public YouTube credits report Gur Sidhu and Kaptaan as performers on all tracks, adding Real Boss on ON MY MIND and Jasmeen Akhtar on HYPE. They report Gur Sidhu under Written by and Produced by, and Brown Town Music as the metadata provider. JioSaavn's separate role-specific credits remain attributed to JioSaavn. Brown Town Music is reported as label in accepted catalog records; Punjabi/pan language values retain source provenance. FOLK MAFIA is the only explicit song reported in these captures. Genre and language are separate source fields.

## Reproduction and checks

```sh
cd "$HOME/test/OMMR"
export PATH="$PREFIX/bin:$PATH"
python -m venv .venv
.venv/bin/pip install -r workers/ytmusic/requirements.txt
# Enable the optional extractor with the absolute .venv Python and worker paths.
# The sanitized public-only test configuration is saved with the reports.
go build -o bin/server ./cmd/server
bin/server --config /path/to/private/test-config.yaml
```

```sh
curl -sS 'http://127.0.0.1:18081/v1/catalog?provider=ytmusic&type=album&id=MPREb_ToFqhXgz9A7&limit=100'
curl -sS 'http://127.0.0.1:18081/v1/catalog?provider=ytmusic&type=credits&id=MPTCjO_BW28XPKk'
curl -sS --get 'http://127.0.0.1:18081/v1/resolve' \
  --data-urlencode artist='Gur Sidhu' --data-urlencode title='FOLK MAFIA' \
  --data-urlencode album='Known Unknowns' \
  --data-urlencode sources='ytmusic,applemusic,deezer,jiosaavn,musicbrainz' \
  --data-urlencode debug=true --data-urlencode bypass_cache=true
curl -sS 'http://127.0.0.1:18081/v1/extract?provider=applemusic&id=6815521939&debug=true'
```

GET/POST album and credits responses were equal. Album limit=2 preserved total trackCount=10 and explicitly reported truncation. Invalid credit IDs were rejected before upstream access. Every final song's artist, album, duration and native ID were checked against source observations; wrong Apple ID 6815521808 is absent from FOLK MAFIA. Production/warm cache behavior was checked for that regression. Public reports exclude preview/media URLs and credentials.

Final Go build, vet, serial and default full tests passed, as did seven Python worker tests. Focused race checks were attempted; Termux Go returned `-race is not supported on android/arm64`. Full commands, exit codes, per-provider statuses, request parameters and before/after metadata are in the [report directory](test-results/youtube-album-MPREb_ToFqhXgz9A7/README.md).

## Verified source ISRCs

All ten ISRCs were recovered from matching source observations. Some final canonical requests still lack ISRC because a source was temporarily unavailable or the featured display text produced no result; the export preserves both the observed source fact and the actual canonical response. Additional shorter-title queries for ON MY MIND and HYPE supplied their MusicBrainz records. These are catalog recording identifiers, not credits or playback access.

| Track | ISRC |
|---|---|
| COACH | `QMFME2632830` |
| MANIFEST | `QMFME2632831` |
| BREEZY | `QMFME2632832` |
| NASHA | `QMFME2632833` |
| ON MY MIND (feat. Real Boss) | `QMFME2632834` |
| KNOWN UNKNOWNS | `QMFME2632838` |
| STREETS | `QMFME2632835` |
| HYPE (feat. Jasmeen Akhtar) | `QMFME2632837` |
| FOLK MAFIA | `QMFME2632836` |
| AFTERIMAGE | `QMFME2632839` |
