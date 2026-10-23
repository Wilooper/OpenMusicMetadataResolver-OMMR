# Automated documentation checks

[Docs](../README.md) · [Testing](testing.md) · [Container publishing](../hosting/container-publishing.md)

Every branch/tag push and pull request runs the main Docker pipeline. Its
documentation job calls the reusable Documentation checks workflow before images
can be published. The documentation workflow can also be started manually.

## Run locally

Use Python 3.11+ and Bash (Termux or WSL works):

~~~sh
python3 -m venv .venv-docs
.venv-docs/bin/python -m pip install -r requirements-docs.txt
.venv-docs/bin/python -m unittest discover -s scripts -p 'test_*.py' -v
.venv-docs/bin/python scripts/check_docs.py
~~~

The separate image-tag tests use Node.js's built-in test runner:

~~~sh
node --test scripts/test_image_tags.cjs
~~~

Node.js is needed for these tag tests, not for running the OMMR Go server.
On Termux, install Python/Bash with pkg and use the available nodejs-lts package
if you want to run the tag tests. The CI runner supplies Node.

## What fails the check

The checker scans root Markdown files and the docs tree. A Markdown parser
recognizes ordinary links, reference links, images, headings and HTML anchors.
It ignores links inside code examples and checks local target paths, percent-encoded
paths/fragments, duplicate-heading suffixes and repository boundaries.

Fenced JSON, YAML, TOML and Python examples are parsed or compiled. Shell examples
are checked with bash -n. Examples are never executed: an installation, deployment,
credential command or HTTP request in a guide will not run during this check.
Compose's !reset YAML tag is understood; duplicate YAML keys are rejected.
Unclosed fenced blocks fail.

Errors identify the file, line and failing target/example. Correct the destination,
heading or syntax and rerun the checker. Use a text fence for deliberately incomplete
schematic text rather than labeling invalid pseudocode as executable JSON or shell.

## Scope and limits

External HTTP URLs are not fetched by this deterministic check; network access
errors, rate limits and sign-in walls should not block unrelated commits.
A syntax check does not establish API correctness or successful platform deployment.
Go fixtures and real runtime/deployment verification remain separate.

Vendored README templates and Go skill libraries are not scanned as first-party
documentation. Linked local Markdown targets are parsed when their fragments need
validation. Dependencies are pinned in requirements-docs.txt. Reusable workflows
use read-only repository permissions; no provider keys are needed.
