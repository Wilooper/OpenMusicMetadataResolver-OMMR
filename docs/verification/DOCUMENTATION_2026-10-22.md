# Documentation verification · 2026-10-22

[Docs](../README.md) · [Testing](../guides/testing.md)

This revision rewrites the entry README, splits the API index into focused
contracts, adds installation/client/operator guides and supplies separate hosting
recipes. Existing metadata semantics and dated provider evidence remain linked.
No runtime code or provider behavior changes in this revision.

## Validation

- `go build ./...`, `go vet ./...`, and `go test ./...` passed with Go 1.26.1
  on Linux/amd64. These checks exercised the existing full suite; no new Go tests
  were needed for documentation changes.
- Relative Markdown paths and heading fragments were checked across the README
  and first-party docs. JSON, YAML, TOML and shell examples in changed pages were
  parsed or syntax-checked. `git diff --check` passed.
- An isolated freshly built server accepted the documented private YAML startup.
  Health, capabilities, providers and gateway capability discovery returned 200;
  a missing-input resolution returned 400.
- A public exact Apple request for `1853034368` returned an accepted `bargad`
  track, confirming the source ID and response path used in the examples.

Hosting instructions were reviewed against the linked official platform docs.
Docker was not installed in this verification environment; no container build,
Compose deployment or hosted-platform deployment was performed. Termux, Windows,
Caddy and Fly CLI commands were not executed here. A syntax check does not replace
those environment-specific deployment checks.

The existing GitHub Go-checks workflow includes race testing. This documentation
revision ran build/vet/ordinary tests locally; it does not assert a new race run.
