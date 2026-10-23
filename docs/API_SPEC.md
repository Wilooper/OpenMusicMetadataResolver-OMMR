# OMMR API reference

[Docs](README.md) · [User guide](guides/user-guide.md) · [Schema](SCHEMA.md)

Base URL in examples: `http://localhost:8080`. The API serves JSON; request JSON
POST bodies with `Content-Type: application/json`. Use `curl --get --data-urlencode`
for titles, names and URLs. No official client SDK is bundled.

## Production and debug responses

Production is the default. Resolve returns `{"track": ...}` or `{"track": null}`.
Add `debug=true` to inspect provider outcomes, match scores and provenance. Debug
is controlled by instance policy; it is not an authentication boundary. Production
still retains accepted source metadata and useful graphs.
[Field locations, units and migration](guides/responses.md)

The debug parser accepts Go boolean syntax, including true/false and 1/0. With
request overrides enabled, empty, invalid or repeated debug values return 400.
Operator policy may replace or suppress a caller's debug value before parsing.
Production/debug share the full cached resolution. Eligible JSON supports gzip
negotiation; use `curl --compressed`.

## Endpoints

| Method | Route | Contract |
| --- | --- | --- |
| GET | `/v1/resolve` | [Unified track and accepted identities](api/resolve.md) |
| GET | `/v1/links` | [Recording links](PLATFORM_LINKS.md) |
| GET | `/v1/extract` | [Independent provider tracks](api/extract.md) |
| GET | `/v1/search` | [Lightweight discovery](api/search.md) |
| POST | `/v1/bulk` | [Bounded batch resolution](api/bulk.md) |
| POST | `/v1/query` | [JSON operation gateway](api/query.md) |
| GET | `/v1/albums` | [Native edition and track list](NATIVE_ALBUMS.md) |
| GET | `/v1/artists` | [Independent artist profiles](ARTIST_PROFILES.md) |
| GET | `/v1/catalog` | [Optional YouTube catalog worker](RICH_CATALOGS.md#go-catalog-endpoint) |
| POST | `/v1/fingerprint` | [AcoustID candidates](api/fingerprint.md) |
| GET | `/v1/capabilities` | [Configuration-aware discovery](SHARE_LINKS_AND_CAPABILITIES.md#capability-discovery) |
| GET | `/v1/providers` | [Provider selection/circuit state](api/system.md) |
| GET | `/v1/health` | [HTTP service health](api/system.md) |
| GET | `/metrics` | [Prometheus metrics](api/system.md) |

## Settings and errors

`sources` selects recording identity adapters, not enrichers such as SponsorBlock
or Last.fm. Source enable flags, API defaults, debug policy and disabled routes
apply through both direct endpoints and the gateway. See [configuration](guides/configuration.md).

| Outcome | Handling |
| --- | --- |
| 200 with `track: null` | No accepted resolution; inspect debug output |
| 400 | Invalid input, unknown source or invalid query shape; correct the request |
| 404 | Unknown or policy-disabled route; some entity lookups use it for not-found |
| 405 | Wrong method |
| 429 | Local API request budget exceeded; back off |
| 502 / 503 / 504 | Endpoint-specific upstream, readiness/capacity or timeout failure |

Resolve can return 200 with some provider failures while retaining accepted
metadata. Error behavior and envelopes differ by operation; use the relevant
contract. There is no application-wide authentication or CORS middleware.
