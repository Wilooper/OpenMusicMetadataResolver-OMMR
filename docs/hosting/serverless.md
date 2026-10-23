# Serverless and static hosting compatibility

[Hosting](README.md)

The current entrypoint starts a persistent `http.Server` and optional Python
subprocess pool. It is packaged as a Go executable/container.

| Platform model | Current compatibility |
| --- | --- |
| Persistent container / VM | Direct fit; configure port and writable cache |
| Vercel Go Functions | Requires a function handler adapter; not included |
| AWS Lambda and similar functions | Requires a runtime adapter and lifecycle/cache review |
| Cloudflare Workers | Current Go executable is not a Worker entrypoint |
| GitHub Pages / static sites | Cannot execute this backend |

Vercel's Go runtime expects an exported HTTP handler in an `/api` source file.
Deploying `cmd/server/main.go` unchanged does not provide that function contract.
OMMR does not currently include such an adapter or a verified Vercel deployment.

A future function adapter would need to review cold starts, provider startup
probes, operation deadlines, cache persistence, optional subprocess support and
per-instance source budgets. The current worker pool cannot be assumed portable
to every function runtime.

You can host a frontend on Vercel or static hosting and run OMMR on a container
platform, then route API calls through a suitable authenticated backend/proxy.
See [browser integration](../guides/client-integration.md#browser-and-android-access).

References: [Vercel Go runtime](https://vercel.com/docs/functions/runtimes/go),
[Vercel runtimes](https://vercel.com/docs/functions/runtimes).
