# Artist profile live verification — Termux, 2026-10-18

See ../../ARTIST_PROFILES.md for schema, identity, metric semantics, settings and examples.

These are actual responses from the real Go server running in Termux with public-only configuration and optional ytmusicapi 1.12.3. No cookies, logins, API keys or media downloads were used. The optional dependency already existed in the project; its installation remains local to the ignored .venv.

- gur-sidhu/daft-punk/arijit-singh-{cold,warm,production}.json: three live name searches, source-specific ambiguity/partial/error outcomes, cached observation times and compact production mode. A failed source is not counted as a profile.
- apple-exact.json and youtube-exact.json: exact native artist IDs resolve ambiguity and provide source profiles without choosing a same-name entry arbitrarily.
- deezer-exact-get/post.json: identical production metadata through GET and the JSON operation gateway.
- go-only-native.json: successful native Deezer/JioSaavn profiles with the Python extractor disabled.
- go-only-youtube.json: explicit worker_unavailable status while native profiles remain usable.
- invalid-url.json and unknown-source.json: HTTP 400 before upstream access.
- unknown-artist.json: no fabricated profiles for an unmatched name.
- summary.json: normalized observed fields and ambiguity IDs for the named artists. Values are source/time-specific, not hardcoded runtime data or globally summed totals.
- requests.json: exact request parameters, status and elapsed times.
- assertions.json: live identity/precision/cache/parity/Go-only assertions.
- final-smoke.json: fresh native and YouTube requests against the final server binary, followed by clean shutdown.
- checks.json: exact final build/vet/full-test/worker-test results; focused race attempts report that android/arm64 is unsupported.
- config.yaml and go-only-config.yaml: sanitized non-secret test configurations, using empty scratch credential-directory paths.

Known live limitations: MusicBrainz intermittently returns HTTP 503; optional top-track requests can fail while a valid profile is retained as partial. Some sources return multiple exact-name artist IDs. JioSaavn can provide an empty top-song list depending on its current catalog response. No list length is promoted to an artist total, no yearly plays are extrapolated, and recommendations never become the requested artist's subscriber/listener metrics.
