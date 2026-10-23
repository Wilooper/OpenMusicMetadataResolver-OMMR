# Health, capabilities, provider state and metrics

[API](../API_SPEC.md) · [Operations](../guides/operations.md)

```sh
curl http://localhost:8080/v1/health
curl http://localhost:8080/v1/capabilities
curl http://localhost:8080/v1/providers
curl http://localhost:8080/metrics
```

| Route | Use |
| --- | --- |
| `/v1/health` | Status, compiled-in version string, uptime, configured cache backend and provider-state counts |
| `/v1/capabilities` | Schema-versioned, configuration-aware endpoint/input/provider feature discovery |
| `/v1/providers` | Safe lifecycle state, selected access mode, credential presence/probe reason and circuit counters |
| `/metrics` | Prometheus exposition; outside `/v1/` and outside JSON gzip middleware |

Health can say `healthy` while individual providers fail. It reports `degraded`
when the registry contains no recording adapters, but still uses HTTP 200. Its
cache field is configured state, not proof that Redis is currently healthy. A
compiled-in version such as `1.0.0` is not proof of a matching GitHub release.

Capabilities describe readiness/support, not live success for an arbitrary song.
Provider `active` means selectable; repeated operational errors may suspend it.
Credential contents and upstream bodies are not returned. See
[startup selection and recovery](../PROVIDER_LIFECYCLE.md).

Routes can be disabled independently in `api.endpoints`; disabled routes return
404. Health/metrics requests use the same process-wide inbound limiter as other
routes, so choose monitoring frequency and RPS accordingly. Restrict metrics,
provider status and debug access at the gateway when they should remain private.
