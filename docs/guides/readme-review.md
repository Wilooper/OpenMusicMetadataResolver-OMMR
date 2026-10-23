# README research and documentation structure

[Docs](../README.md)

Reviewed on 2026-10-22 to inform this documentation revision. These are structural
references, not endorsements, dependencies or claims that OMMR implements their features.
No third-party README prose or branding is copied into OMMR's overview.

| Reference | Useful pattern | Applied to OMMR |
| --- | --- | --- |
| [readme-best-practices](https://github.com/jehna/readme-best-practices) and the local `readme-best-practices/` guide/template | Explain purpose, installation, configuration and contribution | A short entry page with direct links for each task |
| [Navidrome](https://github.com/navidrome/navidrome/blob/master/README.md) | Clear product identity; installation and hosting separated from details | One-sentence purpose and a platform chooser |
| [beets](https://github.com/beetbox/beets/blob/master/README.rst) | Concrete metadata use case and quick example | A real curl workflow before internal implementation detail |
| [ytmusicapi](https://github.com/sigma67/ytmusicapi/blob/main/README.rst) | Requirements, scope, feature grouping and dedicated usage docs | Explicit optional-worker requirements and focused catalog references |

The local README guide is a template repository, not an installed `SKILL.md`.
The documentation follows its guidance without inventing an available skill.

## Navigation rules

- Root `README.md`: purpose, useful capabilities, a first request, limitations and links.
- `docs/README.md`: the full map for users, operators and contributors.
- `getting-started/`: installation and Android/Termux specifics.
- `guides/`: task-oriented use, interpretation, configuration, operations and testing.
- `api/`: focused contracts; `API_SPEC.md` remains the stable route index.
- `hosting/`: one page per deployment environment, with ports, storage and verification.
- Existing uppercase references: detailed metadata semantics and implementation behavior.
- `verification/README.md`: links to dated investigations and retained raw evidence.

Avoid static "build passing", release-version or code-license badges unless they
are backed by the corresponding workflow/release/license. Preserve existing deep
references where possible. When behavior changes, update its reference and any
user-facing example that depends on it; dated historical reports remain snapshots.
