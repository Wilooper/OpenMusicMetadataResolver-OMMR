# Contribute to OMMR

[Docs](README.md) · [Repository rules](../AGENTS.md) · [Testing](guides/testing.md)

Bug reports, source fixtures, documentation and implementation improvements are
welcome. Check existing issues and describe the concrete recording/edition or
operation involved. Keep operator credentials out of examples and reports.

## Local development

Use the [installation guide](getting-started/installation.md). Go 1.26 is the tested
build path; the module declares 1.25.0 and dependencies can request newer toolchains.

```sh
git clone https://github.com/Wilooper/OpenMusicMetadataResolver-OMMR.git
cd OpenMusicMetadataResolver-OMMR
git switch -c docs/your-topic
go mod download
go build ./...
go vet ./...
go test ./...
go test -race ./...
go build -o bin/ommr-server ./cmd/server
```

Follow `AGENTS.md`. For Go code or architectural changes, consult the relevant
local `cc-skills-golang/skills/*/SKILL.md` guidance. Android race-test limitations
and worker/live checks are documented in [testing](guides/testing.md).

## Implementation rules

- Keep base resolution usable without paid keys. Credential-only integrations are
  optional and must remain unloaded until valid startup access checks pass.
- Never weaken score, artist, source-ID or recording/version gates to improve a
  live success count. Emit identity matches only for accepted candidates.
- Wire providers through `internal/bootstrap`, adapters, local rate budgets,
  identity/merge priorities, models, tests and docs as required by `AGENTS.md`.
- Context sources implement catalog enrichment and must not invent recording IDs.
- Preserve source ownership, release positions and contributor scope. Add meaningful
  deterministic fixtures for new behavior and rejection cases.
- Format Go with `gofmt`; keep source errors sanitized and credential redirects refused.
- Append architectural decisions to `ARCHITECTURE_DECISIONS.md`; do not rewrite history.

## Documentation and pull requests

Keep the root README short. Add focused pages under `docs/`, link them from the
index, and validate relative links and examples. Explain unsupported access and
partial data where users encounter them. Preserve dated reports as historical
snapshots; describe current behavior in reference guides.

Lead a PR with the concrete problem and resulting behavior, followed by relevant
validation and limitations. Do not claim authenticated live coverage from fixtures.
Use descriptive commits such as `docs: add deployment guides` or
`fix: preserve source album placement`.

## Licensing

This checkout has no root code license grant. Do not assume the old README's MIT
badge established one. See [licensing and source attribution](guides/licensing.md).
