# Contributing to OMMR

Thank you for your interest in contributing to Open Music Metadata Resolver (OMMR)! We welcome pull requests, bug reports, feature requests, and documentation improvements.

---

## Local Development Setup

### 1. Prerequisites
* **Go**: 1.22 or higher
* **Git**: Latest version
* **Docker & Docker Compose** (optional for Redis testing)

### 2. Clone & Build

```bash
git clone https://github.com/ommr/ommr.git
cd ommr

# Download Go dependencies
go mod download

# Run test suite
go test -v -cover ./...

# Build server binary
go build -o bin/ommr-server.exe ./cmd/server
```

---

## Skill Library Integration (`cc-skills-golang`)

Before making architectural or code modifications, consult the local Go skills library:

```text
cc-skills-golang/skills/
├── golang-architecture/
├── golang-api/
├── golang-testing/
├── golang-concurrency/
├── golang-security/
├── golang-performance/
└── golang-database/
```

Follow the best practices outlined in `SKILL.md` files within those directories.

---

## Pull Request Guidelines

1. **Feature Branches**: Create a descriptive branch (e.g. `git checkout -b feature/jiosaavn-adapter` or `fix/lucene-escaping`).
2. **Formatting**: Ensure your code is formatted with `gofmt` or `goimports`.
3. **Tests**: Add unit tests for new functionality in `*_test.go`. All tests must pass:
   ```bash
   go test -v -cover ./...
   ```
4. **Zero-Key Requirement**: Provider adapters **MUST NOT require** API keys or developer authentication credentials to function. A key-based mode is permitted only as an *optional* upgrade on top of a working zero-key path (e.g. Spotify's official Web API mode).
5. **Clean Commit Messages**: Use conventional commits (e.g. `feat: add Deezer genre parsing`, `fix: correct ISRC merge priority`).

---

## License
By contributing to OMMR, you agree that your contributions will be licensed under the project's [MIT License](../LICENSE).
