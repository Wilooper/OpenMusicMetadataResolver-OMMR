# Linux VPS or home server

[Hosting](README.md) · [Docker](docker.md)

This recipe uses a Linux host, Docker, a domain pointing to it, and Caddy installed
on the host. Adapt firewall rules to the host: allow inbound 80/443 for the proxy;
keep OMMR's host port on loopback. This also works on a home server with appropriate
network reachability and domain configuration.

## Build and run

Clone the project, select your intended revision, then follow the Docker guide:

```sh
git clone https://github.com/Wilooper/OpenMusicMetadataResolver-OMMR.git
cd OpenMusicMetadataResolver-OMMR
docker build -t ommr:local .
docker volume create ommr_cache
docker run -d --name ommr --restart unless-stopped \
  -p 127.0.0.1:8080:8080 \
  -e OMMR_CACHE_DIR=/app/cache_data \
  -v ommr_cache:/app/cache_data ommr:local
curl --fail http://127.0.0.1:8080/v1/health
```

## Add HTTPS

Replace `music-api.example.com` in your Caddyfile:

```caddyfile
music-api.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

Validate and reload using your Caddy installation. With working DNS and reachable
ports, Caddy can manage HTTPS certificates. This proxy example provides HTTPS only;
add your chosen gateway authentication before public use of a private API. Keep
`/metrics` and detailed provider diagnostics private or disable their routes.

```sh
curl --fail https://music-api.example.com/v1/health
curl --get https://music-api.example.com/v1/resolve \
  --data-urlencode 'artist=Karan Aujla' --data-urlencode 'title=Boyfriend'
```

## Operate and update

Monitor `docker logs ommr`, memory, disk usage and provider outcomes. Build a new
image tag, stop/remove the old container, and rerun it with the same volume and
configuration. Keep the previous tag for rollback. Do not delete volumes as an
update step. Back up private configuration independently of rebuildable caches.

The restart policy handles container exits and Docker daemon restarts. It does
not guarantee upstream availability. If you prefer a native service, use a service
manager with a dedicated account, absolute binary/config paths and a writable
cache directory; [installation](../getting-started/installation.md) covers the build.

References: [Caddy reverse proxy](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy),
[automatic HTTPS](https://caddyserver.com/docs/automatic-https).
