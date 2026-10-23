# Host on Render

[Hosting](README.md) · [Troubleshooting](../guides/troubleshooting.md)

Use a **Web Service**, connected to the repository and branch you intend to deploy.
The repository Dockerfile provides a consistent Go toolchain. This is a manual
recipe; no Render Blueprint is included or required.

## Service settings

| Setting | Value |
| --- | --- |
| Runtime | Docker |
| Dockerfile | `./Dockerfile` |
| Root directory | Repository root |
| Health check path | `/v1/health` |
| `PORT` | `10000` |
| `OMMR_PORT` | `10000` |
| `OMMR_CACHE_PROVIDER` | `disk` |
| `OMMR_CACHE_DIR` | `/app/cache_data` |

Leave the Docker command override empty to use the image entrypoint. OMMR binds
all interfaces. Explicitly matching both port values avoids relying on the app
reading Render's `PORT`, which it does not do.

## Cache and secrets

The container filesystem is ephemeral. For rebuildable cache-only operation,
losing it on redeploy is acceptable but increases cold upstream work. For durable
local caching, attach a persistent disk at `/var/data` when your service plan
supports it, and set `OMMR_CACHE_DIR=/var/data/ommr-cache`. Alternatively configure
a compatible Redis connection and keep a writable disk fallback directory.

Store provider secrets in Render environment settings, not Git. Use environment
configuration first; secret-file mounts need the owner-only permissions required
by OMMR. Persistent disks have service/replica/deploy restrictions; check the
platform's current disk documentation before choosing a scaling strategy.

## Verify and update

After deployment, open the service URL's `/v1/health`, then inspect
`/v1/providers` and try a resolution with `debug=true`. A green health check can
coexist with inaccessible providers. A 502 from the platform often means the
process exited, the port is wrong, or startup has not finished; inspect build and
runtime logs. Credential probes can take part of the startup window.

Deploy the intended commit when updating. For optional Python, the default image
is insufficient; deploy an image built from the `with-python` target and enable
the extractor settings. Do not assume a free service, persistent disk or
always-running process is available on every plan.

References: [Web Services](https://render.com/docs/web-services),
[Docker](https://render.com/docs/docker),
[filesystem/deploy behavior](https://render.com/docs/deploys),
[persistent disks](https://render.com/docs/disks).
