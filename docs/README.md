# OMMR documentation

[Project overview](../README.md) · [API index](API_SPEC.md) · [Hosting](hosting/README.md)

Start with installation, send a request, then choose the reference for your task.
Examples use `http://localhost:8080`; replace it with your instance URL.

## Start here

| Topic | Guide |
| --- | --- |
| Linux, macOS, Windows and WSL | [Installation](getting-started/installation.md) |
| Android | [Termux](getting-started/termux.md) |
| Main workflows | [User guide](guides/user-guide.md) |
| Settings and precedence | [Configuration](guides/configuration.md) |
| Response fields and interpretation | [Responses](guides/responses.md) |
| JavaScript, Python and browser access | [Client integration](guides/client-integration.md) |
| Optional YouTube catalog worker | [Python worker](guides/python-worker.md) |
| Common failures | [Troubleshooting](guides/troubleshooting.md) |

## Host and operate

[Choose a platform](hosting/README.md) · [Docker/Compose](hosting/docker.md) ·
[VPS + HTTPS](hosting/vps.md) · [Render](hosting/render.md) ·
[Railway](hosting/railway.md) · [Fly.io](hosting/fly.md) ·
[Serverless compatibility](hosting/serverless.md)

- [Operations, monitoring, scaling and updates](guides/operations.md)
- [Disk/Redis cache implementation](CACHING.md)
- [Credentials checklist](CREDENTIALS_CHECKLIST.md)
- [Startup activation, session import and circuits](PROVIDER_LIFECYCLE.md)
- [Licensing and attribution](guides/licensing.md)

## API and metadata references

| Topic | Reference |
| --- | --- |
| All routes and response modes | [API index](API_SPEC.md) |
| Track resolution and links | [Resolve](api/resolve.md) · [Platform links](PLATFORM_LINKS.md) |
| Independent provider extraction | [Extract](api/extract.md) |
| Lightweight search | [Search](api/search.md) |
| Batches | [Bulk](api/bulk.md) |
| JSON gateway | [Query](api/query.md) |
| Fingerprint candidates | [Fingerprint](api/fingerprint.md) |
| Health, provider status, metrics | [System routes](api/system.md) |
| Share URLs and discovery | [Share links and capabilities](SHARE_LINKS_AND_CAPABILITIES.md) |
| Native album metadata and complete/partial lists | [Native albums](NATIVE_ALBUMS.md) |
| Artist biographies, counters and ambiguity | [Artist profiles](ARTIST_PROFILES.md) |
| YouTube album, artist, song, playlist and credits context | [Rich catalogs](RICH_CATALOGS.md) |
| Data types and extension locations | [Schema](SCHEMA.md) |
| Field coverage | [Metadata catalog](METADATA_CATALOG.md) |
| Providers and access modes | [Providers](PROVIDERS.md) |
| Field selection and optional services | [Enrichment](ENRICHMENT.md) |
| Recording/work/release relations and credits | [Music graph](MUSIC_GRAPH.md) |
| Exact disc/track placement, credit index, gzip | [Detailed production metadata](DETAILED_PRODUCTION_METADATA.md) |
| YouTube highlight points | [SponsorBlock](SPONSORBLOCK.md) |

## Develop and investigate

- [Contributing](CONTRIBUTING.md) · [Testing and reproducible evidence](guides/testing.md)
- [Architecture](ARCHITECTURE.md) · [Architecture decisions](../ARCHITECTURE_DECISIONS.md)
- [Matching engine](MATCHING_ENGINE.md) · [Canonical identity](IDENTITY_ENGINE.md)
- [Limitations and metadata options](LIMITATIONS_AND_METADATA_OPTIONS.md) · [FAQ](FAQ.md)
- [HTML recovery](HTML_RECOVERY.md) · [JioSaavn public pages](JIOSAAVN_PUBLIC_PAGES.md)
- [README research and documentation design](guides/readme-review.md)
- [Dated verification and source investigations](verification/README.md)

Dated reports are snapshots, not current uptime guarantees. The references above
cover the implementation; use `/v1/capabilities` and `/v1/providers` to inspect
configuration and source state on the instance you are actually using.
