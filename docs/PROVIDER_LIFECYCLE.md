# Startup credentials, provider health and API configuration

## One startup configuration file

Copy `ommr.example.yaml` to `ommr.yaml`, set permissions to 600, and start the server. A local `ommr.yaml` is discovered automatically. Choose another file with `./bin/ommr-server --config /absolute/path/config.yaml` or `OMMR_CONFIG_FILE`. Explicitly setting `OMMR_CONFIG_FILE` to an empty value disables automatic YAML discovery. Configuration is loaded once; restart after changing policy or credentials.

The file configures listener/cache settings, timeouts, bulk limits, metadata trust, enrichment, credential paths, source activation, health policy, endpoint availability and request defaults. Environment variables retain the `OMMR_` prefix; nested keys use underscores, for example `OMMR_API_DEFAULT_OPERATION=links`. A selected YAML file and env file must be owner-only. Select one file format where possible; process variables override YAML, YAML overrides env-file defaults. Non-empty explicit settings override credential-directory discovery; an explicitly empty process credential variable disables that imported field.

```yaml
credentials_dir: ./credentials
api:
  default_operation: resolve
  default_sources: [spotify, applemusic, ytmusic, deezer, musicbrainz]
  debug: false
  bypass_cache: false
  search_limit: 10
  allow_overrides: true
  allow_debug: false
  endpoints:
    extract: false
    metrics: false
providers:
  failure_threshold: 3
  cooldown: 1m
  probe_timeout: 8s
  startup_timeout: 45s
  probe_workers: 4
  enabled:
    tidal: false
```

With this configuration a client can send:

```http
POST /v1/query
Content-Type: application/json

{"artist":"Example Artist","title":"Example Song"}
```

No source/debug/endpoint selection is needed per request. `operation` optionally selects `resolve`, `links`, `search`, `extract`, `bulk` or `fingerprint`; omitting it uses the configured default. Resolve/links/extract/search accept their existing flat parameter names, or a nested `query` object. For bulk/fingerprint, use the existing payload with an optional `operation` field. Search still needs `q`; extraction needs provider/id or artist/title. Track inputs remain request data, not persistent defaults.

`allow_overrides=false` locks operation selection, sources, debug, cache-bypass and search-limit defaults. It does not lock artist/title/IDs or the requested search page. `allow_debug=false` suppresses diagnostics even when requested. An endpoint set to false returns 404 through both its original route and the JSON gateway. Known endpoint flags: `resolve`, `links`, `search`, `extract`, `bulk`, `fingerprint`, `health`, `providers`, `query`, `metrics`. Missing flags mean enabled. Default source lists are preferences, not an access-control allowlist; use `providers.enabled` to prohibit a source. Disabled or suspended known sources are skipped rather than contacted.

## Credential discovery and private session import

Discovery reads only these names in `credentials_dir` (default `./credentials`):

- `spotify.json`, `applemusic.json`, `ytmusic.json`
- `qobuz.json`, `tidal.json`, `amazonmusic.json`, `pandora.json`
- `lastfm.json`, `discogs.json`, `acoustid.json`

The directory must be mode 700 and files mode 600. Credential files cannot be symlinks and are limited to 1 MiB. Discovery never scans browser profiles or arbitrary folders. Relative credential paths are resolved against the selected YAML file's directory; without a YAML file they use the working directory. The default directory is git-ignored.

Sessions accept raw Cookie header files, Netscape cookie exports, browser cookie JSON arrays, or JSON objects containing `cookie`/`cookies` and optional headers. Cookie arrays/exports are filtered to the provider's domain; raw headers are operator-supplied. Credentials containing header line breaks fail config loading. File permission/schema errors also fail loading; failed remote authentication is handled by fallback/source disabling instead.

Spotify example (`credentials/spotify.json`):

```json
{"cookie":"sp_dc=YOUR_COOKIE","access_token":"CURRENT_BROWSER_BEARER","expires_at":2000000000}
```

`expires_at` is Unix seconds. Browser token responses with `accessToken` and `accessTokenExpirationTimestampMs` are supported too. Official `client_id`/`client_secret` can coexist; startup tries official credentials, then the imported browser session, then public methods. Browser sessions must pass protected `/me` and track-metadata probes. A cookie exchange returning an anonymous token is rejected. Cookies are sent only to Spotify's fixed HTTPS web-token host; bearer tokens are used on its API host. Redirects are refused.

**Spotify limitation:** the web token endpoint may require its changing web-player challenge. OMMR makes a bounded cookie exchange attempt; it does not reproduce challenge secrets. If that fails, import a current browser bearer with the cookie or use official credentials. Raw `sp_dc` alone is not guaranteed to refresh successfully. Imported bearers are not OAuth refresh tokens; without a usable cookie exchange they must be renewed manually after expiry and the server restarted.

Apple example (`credentials/applemusic.json`):

```json
{"headers":{"Authorization":"Bearer DEVELOPER_OR_BROWSER_BEARER","Cookie":"media-user-token=YOUR_USER_TOKEN"}}
```

Alternatively supply `developer_token` and `music_user_token`. The `media-user-token` cookie is extracted as a Music User Token; ordinary Apple ID cookies cannot replace the required bearer. Session mode validates `/v1/me/storefront` and catalog search. A separately usable developer/catalog token can still be selected if session validation fails. With no valid catalog mode, the source uses public iTunes methods. Apple ID passwords and arbitrary browser cookies are never sent to catalog endpoints.

YouTube Music example (`credentials/ytmusic.json`):

```json
{"cookie":"SAPISID=YOUR_VALUE; __Secure-3PAPISID=YOUR_VALUE"}
```

A `ytmusicapi` header JSON export or domain-filtered cookie array also works. OMMR calculates a fresh SAPISIDHASH for the exact YouTube HTTPS origin. Startup calls YouTube Music's account menu and requires an authenticated account header; HTTP 200 alone does not establish login. Failed authentication falls back to public InnerTube/oEmbed metadata.

Explicit alternatives: `OMMR_SPOTIFY_SESSION_FILE`, `OMMR_SPOTIFY_COOKIE_FILE`, `OMMR_SPOTIFY_COOKIE`, `OMMR_SPOTIFY_ACCESS_TOKEN`; `OMMR_APPLEMUSIC_SESSION_FILE`, `OMMR_APPLEMUSIC_COOKIE_FILE`, `OMMR_APPLEMUSIC_COOKIE`, `OMMR_APPLE_MUSIC_USER_TOKEN`, `OMMR_APPLE_DEVELOPER_TOKEN`; `OMMR_YTMUSIC_SESSION_FILE`, `OMMR_YTMUSIC_COOKIE_FILE`, `OMMR_YTMUSIC_COOKIE`.

For required-key files use:

| File | JSON fields |
| --- | --- |
| `qobuz.json` | `app_id`, optional `token` |
| `tidal.json` / `pandora.json` | `token` |
| `amazonmusic.json` | `token`, `api_key` |
| `lastfm.json` / `acoustid.json` | `api_key` |
| `discogs.json` | `token` |

## Startup activation

| Source type | No credentials | Credentials fail probe | Probe succeeds |
| --- | --- | --- | --- |
| Spotify / Apple / YouTube Music | Public metadata mode | Next supported mode, then public fallback | Register selected credentialed mode |
| TIDAL / Qobuz / Amazon / Pandora | Do not register | Do not register | Register |
| Last.fm / Discogs / AcoustID | Disabled | Disabled | Activate optional service |
| Keyless base providers / artwork / release context | Active unless disabled by config | Not applicable | Not applicable |

No supplied credentials are declared valid merely because a public fallback returned a song. Startup probes bypass response caches. TIDAL/Qobuz/Amazon probes require a successful known-track catalog search with non-empty results; this checks usable catalog capability, not every optional account scope. HTTP 200 API-error envelopes are rejected. Discogs uses its identity endpoint; Last.fm checks a known track; AcoustID uses a documented track-ID lookup to validate application access without processing audio. Pandora requires `providers.pandora_probe_id` to name a real accessible track; without one it remains unloaded even when a token is supplied. A provider configuration flag set to false prevents probes entirely.

Public modes are not live-probed on every boot; they enter the active pool and are monitored when used. Startup failure can mean inaccessible network, insufficient permissions or invalid/expired credentials; `credential_probe_failed` is deliberately not a definitive diagnosis of the secret itself. No operator sessions were available for live authenticated testing during implementation; request/response, fallback and isolation paths are fixture-tested.

## Health and recovery

`GET /v1/providers` returns safe lifecycle fields: state, selected mode, startup credential result, failure count, last evaluation and retry time. It excludes cookie/token values and account names/IDs. Disable the endpoint with `api.endpoints.providers: false` if desired. `/v1/health` reports counts and becomes `degraded` when no identity adapters are selectable; it does not promise upstream catalog availability.

After `failure_threshold` consecutive operational errors, a source is `suspended` and omitted from automatic source selection. Canceled requests and not-found responses do not count as health failures. After `cooldown`, one caller may attempt recovery; concurrent callers skip that provider. Successful recovery resets the streak. Stale requests that began before suspension cannot reopen it. `cooldown: 0s` keeps a runtime-suspended provider out until restart. Startup-disabled sources require corrected configuration/credentials and a restart; they are not periodically re-probed.

The same circuit applies to registered catalog enrichment and fingerprint lookup. A source remains a small in-memory object while suspended; it is removed from the active request pool rather than dynamically unloading Go code. Provider-health transitions and recovery eligibility partition resolved-cache keys, and responses spanning a transition are not written into the old partition. Partial provider-failure responses use at most a 30-second resolved TTL.

This service imports operator-owned sessions; it does not host a browser sign-in UI, store passwords, or reload credentials live. Runtime health is about operational metadata requests; the startup credential result is not continuous account-session verification.

## Rich public catalogs

See [RICH_CATALOGS.md](RICH_CATALOGS.md) for native Bugs/Genie/Melon song adapters, `bugs_id`/`genie_id`/`melon_id`, accepted YouTube artist/album context in provider data, optional Python pooling and `/v1/catalog` (also `operation: catalog` through the JSON gateway). These Korean adapters require no credentials; the optional Python path currently uses public YouTube access.

Capability discovery: `/v1/capabilities` summarizes registered track sources, independent artist service readiness, endpoint enablement, input support and worker dependencies. It never probes credentials or upstream hosts; local readiness is not proof of live source access. See [discovery semantics](SHARE_LINKS_AND_CAPABILITIES.md).

## Native album catalogs

`api.endpoints.albums` controls REST and JSON album requests; `api.default_operation: albums` selects the JSON default. Exact album lookups share source enable flags, request rate limits and health circuits. Their `albums:v1` cache is independent of track resolution and includes native ID, limit and storefront. See [native album metadata and track lists](NATIVE_ALBUMS.md).
