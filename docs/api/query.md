# JSON operation gateway

[API](../API_SPEC.md) · [Configuration](../guides/configuration.md)

`POST /v1/query` dispatches the same operations as the direct endpoints. Send one
JSON object, at most 1 MiB. No SDK or separate authentication protocol is required.

```sh
curl --request POST http://localhost:8080/v1/query \
  --header 'Content-Type: application/json' \
  --data '{"operation":"resolve","artist":"Karan Aujla","title":"Boyfriend","sources":["applemusic","ytmusic"]}'
```

Supported operations: `resolve`, `links`, `search`, `extract`, `bulk`, `fingerprint`,
`catalog`, `artists`, `albums`, `capabilities`. Health, providers and metrics use
their direct routes. Omit `operation` to use configured `api.default_operation`
(default `resolve`). An operation returns the same envelope as its direct endpoint.

For non-bulk/non-fingerprint requests, inputs can be top-level or nested under
`query`; duplicate fields are rejected. `sources` accepts a string list or a
comma-separated string. Other query fields must be scalar values. Unknown fields
and trailing JSON are rejected. IDs should be strings to preserve exact values.

```json
{"operation":"albums","provider":"applemusic","id":"1853034200"}
```

```json
{"operation":"artists","query":{"artist":"Gur Sidhu","limit":5,"debug":true}}
```

For bulk/fingerprint, use their normal body fields alongside `operation`. Their
debug/presentation control follows the direct POST route's query-parameter policy.
Disabled operations stay disabled through this gateway. With
`api.allow_overrides: false`, configured operation/source/debug/cache/search
defaults take precedence over caller overrides. Debug can also be suppressed
independently with `api.allow_debug: false`.
