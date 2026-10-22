# Docker and Compose

[Hosting](README.md) · [Operations](../guides/operations.md)

Run commands from the repository root. Docker Engine and the Compose plugin are
required. The default image contains the Go service; Python is an explicit target.

## Build the checked-out source

```sh
docker build -t ommr:local .
docker volume create ommr_cache
docker run -d --name ommr --restart unless-stopped \
  -p 127.0.0.1:8080:8080 \
  -e OMMR_PORT=8080 \
  -e OMMR_CACHE_PROVIDER=disk \
  -e OMMR_CACHE_DIR=/app/cache_data \
  -v ommr_cache:/app/cache_data \
  ommr:local
curl --fail http://localhost:8080/v1/health
docker logs --tail 50 ommr
```

Binding host loopback is suitable for local use or a host reverse proxy. Bind a
public interface only with the access policy you intend. To stop/remove the
container, use `docker stop ommr` and `docker rm ommr`; the named cache volume remains.

## Existing Compose file

```sh
docker compose up -d --build ommr-server
docker compose logs --tail 50 ommr-server
```

`--build` builds the current checkout. Compose publishes port 8080 on all host
interfaces; adjust the mapping to `127.0.0.1:8080:8080` for a host-only proxy.
The file defaults to **disk cache**. Starting the Redis profile alone does not
switch OMMR to Redis. Create `compose.redis.yaml` in the root:

```yaml
services:
  ommr-server:
    environment:
      OMMR_CACHE_PROVIDER: redis
      OMMR_REDIS_URL: redis://redis:6379/0
  redis:
    ports: !reset []
```

`!reset` requires a Compose version supporting that override tag. It removes the
base Redis host-port publication; otherwise edit the base Redis `ports` mapping
out before production use. Start both services:

```sh
docker compose -f docker-compose.yml -f compose.redis.yaml --profile redis up -d --build
```

Redis initialization failure falls back to disk and logs a warning; keep the disk
path writable. See [cache semantics](../CACHING.md).

## Mount private configuration

```sh
cp ommr.example.yaml ommr.yaml
chmod 600 ommr.yaml
docker run --rm -p 127.0.0.1:8080:8080 \
  -v "$PWD/ommr.yaml:/app/ommr.yaml:ro" \
  -v ommr_cache:/app/cache_data \
  ommr:local --config /app/ommr.yaml
```

Set paths inside the config to container paths. Mount credential files separately
at those paths, retaining owner-only permissions; do not copy them into an image.
Build `--target with-python` and follow [worker setup](../guides/python-worker.md)
for optional catalog extraction. To update, rebuild and recreate the container
from a known revision while preserving volumes and private configuration.

Reference: [Compose override/reset semantics](https://docs.docker.com/reference/compose-file/merge/).
