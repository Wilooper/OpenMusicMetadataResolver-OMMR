# OMMR · Open Music Metadata Resolver

### One song. Its identities, editions, credits, and context.

Turn a music link, provider ID, ISRC, or artist and title into a unified music
record—with accepted cross-platform links and the source metadata behind it.
Self-host the Go API on a computer, a server, or Android with Termux.

[**Get started →**](docs/getting-started/installation.md) ·
[Documentation](docs/README.md) · [API](docs/API_SPEC.md) ·
[Hosting](docs/hosting/README.md) ·
[Report an issue](https://github.com/Wilooper/OpenMusicMetadataResolver-OMMR/issues)

![Go service](https://img.shields.io/badge/backend-Go-00ADD8?logo=go&logoColor=white)
![Public metadata first](https://img.shields.io/badge/access-public%20metadata%20first-16803c)
![Optional Python worker](https://img.shields.io/badge/Python-worker%20optional-3776AB?logo=python&logoColor=white)

## The metadata layer for your music app

| Give OMMR… | Get back, where sources supply it… |
| --- | --- |
| A song link or provider ID | A canonical track and accepted platform IDs/URLs |
| Artist + title, optionally an album | Matched metadata with an explicit identity status |
| A played song's exact source ID | Its source edition, disc, and track position |
| A native album ID | An ordered track list with honest completion indicators |
| An artist name or native ID | Source-owned profiles, biographies, and audience metrics |
| An accepted YouTube video | Optional SponsorBlock highlight points for that video |

Build catalog pages, enrich a player, connect music services, or inspect conflicting
metadata. Returned fields can include artwork, previews, credits, recording/work/release
relationships, language, labels, and source-specific catalog details.

**Public metadata works without configuring keys.** Access varies by provider,
region, and query type; some integrations require validated credentials. Matching
gates can reject a candidate even when its title looks right. Missing data stays
missing. See [providers and access](docs/PROVIDERS.md).

## Run your first instance

Use a current Go toolchain; **Go 1.26 is the tested build path**. The module declares
Go 1.25.0 and dependencies can require newer toolchains.

```sh
git clone https://github.com/Wilooper/OpenMusicMetadataResolver-OMMR.git
cd OpenMusicMetadataResolver-OMMR
go build -o bin/ommr-server ./cmd/server
./bin/ommr-server
```

In another terminal:

```sh
curl --fail http://localhost:8080/v1/health
curl --compressed --get http://localhost:8080/v1/resolve \
  --data-urlencode 'artist=sufr, Arpit Bala, toorjo dey' \
  --data-urlencode 'title=bargad' \
  --data-urlencode 'sources=applemusic,ytmusic'
```

The second request returns `{"track": {...}}` when a match passes the gates, or
`{"track": null}` when none does. Upstream access can fail independently of server
health. Add `debug=true` to inspect provider outcomes.

Prefer a container? [Build and run with Docker](docs/hosting/docker.md).
Using your phone? [Follow the Termux guide](docs/getting-started/termux.md).

## Rich data, manageable responses

- **Edition-aware positions:** album, disc, and track numbers come from one accepted
  source observation. [Album placement](docs/DETAILED_PRODUCTION_METADATA.md)
- **Scoped credits:** follow contributor IDs and subject relationships across recordings,
  works, and releases. [Music graph](docs/MUSIC_GRAPH.md)
- **Source transparency:** keep accepted provider alternatives; use debug mode for match
  diagnostics and provenance. [Response guide](docs/guides/responses.md)
- **Optional YouTube depth:** a bounded Python worker adds catalog context and profiles.
  [Worker setup](docs/guides/python-worker.md)
- **Video highlights:** retain SponsorBlock point timing, votes, and attribution.
  [POI behavior and license](docs/SPONSORBLOCK.md)
- **Self-hosting controls:** disk/Redis cache, source health circuits, bulk requests,
  endpoint policy, Prometheus metrics, and lossless gzip.
  [Operations](docs/guides/operations.md)

## Find your next step

| I want to… | Start here |
| --- | --- |
| Install on Linux, macOS, Windows/WSL | [Installation](docs/getting-started/installation.md) |
| Learn the main requests | [User guide](docs/guides/user-guide.md) |
| Connect JavaScript or Python | [Client integration](docs/guides/client-integration.md) |
| Configure providers, credentials, and limits | [Configuration](docs/guides/configuration.md) |
| Deploy to a hosting platform | [Hosting chooser](docs/hosting/README.md) |
| Understand fields and missing data | [Response guide](docs/guides/responses.md) |
| Diagnose empty results or timeouts | [Troubleshooting](docs/guides/troubleshooting.md) |
| Develop or verify changes | [Contributing](docs/CONTRIBUTING.md) · [Testing](docs/guides/testing.md) |
| Browse every topic and evidence report | [Documentation index](docs/README.md) |

## Project boundaries

OMMR serves metadata. It does not stream/download songs, generate audio fingerprints,
or provide a music-player UI. Preview URLs and highlights are source data; highlight
points do not define a playable segment or align different recordings automatically.
Source availability, metadata completeness, and credentialed access are conditional.
[Known limitations](docs/LIMITATIONS_AND_METADATA_OPTIONS.md)

The checkout currently has **no root code license file**; the former MIT badge did
not establish a license grant. Upstream datasets have separate terms, including
SponsorBlock attribution and noncommercial/share-alike conditions.
[Licensing and attribution](docs/guides/licensing.md)

Contributions are welcome. Start with the [contribution guide](docs/CONTRIBUTING.md)
and [repository rules](AGENTS.md).
