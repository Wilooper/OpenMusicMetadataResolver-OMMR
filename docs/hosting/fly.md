# Host on Fly.io

[Hosting](README.md) · [Operations](../guides/operations.md)

Install and authenticate `flyctl`, then run from the repository root:

```sh
fly launch --no-deploy
```

Choose a unique app name and region. Review the generated `fly.toml`; preserve
its actual app/region values and configure these sections:

```toml
[build]
  dockerfile = "Dockerfile"

[env]
  OMMR_PORT = "8080"
  OMMR_CACHE_PROVIDER = "disk"
  OMMR_CACHE_DIR = "/app/cache_data"

[http_service]
  internal_port = 8080
  force_https = true
  auto_stop_machines = "off"
  auto_start_machines = true
  min_machines_running = 1

[[http_service.checks]]
  grace_period = "60s"
  interval = "30s"
  method = "GET"
  timeout = "5s"
  path = "/v1/health"
```

This chooses an always-running instance, which affects cost. Review current
platform billing and machine settings. No fixed memory/throughput guarantee is
made; inspect the Go process and optional Python pool before sizing it.

## Persistence options

The root filesystem is ephemeral. For a single-Machine disk cache, create a volume
in the same region, replacing `YOUR_REGION`:

```sh
fly volumes create ommr_cache --region YOUR_REGION --size 1
```

Add a mount and change `OMMR_CACHE_DIR` to `/data/ommr-cache`:

```toml
[[mounts]]
  source = "ommr_cache"
  destination = "/data"
```

Volumes are local to a Machine; they are not a shared replicated filesystem.
Use a compatible shared Redis service if that is the cache model you need.

## Deploy and verify

```sh
fly deploy
fly status
fly logs
```

Check the assigned HTTPS URL's `/v1/health`, `/v1/providers` and a real song.
Set provider secrets using Fly's secret mechanism rather than placing them in
`fly.toml`. Restart/redeploy after configuration changes. Default Docker builds
are Go-only; Python needs a suitable image target and explicit extractor settings.
For additional Machines, account for independent rate budgets, health and workers.

References: [app configuration](https://fly.io/docs/reference/configuration/),
[volumes](https://fly.io/docs/volumes/overview/),
[secrets](https://fly.io/docs/apps/secrets/).
