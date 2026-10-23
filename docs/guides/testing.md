# Build, test and reproduce evidence

[Docs](../README.md) · [Contributing](../CONTRIBUTING.md)

Run from the repository root with a compatible Go toolchain:

```sh
go build ./...
go vet ./...
go test ./...
go test -race ./...
go build -o bin/ommr-server ./cmd/server
```

Build, vet and full tests are required by `AGENTS.md`. The CI workflow also runs
race tests. Race support depends on host/architecture/toolchain; the tested
Android/arm64 environment did not support it. Run that check on a supported host.

For the optional worker, install its pinned requirements and run:

```sh
.venv/bin/python -m unittest discover -s workers/ytmusic -p 'test_*.py'
```

## Live verification

Live tools require outbound network and can produce different metadata, timings
or access failures. They are not prerequisites for ordinary offline fixture tests.
Inspect a tool's `--help` before running; use isolated output outside the repository
for local experiments, and do not supply private credentials for a public-only run.

| Tool | Purpose |
| --- | --- |
| `tools/live_song_matrix.py` | Multi-song source/identity behavior |
| `tools/live_album_coverage.py` | Album disc/track coverage |
| `tools/live_production_metadata.py` | Exact placements, credits and gzip parity |
| `tools/live_sponsorblock.py` | Accepted-video POIs, absence and disabled behavior |

```sh
python3 tools/live_sponsorblock.py \
  --server bin/ommr-server --output /tmp/ommr-sponsorblock
python3 tools/live_production_metadata.py \
  --server bin/ommr-server --output /tmp/ommr-production
```

These scripts start their own isolated server/config/cache and stop it afterward.
A source access failure must stay visible; do not relabel it as "no data" or
weaken matching gates to make a live matrix pass.

## Documentation checks

Documentation validation is now automated on every branch/tag push and pull request
through the main container pipeline. It checks local links/anchors/images and
example syntax, with tests for the checker and image-tag policy. See
[run commands and scope](documentation-checks.md).

The Go checks workflow is reusable and is called by the same pipeline before
container publication. It also supports manual dispatch. See
[container build triggers and tag policy](../hosting/container-publishing.md).

Historical counts and timing/byte measurements belong to their dated evidence
report. Do not turn one live observation into a release badge or general benchmark.
[Evidence index](../verification/README.md)
