# Termux run artifacts

See ../../TERMUX_PROVIDER_VERIFICATION.md for findings, verified recording identities and reproduction.

- `connectivity.json`: initial four requested public pages, original body hashes, safe header allowlist.
- `before-*.json`: explicitly retained failures before search/artist fixes; never successes.
- `<song>-<provider>.json`: first isolated adapter search matrix. `metadata_available` means some metadata decoded, not that the intended recording matched. This predates the final Deezer artist filter.
- `final-*-routes.json`: final isolated route diagnostics, sequential by provider.
- `mera-page.json`: final standalone URL check; one page request and no API fallback.
- `*-extract.json`: final server's independent extraction candidates, not accepted identities.
- `*-resolve.json`: final server provider-only resolution matrix; includes real intermittent upstream errors and circuit suspension.
- `*-retry.json`: requests still inside cooldown; not recovery evidence.
- `*-after-cooldown.json`: actual later server requests and health snapshot after cooldown.
- `get-lucky-musicbrainz-isrc.json`: separate explicit ISRC/album-context query. Same recording ISRC verified; returned compilation release context remains reported.
- `*-normal-*.json`: normal resolution cold, warm production and cached debug results.
- `page-*.json`, `native-extract.json`, `share-extract.json`: exact page GET/POST parity and native/share API extraction.
- `http-*.json`: request inputs, observed HTTP status and timings; byte sizes describe original HTTP bodies.
- `song-provider-results.json`: verified outcomes and metadata/missing fields, with links to source responses.
- `verification-assertions.json`: metadata parity, unknown precision, production/debug, cache and wrong-cover assertions.
- `checks.json`: final build/vet/test exit codes and exact unsupported-race message.
- `server-config.yaml`: public-only configuration used in this focused test, with fresh scratch cache and empty credential-directory path.
- `api-routing-captures.json`, `fixture-manifest.json`: safe capture summaries and original hashes.

JSON responses are sanitized and compacted; preview fields and signed preview URL parameters are omitted. Cookies, authorization headers and playback blobs are not included. Original response hashes/byte counts refer to original network data, not the sanitized artifacts. No fixture or independent extraction candidate is counted as live identity success. Machine report metadata can include provider-reported compilation contexts and missing duration; see the narrative for limitations.
