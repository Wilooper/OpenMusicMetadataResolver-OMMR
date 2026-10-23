# Install OMMR

[Docs](../README.md) · [Termux](termux.md) · [Docker](../hosting/docker.md)

## Requirements

- Git and a current Go toolchain. Go 1.26 is used by the Dockerfile and verified
  development build. `go.mod` declares 1.25.0; dependencies may trigger automatic
  toolchain download. Check `go version` if a build fails.
- Outbound HTTPS and DNS access to enabled providers.
- A writable cache directory. Redis and Python are optional.

## Linux, macOS or WSL

Install Go using the [official instructions](https://go.dev/doc/install), then run:

```sh
git clone https://github.com/Wilooper/OpenMusicMetadataResolver-OMMR.git
cd OpenMusicMetadataResolver-OMMR
go mod download
go build -o bin/ommr-server ./cmd/server
./bin/ommr-server
```

For a feature branch, select it before building. A default branch or published
container does not necessarily include unmerged features. The server listens on
port 8080 on all interfaces. Stop it with Ctrl+C.

## Windows PowerShell

Install Git and Go, then:

```powershell
git clone https://github.com/Wilooper/OpenMusicMetadataResolver-OMMR.git
Set-Location OpenMusicMetadataResolver-OMMR
go build -o bin/ommr-server.exe ./cmd/server
$env:OMMR_CACHE_DIR = "$PWD/cache_data"
.\bin\ommr-server.exe
```

Use `curl.exe` for shell examples; PowerShell may alias `curl`. The server checks
Unix-style private-file permissions for YAML, env files and sessions. Use WSL or
Docker with owner-only files for file-based secrets/configuration. Native Windows
users can start with process environment variables and public providers instead
of assuming `chmod` has a Windows equivalent.

## Verify from a second terminal

```sh
curl --fail http://localhost:8080/v1/health
curl --fail http://localhost:8080/v1/capabilities
curl --compressed --get http://localhost:8080/v1/resolve \
  --data-urlencode 'artist=Karan Aujla' \
  --data-urlencode 'title=Boyfriend' \
  --data-urlencode 'sources=applemusic,ytmusic'
```

Health proves the HTTP service responds; it does not prove every upstream works.
`track: null` means no candidate passed. Retry with `debug=true` to distinguish
no results, access errors and identity rejection.

## Save configuration

From the repository root on Unix/WSL:

```sh
cp ommr.example.yaml ommr.yaml
chmod 600 ommr.yaml
./bin/ommr-server --config ./ommr.yaml
```

Edit what you need and restart after changes. Keep the working directory stable
or use absolute cache/worker paths. Continue to [configuration](../guides/configuration.md)
and the [user guide](../guides/user-guide.md). Public deployment needs the setup in
[hosting](../hosting/README.md).
