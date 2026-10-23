# Enable the optional YouTube worker

[Docs](../README.md) · [Catalog reference](../RICH_CATALOGS.md)

The Go service handles resolution without Python. Enable the pinned `ytmusicapi`
worker for richer YouTube song/album/artist/playlist/credit context. This worker
currently uses public mode; configuring a Go adapter cookie does not authenticate it.

## Native installation

From the repository root, with Python 3.10+ and venv support installed:

```sh
python3 -m venv .venv
.venv/bin/python -m pip install -r workers/ytmusic/requirements.txt
```

Add to private YAML, replacing both absolute paths:

```yaml
extractor:
  enabled: true
  python: /absolute/path/to/ommr/.venv/bin/python
  script: /absolute/path/to/ommr/workers/ytmusic/worker.py
  workers: 2
  timeout: 5s
  ttl: 6h
  profiles: true
```

Workers are bounded to 1–8, with a queue of `workers * 4`. Each job timeout must
be positive and no more than 30s; TTL must be positive. Environment equivalents
are `OMMR_EXTRACTOR_ENABLED`, `_PYTHON`, `_SCRIPT`, `_WORKERS`, `_TIMEOUT`, `_TTL`
and `_PROFILES`, each with the same `OMMR_EXTRACTOR` prefix. Start with one worker
on a phone and inspect memory/latency before increasing concurrency.

## Docker

```sh
docker build --target with-python -t ommr:python .
docker run --rm -p 127.0.0.1:8080:8080 \
  -e OMMR_EXTRACTOR_ENABLED=true \
  -e OMMR_EXTRACTOR_SCRIPT=/app/workers/ytmusic/worker.py \
  ommr:python
```

The target sets the Python interpreter path. The default Docker target is Go-only.

## Verify

```sh
curl http://localhost:8080/v1/providers
curl 'http://localhost:8080/v1/catalog?provider=ytmusic&type=song&id=BBM57gr9fFU'
```

`ytmusic_python` reports worker health. A successful startup validates local
capabilities, not all remote catalog requests. The catalog endpoint accepts
`song`, `credits`, `album`, `artist`, `artist_albums`, `artist_singles` and `playlist`
with validated native ID forms. See [all inputs and error codes](../RICH_CATALOGS.md#go-catalog-endpoint).
Queue saturation returns 503. Partial upstream context stays partial. Catalog
references never become accepted recording IDs merely because a worker returned them.
