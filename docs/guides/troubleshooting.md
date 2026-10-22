# Troubleshoot an instance

[Docs](../README.md) · [Configuration](configuration.md)

First confirm the process is running, then inspect its configured capabilities,
source state and the failing operation independently:

```sh
curl -i http://localhost:8080/v1/health
curl http://localhost:8080/v1/capabilities
curl http://localhost:8080/v1/providers
curl --get http://localhost:8080/v1/resolve \
  --data-urlencode 'artist=Karan Aujla' --data-urlencode 'title=Boyfriend' \
  --data-urlencode 'sources=applemusic,ytmusic' --data-urlencode 'debug=true'
```

| Symptom | Check and next action |
| --- | --- |
| Build requests a newer Go version | Check `go version` and toolchain download access; use the tested Go 1.26 path |
| Connection refused | Process/startup logs, actual `OMMR_PORT`, host/container mapping |
| Platform 502 after deploy | Matching public target port and `OMMR_PORT`; startup exit/probe logs |
| Private-file permission error | Use a regular owner-only file (`chmod 600`) on Unix/WSL; avoid shared Android storage |
| `track: null` | Debug provider errors/rejections; correct artist/version/ID and try the exact source reference |
| HTTP 200 with "Site Unavailable" upstream | Access failure, not empty catalog; no parser can recover absent metadata |
| Source disabled | Explicit flag, missing credentials, failed startup probe, or unsupported worker mode |
| Source suspended | Failure threshold/cooldown; review access failure before restarting repeatedly |
| Credential file edited but source unchanged | Restart; no live reload is implemented |
| Spotify exact lookup works but text search fails | Public search and embed availability differ; review source mode and diagnostics |
| No POI | Accepted YT ID, enable flag, request budget, data coverage and duration compatibility |
| Wrong-looking disc/track position | Verify selected source edition; text search cannot establish which edition was played |
| Album list is short | Inspect `track_list.complete`, total, requested limit and provider coverage |
| Artist name yields candidates | Ambiguous identity; choose the correct source-native artist ID |
| `/v1/catalog` returns 503 | Enable worker, verify interpreter/dependencies, paths and queue capacity |
| Cache disappears on redeploy | Ephemeral filesystem; mount a volume or use shared Redis |
| Redis profile started but disk still used | Set `OMMR_CACHE_PROVIDER=redis` and `OMMR_REDIS_URL` explicitly |
| Browser fetch fails but curl succeeds | Configure same-origin proxy or gateway CORS/OPTIONS; router has no CORS middleware |
| Unexpected 404 | Route may be disabled in `api.endpoints`; gateway cannot bypass that flag |
| 429 | Local process-wide request budget; reduce concurrency and back off |
| No structured writer credits | Source may expose only raw description text or omit roles; credits are not guessed |

## Prepare a useful bug report

Include the commit, OS/Go version, operation and nonsecret query, selected sources,
HTTP status, sanitized debug provider statuses, expected recording/edition and
whether the cache was warm. Distinguish fixture tests from a live network result.
Keep cookies, tokens, private paths and full session files out of the report.

[Known limitations](../LIMITATIONS_AND_METADATA_OPTIONS.md),
[provider diagnostics](../PROVIDER_DIAGNOSTICS.md), and
[dated verification](../verification/README.md) provide additional context.
