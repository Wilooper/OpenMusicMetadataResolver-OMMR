# Operate, monitor and update OMMR

[Docs](../README.md) · [Hosting](../hosting/README.md) · [Cache internals](../CACHING.md)

## Before exposing an instance

OMMR listens on all interfaces and has no built-in authentication, TLS termination
or CORS middleware. Choose access controls at a reverse proxy/gateway. Keep a
host-local container port on loopback where possible. Restrict `/metrics`, debug
and provider diagnostics according to the audience. Endpoint policy is available
in YAML, but it does not authenticate callers.

Store secrets outside Git and container images. Owner-only files and startup
validation apply to configured credentials. The Dockerfile copies the build
context into its builder stage: keep private files out of that context when
building images. Never publish operator caches, sessions or unredacted logs.

## Monitor three different things

| Check | What it establishes |
| --- | --- |
| `/v1/health` | HTTP process uptime/status; not end-to-end provider health |
| `/v1/providers` | Source modes, disabled/suspended states and sanitized reasons |
| A bounded real resolve request | Whether selected sources can retrieve that recording now |
| `/metrics` and JSON request logs | Request/process observations; correlate request IDs |

Health returns HTTP 200 even with its `degraded` status. Container/platform health
checks based only on status code will not detect missing upstream coverage. Do
not make a costly live song request every few seconds as a liveness probe.

## Cache and persistence

Disk cache is simplest for one instance. A writable directory is required even
when Redis is selected because startup connection failure falls back to disk.
Persistent volumes preserve cache across redeploys; ephemeral cache is rebuildable
but incurs additional upstream work after restart. Back up configuration and
secrets separately. Caches are not the authoritative music catalog.

`bypass_cache=true` refreshes resolved lookup; it is not a universal purge of all
raw catalog layers. No cache-clear HTTP endpoint is implemented. To clear disk
cache deliberately, stop the instance and clear only its configured cache directory,
then restart. For Redis, isolate the OMMR database/namespace; do not flush a shared
database. See [layer lifetimes and keys](../CACHING.md).

## Concurrency and scale

Begin with one process and a narrow source set. Measure provider latency, accepted
results, cache hit behavior, memory, Python pool occupancy and CPU before scaling.
`rate_limit_rps` is one process-wide inbound budget, not a per-user quota. Provider
budgets, health circuits and in-flight deduplication are also local to the process.
Redis shares entries; it does not distribute those controls across replicas.

A deployment with several replicas needs an aggregate provider budget/gateway.
Higher `bulk_workers` can increase pressure without making a rate-limited source
faster. There is no demonstrated millions-of-concurrent-requests capacity claim.

## Timeouts and shutdown

The server uses 10s read, 30s write, 60s idle and 10s graceful-shutdown timeouts.
Use an operation deadline below the write timeout and a proxy/client budget that
allows the intended operation. Credential startup probes can use up to the
configured startup window. SIGINT/SIGTERM initiates shutdown and closes workers/cache.

## Upgrade and rollback

1. Record the deployed commit/image and preserve private configuration/volumes.
2. Build and check the new revision; compare API/response changes in docs.
3. Recreate/restart with the same intended cache and configuration.
4. Check HTTP health, source state, one real request and optional worker operations.
5. Roll back to the recorded image/binary if needed; preserve evidence of the failure.

Cache namespace changes intentionally make older entries unavailable. A canonical
ID derived from metadata can change when accepted metadata changes; retain native
provider IDs in application storage. Restart after rotating credentials; there is
no live session reload.
