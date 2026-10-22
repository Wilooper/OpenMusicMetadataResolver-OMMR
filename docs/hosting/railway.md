# Host on Railway

[Hosting](README.md) · [Configuration](../guides/configuration.md)

Create a project/service from the repository, select the intended branch and keep
the service root at the repository root. Railway detects the Dockerfile. This
recipe does not require a platform template.

## Configure the listener

| Setting | Value |
| --- | --- |
| Build | Repository `Dockerfile` |
| `PORT` | `8080` |
| `OMMR_PORT` | `8080` |
| `OMMR_CACHE_PROVIDER` | `disk` |
| `OMMR_CACHE_DIR` | `/app/cache_data` |
| Health check path | `/v1/health` |
| Public domain target port | `8080` |

Generate a public domain in Networking. OMMR reads `OMMR_PORT`, so explicitly
match it to the target port rather than relying on an automatically injected
`PORT`. Leave the image entrypoint intact.

## Choose storage

Container-local files are ephemeral. Either accept disposable cache, attach a
volume at `/data` and set `OMMR_CACHE_DIR=/data/ommr-cache`, or use shared Redis:

```text
OMMR_CACHE_PROVIDER=redis
OMMR_REDIS_URL=<private RESP Redis connection URL>
OMMR_CACHE_DIR=/app/cache_data
```

The angle-bracket value is a placeholder; select the actual Redis connection
variable from your service. Use a private connection where available. Do not use
a Redis HTTP REST endpoint. Keep the disk fallback writable. Volume-backed
services have scaling restrictions; do not assume one volume can serve replicas.

## Verify and update

Deploy, inspect build/runtime logs, then call the public `/v1/health` and a real
resolve request. Review `/v1/providers` for access failures. Keep provider keys in
service secrets/variables and restart/redeploy to apply changes. Restrict access
through your gateway policy before sharing the URL as a private service.

Default Docker builds exclude Python. Use a `with-python` image plus extractor
configuration if you need `/v1/catalog`. Redeploy a pinned revision for updates;
check storage and source behavior after the restart.

References: [Dockerfile builds](https://docs.railway.com/builds/dockerfiles),
[public networking](https://docs.railway.com/networking/public-networking),
[volumes](https://docs.railway.com/volumes),
[service filesystem](https://docs.railway.com/services).
