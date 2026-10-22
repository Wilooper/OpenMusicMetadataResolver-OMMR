# Resolve a batch

[API](../API_SPEC.md)

`POST /v1/bulk` accepts one JSON object with independent `queries` and optional
batch `sources`. Default maximum: 100 items; worker pool: 10. Operators can tune
these bounds. The request body is limited to 1 MiB and one JSON object.

```sh
curl --request POST http://localhost:8080/v1/bulk \
  --header 'Content-Type: application/json' \
  --data '{"queries":[{"apple_id":"1853034368"},{"youtube_id":"BBM57gr9fFU"}],"sources":["applemusic","ytmusic"]}'
```

Items use the same recording input fields as [resolve](resolve.md), including
supported share `url` inputs. Do not put unrelated IDs in the same item. Item
errors remain independent and `query_index` identifies original input order. Production
returns item success/error and public track, plus `total`, `successful` and `failed`.
Debug adds full track diagnostics and item provider status (`sources`).

Use `/v1/bulk?debug=true` for detailed output; the debug flag is a query parameter
for this POST route. API defaults and endpoint policy still apply.
[JSON gateway](query.md) can dispatch the same body with `operation: bulk`.

Batching controls concurrency; it does not bypass per-provider request budgets
or accelerate upstream quotas. Identical cache-enabled resolutions can share
in-flight work within one process. Split larger queues into bounded batches and
back off after throttling instead of starting unlimited batches simultaneously.
