# Choose a hosting platform

[Docs](../README.md) · [Configuration](../guides/configuration.md)

OMMR is a long-running Go HTTP service. A native process or container is the most
direct deployment. These are deployment recipes, not claims that an instance was
created or validated on each platform. Platform instructions were reviewed against
official documentation on 2026-10-22; plan availability and dashboard labels can change.

| Environment | Guide | Cache and runtime considerations |
| --- | --- | --- |
| Local computer / WSL | [Install](../getting-started/installation.md) | Writable local disk; stop with Ctrl+C |
| Android | [Termux](../getting-started/termux.md) | Build natively; Android can suspend the process |
| Docker / Compose | [Docker](docker.md) | Named cache volume; optional Redis/Python target |
| Linux VPS / home server | [VPS](vps.md) | Container + restart policy + HTTPS proxy |
| Render | [Render](render.md) | Explicit OMMR port; ephemeral disk unless mounted |
| Railway | [Railway](railway.md) | Match target port; volume or shared Redis |
| Fly.io | [Fly.io](fly.md) | Explicit internal port; local volume per Machine |
| Vercel / static hosting / function runtimes | [Compatibility](serverless.md) | Requires an adapter; current executable is not a function |

## Common setup

1. Deploy the branch/commit containing the features you need. Build the repository
   Dockerfile instead of assuming a registry `latest` tag includes that branch.
2. Set **`OMMR_PORT`**, a writable `OMMR_CACHE_DIR`, and the desired source defaults.
   Platform `PORT` variables are not read automatically.
3. Start without credentials. Check `/v1/health`, `/v1/capabilities`, `/v1/providers`
   and one real resolution; a healthy HTTP server is not proof of upstream access.
4. Add secrets through the host's private configuration. Credentialed modes require
   successful startup probes. A mounted YAML/session file must be owner-only.
5. Add HTTPS and gateway access controls before exposing a private instance publicly.
   The application has no built-in authentication, TLS termination or CORS layer.

Start with one replica. Disk caches, source health, rate budgets, worker pools and
request coalescing are local to a process. Redis shares cached data, not a global
upstream quota or distributed job lock. See [operations](../guides/operations.md).

No guide promises a free tier, a throughput figure or successful provider access
from a particular datacenter. Measure from the deployed network.
