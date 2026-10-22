# Run on Android with Termux

[Docs](../README.md) · [User guide](../guides/user-guide.md)

Use native Termux packages and build on the phone. Ordinary Linux binaries and
Docker instructions are not a native Android installation method.

## Install and build

```sh
pkg update
pkg upgrade
pkg install git golang curl
git clone https://github.com/Wilooper/OpenMusicMetadataResolver-OMMR.git
cd OpenMusicMetadataResolver-OMMR
go version
go build -o bin/ommr-server ./cmd/server
cp ommr.example.yaml ommr.yaml
chmod 600 ommr.yaml
./bin/ommr-server --config ./ommr.yaml
```

Keep the repository, config and cache in Termux's private home directory. Shared
Android storage can lack executable and permission semantics needed by binaries
and private files. If Go is too old, update packages; the tested desktop/container
build uses Go 1.26.

In a second Termux session:

```sh
curl --fail http://127.0.0.1:8080/v1/health
curl --compressed --get http://127.0.0.1:8080/v1/resolve \
  --data-urlencode 'artist=sufr, Arpit Bala, toorjo dey' \
  --data-urlencode 'title=bargad' \
  --data-urlencode 'sources=applemusic,ytmusic' \
  --data-urlencode 'debug=true'
```

## Keep resource use predictable

Start with Go-only operation and disk cache. In private YAML:

```yaml
api:
  default_sources: [applemusic, ytmusic]
extractor:
  enabled: false
bulk_workers: 2
```

Source filtering limits request fan-out; provider enable flags control activation.
Android may stop background processes. For a longer session, allow Termux to run
in the background and optionally use `termux-wake-lock`; release it with
`termux-wake-unlock`. A wake lock uses battery and is not an uptime guarantee.
Use a managed host for an API that needs continuous reachability.

## Optional Python and verification

Install `python` with `pkg install python`, then follow the
[worker guide](../guides/python-worker.md). Begin with one worker if memory is tight.
Provider access can differ between phone and datacenter networks. Inspect the
actual response before assuming a parser or authentication defect.

```sh
go build ./...
go vet ./...
go test ./...
```

The race detector was unavailable on the tested Android/arm64 environment; use a
supported Linux/desktop host for race checks. [Termux verification](../TERMUX_PROVIDER_VERIFICATION.md)
is dated network evidence, not a promise that every provider works today.
