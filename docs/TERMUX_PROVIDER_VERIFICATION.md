# Termux public-provider verification — 2026-10-18

This run used the existing clean Android checkout, fetched `origin/codex/compact-debug-responses` at `7137f81`, and created `codex/termux-live-provider-fixes`. No browser login, operator credentials, playback requests, JavaScript execution, TLS changes or VPN/proxy changes were used. The server child received only system runtime environment variables, an empty credential-directory path and a public-only test configuration.

## Connectivity evidence

All four requested pages returned real public HTML from this Termux process. See [sanitized connectivity records](test-results/termux-2026-10-18/connectivity.json) for requested/final URLs, redirects, status, content types, sizes, SHA-256 and allowlisted headers. Deezer redirected once to `/ja/`. This establishes process connectivity, not whether the process traversed Proton VPN. No identical 195-byte Site Unavailable body was observed here; earlier remote observations remain specific to those runs. The homepages are public site content, not evidence of successful metadata decoding.

The supplied JioSaavn URL returned an 83,667-byte page. Its canonical URL is `https://www.jiosaavn.com/song/mera-mann-juss-mixsingh/ERIkRAwJf38`: the slug differs but the share token is unchanged. There was no JSON-LD MusicRecording. Its `window.__INITIAL_DATA__.song.song` contains the requested recording, separately from `song.modules` recommendations and artist-page modules. The state includes `new Date(...)` and `undefined` outside the song subtree; decoding the whole state as JSON fails. The song subtree itself is JSON.

Fetched visible heading and embedded fields agree with the supplied comparison: “Mera Mann : Juss & MixSingh”; Juss and Mixsingh; album Mera Mann; 2024; Punjabi; 163 seconds; © 2024 Speed Records (under exclusive assignment to Times Music). The page reports native song ID `azOux9HL` separately from share token `ERIkRAwJf38`. Structured artist rows additionally report native IDs and roles `music`, `singer`, `lyricist`; these are source evidence, not roles inferred from the comparison excerpt. `music` is retained as reported rather than silently renamed composer. Singer rows remain in provider data. The page reports only year precision and no explicit status, ISRC, genre or exact date.

The page-referenced public Song client bundle renders title/album display objects and artist rows. A page-referenced application bundle contains `webapi.get` metadata routing for entity tokens. Direct public metadata probes independently confirmed both `song.getDetails&pids=azOux9HL` and `webapi.get&type=song&token=ERIkRAwJf38` return the intended recording. They report 163 seconds, label Speed Records, release date 2024-11-14 and `explicit_content: 0`. These more precise values belong to the API responses, not the HTML-only extraction. No media endpoint was fetched. See [routing captures](test-results/termux-2026-10-18/api-routing-captures.json).

## Confirmed defects and changes

- **JioSaavn:** decode only the top-level requested-song JSON subtree within the observed serialized state, without evaluating JavaScript. Map display objects, image arrays, role-marked artist rows, native identifiers and year precision into existing provider records. Reject wrong share identities, ambiguous/duplicate state and unmarked artists. Preserve safe source extras in existing provider data; omit playback fields. Existing API routing already distinguishes native IDs from share tokens correctly and remains unchanged.
- **Deezer:** combined advanced artist/track filters returned an empty result for Boyfriend while the same public service's text query returned exact recording `3495188951` (160 seconds). Use one text search capped at five rows. Actual Daft Punk rows localized to Japanese without an English Accept-Language header; the header restores the reported English artist name (`lang=en` did not). Require a normalized exact reported artist name for artist/title text searches, since near-name tribute artists can otherwise pass fuzzy matching. The initial broad-query experiment wrongly accepted Daft Punk Experience; that failure is preserved as explicitly named before-fix evidence and is excluded from successes. Exact-ID validation, ISRC routing and all resolver thresholds remain unchanged.
- **MusicBrainz:** homepage and correctly decoded recordings work from Termux. Intermittent HTTP 503 JSON API errors occur, including in isolated routes. This is an observed upstream response, not proof of global unavailability or a demonstrated adapter parsing defect. No speculative adapter rewrite or automatic retry expansion was made.

Resolved cache namespace advances from `rich-v10` to `rich-v11` to invalidate old accepted matches; TTL, backend, source health and presentation-mode cache sharing are unchanged. No architectural design changed, so no ADR was added.

## Environment and reproduction

Termux packages: Git 2.56.0, Go 1.27.2 (`android/arm64`, satisfying `go.mod`'s Go 1.25.0 minimum), Python 3.14.6, curl 8.22.0. Missing Go/Python and ripgrep were installed with the normal Termux package manager; its compiler dependencies were installed automatically. No dependencies were installed through a browser or credential flow.

```sh
cd "$HOME/test/OMMR"
export PATH="$PREFIX/bin:$PATH"
git fetch origin codex/compact-debug-responses
# This run already created codex/termux-live-provider-fixes; preserve local work.
pkg install golang python ripgrep # Only when missing.
go build ./...
go vet ./...
go test ./...
go test -race ./internal/adapters/webdata ./internal/adapters/jiosaavn ./internal/adapters/deezer ./internal/resolver

# Run individually, sequentially.
go run ./cmd/providercheck -provider jiosaavn -routes
go run ./cmd/providercheck -provider deezer -routes
go run ./cmd/providercheck -provider musicbrainz -routes
go run ./cmd/providercheck -provider jiosaavn \
  -id 'https://www.jiosaavn.com/song/mera-mann/ERIkRAwJf38' -id-type jiosaavn_url
```

Use a fresh private test configuration, an unused port and a fresh cache directory to reproduce cold results. Enable the three providers under test and disable credential sources/enrichment for this focused run. The exact non-secret server configuration is included with the reports. Do not copy operator credentials into it. Start `go run ./cmd/server --config /path/to/test.yaml` and execute the requests recorded in [http-requests.json](test-results/termux-2026-10-18/http-requests.json). MusicBrainz calls must remain sequential, no more than one per second across API and website; the adapter and diagnostic command share budgets within their processes. Avoid simultaneous server and diagnostic MusicBrainz runs.

```sh
curl -sS --get 'http://127.0.0.1:18080/v1/extract' \
  --data-urlencode provider=jiosaavn \
  --data-urlencode 'url=https://www.jiosaavn.com/song/mera-mann/ERIkRAwJf38' \
  --data-urlencode debug=true
curl -sS 'http://127.0.0.1:18080/v1/query' -H 'Content-Type: application/json' \
  --data '{"operation":"extract","provider":"jiosaavn","url":"https://www.jiosaavn.com/song/mera-mann/ERIkRAwJf38","debug":true}'
curl -sS --get 'http://127.0.0.1:18080/v1/resolve' \
  --data-urlencode artist='Karan Aujla' --data-urlencode title=Boyfriend \
  --data-urlencode sources=deezer --data-urlencode debug=true
```

Official MusicBrainz [API](https://musicbrainz.org/doc/MusicBrainz_API), [Search](https://musicbrainz.org/doc/MusicBrainz_API/Search) and [Rate Limiting](https://musicbrainz.org/doc/MusicBrainz_API/Rate_Limiting) documents were fetched and read in Termux before testing. Public metadata needs no API key; requests use an application/version/contact User-Agent. Search uses Lucene query fields; JSON/XML formats are supported. HTTP 503 can represent throttling/service pressure; the specific source of these observed intermittent errors remains unconfirmed.

## Verification artifacts

Captured fixtures are labeled in their testdata READMEs. The JioSaavn fixture retains the captured requested-song subtree and minimizes surrounding non-JSON state; it excludes unrelated modules and playback fields. Deezer fixtures retain actual search results, including cover rows for the artist regression, and omit preview URLs. Existing constructed fixtures remain explicitly labeled as constructed. JSON reports omit response cookies, credentials and signed preview URLs; body hashes describe the original capture, not a rewritten fixture. Extraction reports are independent candidates and do not establish accepted recording identity; resolve reports and the results table distinguish accepted, rejected and unavailable sources.

## Per-song results

These are live accepted recordings, not raw extraction candidate counts. Durations and identifiers remain source-specific. MusicBrainz's main matrix hit upstream 503s and then suspended; the table includes the clearly labeled after-cooldown run. [Machine-readable per-song results](test-results/termux-2026-10-18/song-provider-results.json) link each row to its underlying response and missing fields.

| Recording | JioSaavn | Deezer | MusicBrainz after cooldown |
| --- | --- | --- | --- |
| Juss / Mera Mann | Verified, 163 s, native `azOux9HL`, share `ERIkRAwJf38` | Verified, 163 s, `3088162071`, ISRC `INM432400484` | Candidate rejected (`identity_mismatch`); not a success |
| Karan Aujla / Boyfriend | Only Aye Manny remix found; rejected | Verified, 160 s, `3495188951`, ISRC `CAW112500965` | Verified, 160.796 s, `c485675e-9c06-479b-b2dc-35a565d08558`, same ISRC |
| Daft Punk / Get Lucky | Karaoke/cover search candidates rejected | Verified original album recording, 367 s, `67238735`, ISRC `USQX91300108` | Plain search edition unverified; separate exact-ISRC request verified original recording, 369 s, `833f00e1-781f-4edd-90e4-e52712618862` |
| Arijit Singh / Tum Hi Ho | Verified, 262 s, native `aRZbUYD7`, share `EToxUyFpcwQ` | Verified, 261 s, `3191972361`, ISRC `INS181303031` | Verified, 262 s, `3e5bc764-dfa4-454d-a2a9-0ee84ae35db2`, same ISRC |

Deezer also returned the real Get Lucky radio edit (`66609426`, 248 s) alongside the album recording; the final resolver selected the album recording and excluded the 204-second Daft Punk Experience cover (`1819285597`). MusicBrainz's plain-search Get Lucky selection lacked duration and ISRC and was associated with a soundtrack compilation; it is explicitly not counted as verified. The exact-ISRC query establishes recording identity but still reports a compilation album rather than the requested Random Access Memories album context. MusicBrainz's Tum Hi Ho release context is also a later compilation; the exact ISRC identifies the recording, not its original release. These album-context limitations remain visible. One- or two-second duration differences are retained rather than overwritten in per-source reports.

### Recovered fields and remaining gaps

The HTML-only Mera Mann extraction now recovers title, structured artists and artist IDs, album title, native/share identifiers, 163000 ms duration, artwork, year-only release precision, language, play count, copyright and explicitly reported music/lyricist credits. Artist singer rows and original display metadata remain in provider data. It does not invent native album IDs, label from copyright, exact date, ISRC/ISWC, genre or explicit status. The separately captured JioSaavn API gives label, exact release date and clean status, with that source precision. Deezer details recover album/artist IDs, label, exact dates, clean/explicit evidence, positions, artwork and ISRC for all four verified recordings. MusicBrainz supplies MBIDs, credited artists, ISRCs where reported, durations and release context; roles/genres/ISWC may remain absent. Missing-field arrays in the extraction reports and the per-song summary retain these gaps.

### API, cache and unavailable-source checks

The real Go server used the three public providers, disk cache and normal 10-second provider deadline. Catalog enrichment and unrelated providers were disabled to keep this run focused and avoid separate MusicBrainz request budgets. GET `/v1/extract` and equivalent POST `/v1/query` produced identical page metadata. Native and share extraction separately decoded the intended song. Production output omitted matching/provenance diagnostics; debug included them. HTML-only explicit status stayed `unknown`.

Normal resolution succeeded for all four songs. Cold/warm timings in milliseconds were Mera Mann 1266/13, Boyfriend 1117/40, Get Lucky 1224/8 and Tum Hi Ho 1390/10. Switching to debug reused the same accepted IDs and cached matched sources. Production responses are smaller than debug, but source metadata remains substantive; “compact” does not promise tiny fixed-size payloads. See [assertions](test-results/termux-2026-10-18/verification-assertions.json).

Repeated real MusicBrainz 503s suspended its health circuit. Provider-only requests then returned HTTP 503 with `no provider adapters available`; normal resolution still worked with the other sources. Requests during cooldown were preserved separately from the successful after-cooldown retry. The manager returned to active with zero failures after actual successful requests. Disabled Qobuz extraction was also tested without collecting credentials. [Before/after health](test-results/termux-2026-10-18/health-after.json) and [after-cooldown health](test-results/termux-2026-10-18/health-after-cooldown.json) capture those transitions.

### Build and environment results

Final `go build ./...`, `go vet ./...`, and `go test ./...` all passed. Focused fixture, wrong-artist, exact-ISRC, URL and parser-boundary regressions passed. The initial parallel suite hit an existing Python worker test's 250 ms startup deadline; it passed standalone and in the serial full suite. A later parallel link was killed by Android (`signal: killed`); resource pressure is a hypothesis, not a confirmed kernel diagnosis. `go test -p 1 ./...` passed, followed by passing default full runs. No unrelated worker/runtime timeouts were loosened.

Focused race checks were attempted for all changed runtime packages and the resolver. Go returned exactly `-race is not supported on android/arm64` (exit 2). Race tests did not run on this device. See [exact final commands, outputs and exit codes](test-results/termux-2026-10-18/checks.json).
