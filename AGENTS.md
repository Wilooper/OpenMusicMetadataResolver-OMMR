# AGENTS.md

Guidance for AI coding agents working in the OMMR (Open Music Metadata Resolver) repository.

## Project Overview

OMMR is a Go service that resolves music metadata across streaming platforms. It accepts a single platform identifier (YouTube video ID, Spotify track ID, Apple Music ID, Deezer ID, SoundCloud permalink, ISRC, or artist + title) and returns a unified canonical track record with cross-platform IDs, ISRC/ISWC, credits, artwork, and identity confidence — **zero API keys by default**.

## Commands

```bash
go build ./...              # compile all packages
go vet ./...                # static analysis
go test ./...               # full test suite
go build -o bin/server ./cmd/server   # build the server binary
```

Verify all three (`build`, `vet`, `test`) pass before finishing any change.

## Architecture (key packages)

- `cmd/server/main.go` — server bootstrap; registers provider adapters into the registry.
- `internal/adapters/` — one package per provider (`ProviderAdapter` interface in `adapters/adapter.go`). Search endpoints return up to `maxCandidatesPerProvider` candidates.
- `internal/resolver/resolver.go` — 6-stage pipeline: direct lookup → cross-resolution → scoring → **strict candidate acceptance** → merge → identity/canonical ID.
- `internal/matcher/matcher.go` — composite scoring (`0.6*JaroWinkler + 0.4*TokenSetRatio`); direct platform-ID match returns 1.0.
- `internal/merger/merger.go` — field priority lists per metadata field; deduplicates `sources`.
- `internal/identity/graph.go` — provider trust weights + `IdentityGraph`.
- `internal/models/canonical/` — immutable canonical `Track`/`Album`/`Copyright` schema.
- `internal/api/handler.go` — HTTP handlers; parse query params incl. `soundcloud_id`, `album`.
- `internal/config/config.go` — env-based config; optional `spotify_client_id`/`spotify_client_secret`.

## Critical Rules

1. **Zero-key first**: every provider must work without credentials. Key-based modes are optional upgrades only (e.g. Spotify dual-mode).
2. **Never return wrong IDs**: candidate acceptance is gated in `internal/resolver/resolver.go` — `rejectThreshold = 0.40`, `minReportableScore = 0.30`, and the **artist gate** (`minArtistScore = 0.45`) rejects title-only matches on a different artist. Do not weaken these gates.
3. **Emit `IdentityMatch` only for accepted candidates**; do not force-merge sub-threshold candidates.
4. **Provider wiring is multi-file**: adding an adapter requires (a) `cmd/server/main.go` registration, (b) a rate limit in `internal/ratelimit/provider_limiter.go`, (c) a trust weight + graph field in `internal/identity/graph.go`, (d) merger field priorities in `internal/merger/merger.go`, (e) provider model in `internal/models/provider/`, (f) tests, and (g) doc updates (`docs/PROVIDERS.md`).
5. **Public endpoint stability**: verify live before committing (e.g. Shazam's public search now returns HTTP 405; it was replaced by JioSaavn). JioSaavn mixes JSON string/number field types — handle with `provider.JSONString`.
6. **ADR discipline**: architecture decisions are appended to `ARCHITECTURE_DECISIONS.md`; never delete or rewrite existing ADRs.
7. **Code style**: no comments unless they explain non-obvious decisions; `gofmt` all Go; keep field JSON tags consistent with `docs/SCHEMA.md`.

## Docs

`README.md` and everything in `docs/` describe the current architecture — keep them in sync with code changes. `ARCHITECTURE_DECISIONS.md` records decisions append-only. `docs/CONTRIBUTING.md` documents the contribution workflow.