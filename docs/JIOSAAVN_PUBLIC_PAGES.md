# JioSaavn public song pages — 2026-10-18

**Later Termux verification:** the exact page is reachable on Android. Captured serialized display state is now decoded without executing JavaScript; see [live findings and results](TERMUX_PROVIDER_VERIFICATION.md). The remote-access observations below are historical and do not describe the Termux network. Current resolved cache namespace is `rich-v11`.

The user supplied `https://www.jiosaavn.com/song/mera-mann/ERIkRAwJf38` and its visible About text. The excerpt describes “Mera Mann : Juss & MixSingh”, artists Juss and Mixsingh, album Mera Mann, year 2024, Punjabi, duration 2:43 (163000 ms), and © 2024 Speed Records (under exclusive assignment to Times Music). These are public metadata observations supplied by the user, not a live response retrieved here. A normalized transcription is saved [separately](test-results/2026-10-18-mera-user-excerpt.json); it is not inserted into runtime results or used as a song-specific hardcoded fallback.

The excerpt does not establish ISRC, a native song ID, genre, explicit status, an exact release date or contributor roles. Its empty “Songs By Same Artists” / “Top Songs By Same Actors” sections are related content, not attributes of this recording. A playback subscription requirement does not imply that the public metadata requires that subscription. No playback endpoint is called by this implementation; the excerpt alone does not establish whether the site's underlying API changed.

## Public URL extraction

```text
GET /v1/extract?provider=jiosaavn&url=https%3A%2F%2Fwww.jiosaavn.com%2Fsong%2Fmera-mann%2FERIkRAwJf38&debug=true
```

```json
{"operation":"extract","provider":"jiosaavn","url":"https://www.jiosaavn.com/song/mera-mann/ERIkRAwJf38","debug":true}
```

Both use the existing extraction endpoint and policy. The URL path is:

1. Validate an official HTTPS `/song/<slug>/<share-token>` URL and normalize to `www.jiosaavn.com`. Reject foreign/userinfo hosts, ports, queries, fragments, non-song paths, traversal, and ambiguous URL/id inputs before network access.
2. Request that exact public song page through the existing five-second, bounded HTML fallback client with same-host HTTPS redirects and source request budgets. Decode known native app state, embedded JSON or JSON-LD MusicRecording records.
3. Require exactly the requested provider-owned share identity. Standard JSON-LD song metadata retains structured artist names, album, duration, reported date precision, language, artwork, copyright notice and extra structured catalog fields. Native IDs are absent unless a native record actually supplies them.
4. If recognized page data is unavailable, try the existing exact `webapi.get&type=song&token=...` lookup within the remaining context. Reject a contradictory or ambiguous recording; do not substitute related songs or nearby matches.

Page metadata uses the same canonical normalizer and public/debug presentation as API records. Page-only layouts that provide unstructured prose need verification against actual HTML before further selectors are implemented. No generic prose guesser, JavaScript execution, cookie login, media download or paid-playback access is introduced. Resolved cache namespace is `rich-v10`.

## Verification and current access limit

The supplied URL was retried directly in the remote browser and from the server runtime. Both still showed Site Unavailable. The server received HTTP 200, HTML, 195 bytes, OpenResty, SHA-256 `5b131ca14aa96311d3432b0062c7443d3b0e6346ec279bd376a75f0a76bcd5d7`. Deezer and MusicBrainz homepages returned the same response in both environments. The user's successful Japan-VPN access is a separate observation; this runtime has not been shown to share that VPN path. No VPN, proxy or browser network settings were changed here.

The individual adapters were also retried after the homepage checks: the Mera Mann exact page/share lookup and Juss/Mera Mann searches on Deezer and MusicBrainz. See the saved reports for actual responses rather than interpreting fixture success as restored live access:

- [Homepage/song-page probes](test-results/2026-10-18-mera-home-probes.json)
- [JioSaavn page-first extraction](test-results/2026-10-18-mera-retry-mera-page.json)
- [Deezer](test-results/2026-10-18-mera-retry-deezer.json)
- [MusicBrainz](test-results/2026-10-18-mera-retry-musicbrainz.json)

Repeat the exact URL path with the isolated command:

```bash
go run ./cmd/providercheck -provider jiosaavn \
  -id 'https://www.jiosaavn.com/song/mera-mann/ERIkRAwJf38' \
  -id-type jiosaavn_url
```

The page test fixture is constructed JSON-LD based on the supplied metadata, not captured site markup. It verifies protocol behavior, parsing, metadata retention and identity checks; it does not prove this exact live page currently uses that structure. GET/JSON parity and unsafe-target tests verify the API path.

Fresh adapter results: JioSaavn's page request reported a network error and its share API returned unavailable HTML. Deezer's API returned unavailable HTML and its public fallback timed out. MusicBrainz's API and public fallback returned unavailable HTML. Separate direct page probes returned the 195-byte body described above. Build, vet, full Go tests and focused race checks all passed.
