# Configure an instance

[Docs](../README.md) · [Credentials](../CREDENTIALS_CHECKLIST.md) · [Runtime reference](../PROVIDER_LIFECYCLE.md)

## Load settings

On Unix/WSL, copy `ommr.example.yaml` to `ommr.yaml`, set mode 600, and run
`./bin/ommr-server --config ./ommr.yaml`. Without an explicit path, the server
checks `ommr.yaml` in its working directory. `OMMR_CONFIG_FILE` selects a path;
`--config` sets that selector. `OMMR_CONFIG_FILE=''` disables automatic YAML loading.

Precedence is **process environment (including explicitly empty values) → YAML →
selected env-file values → built-in defaults**. Named credential discovery supplies
missing values. `OMMR_ENV_FILE=/absolute/private/.env` enables env-file loading;
`.env` is not automatically loaded. The selected YAML/env/session files must be
regular owner-only files (`chmod 600`). Restart after changes; no live reload exists.

Top-level environment names use `OMMR_` plus uppercase key names. Nested scalar
examples: `OMMR_EXTRACTOR_ENABLED`, `OMMR_API_ALLOW_DEBUG`. Use YAML for provider
and endpoint maps; do not assume arbitrary nested map environment names are bound.

**Platform port variables are not automatically read.** Set `OMMR_PORT` explicitly
and make the host's public target port agree with it.

## Core settings

| YAML key | Environment | Default | Purpose |
| --- | --- | --- | --- |
| `port` | `OMMR_PORT` | `8080` | HTTP listener, all interfaces |
| `cache_provider` | `OMMR_CACHE_PROVIDER` | `disk` | `disk` or `redis` |
| `cache_dir` | `OMMR_CACHE_DIR` | `./cache_data` | Writable disk cache/fallback path |
| `cache_ttl` | `OMMR_CACHE_TTL` | `24h` | Base cache lifetime; shorter service caps apply |
| `redis_url` | `OMMR_REDIS_URL` | `redis://localhost:6379/0` | RESP Redis URL; HTTP REST URLs are incompatible |
| `rate_limit_rps` | `OMMR_RATE_LIMIT_RPS` | `50` | Process-wide inbound token bucket, burst twice RPS |
| `provider_timeout` | `OMMR_PROVIDER_TIMEOUT` | `10s` | HTTP operation deadline |
| `bulk_max_items` | `OMMR_BULK_MAX_ITEMS` | `100` | Batch item bound |
| `bulk_workers` | `OMMR_BULK_WORKERS` | `10` | Batch concurrency |
| `credentials_dir` | `OMMR_CREDENTIALS_DIR` | `./credentials` | Optional named credential discovery |
| `apple_storefront` | `OMMR_APPLE_STOREFRONT` | `us` | Two lowercase letters |
| `enable_enrichment` | `OMMR_ENABLE_ENRICHMENT` | `true` | Post-merge catalog services |
| `sponsorblock_timeout` | `OMMR_SPONSORBLOCK_TIMEOUT` | `2s` | Optional POI budget, 100ms–30s |
| `metadata_policy` | `OMMR_METADATA_POLICY` | consensus behavior | JSON-string field selection policy |

The HTTP server currently has a 30-second write timeout. Keep operation deadlines
below that bound. For a slow POI source, a measured starting experiment is 25s
operation / 10s SponsorBlock; this is not a latency guarantee. See [cache behavior](../CACHING.md).

## API policy

```yaml
api:
  default_operation: resolve
  default_sources: [applemusic, ytmusic]
  debug: false
  bypass_cache: false
  search_limit: 10
  allow_overrides: true
  allow_debug: true
  endpoints:
    fingerprint: false
    metrics: false
```

Unspecified endpoint flags remain enabled. `default_sources: []` selects all
currently selectable recording adapters. Artist operations use their own supported
source defaults. `search_limit` is 1–50. `allow_overrides: false` prevents callers
from overriding configured source/search/cache/debug defaults. `allow_debug: false`
suppresses debug output. Disabled operations cannot be reached through the JSON gateway.
These policies are not authentication. [Gateway](../api/query.md)

## Provider activation and health

```yaml
providers:
  failure_threshold: 3
  cooldown: 1m
  probe_timeout: 8s
  startup_timeout: 45s
  probe_workers: 4
  enabled:
    sponsorblock: true
    spotify: false
```

Unspecified providers are enabled subject to actual access. `cooldown: 0s` requires
restart after suspension. Credential-gated sources must pass startup probes.
Filtering request `sources` does not disable post-merge enrichment; use its enable
flag or `enable_enrichment: false`. Inspect `/v1/providers` after startup.

## Credentials, worker and field trust

Use [the checklist](../CREDENTIALS_CHECKLIST.md) for each source's exact variables,
named JSON fields, sessions, fallbacks and access conditions. `.env.example` lists
common environment inputs. Never put credentials in request URLs.

[Worker settings](python-worker.md) cover all `extractor.*` options.
For field priorities, use a JSON string, for example:

```yaml
metadata_policy: '{"mode":"consensus","priorities":{"album":["applemusic","musicbrainz"]}}'
```

Configure `album` to select album and position together. Separate `track_number`
or `disc_number` priorities are rejected. [Selection policy](../ENRICHMENT.md#field-selection)
