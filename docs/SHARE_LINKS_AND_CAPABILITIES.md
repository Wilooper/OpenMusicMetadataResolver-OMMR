# Share-link resolution and capability discovery

Track resolution accepts a direct music link through `url`, alongside the existing typed ID and artist/title inputs. The same parser runs before provider access and cache lookup for REST resolve/links, the JSON query gateway, bulk items and direct Go resolver calls.

```sh
curl --get 'http://localhost:8080/v1/resolve' \
  --data-urlencode 'url=https://music.apple.com/us/album/p-pop-culture/1829591921?i=1829591986'
curl --get 'http://localhost:8080/v1/links' \
  --data-urlencode 'url=https://www.jiosaavn.com/song/mera-mann/ERIkRAwJf38'
```

```json
{"operation":"resolve","url":"https://youtu.be/dQw4w9WgXcQ"}
```

```json
{"queries":[{"url":"https://www.deezer.com/track/3495188951"},{"url":"https://musicbrainz.org/recording/c485675e-9c06-479b-b2dc-35a565d08558"}]}
```

The first JSON example is for `POST /v1/query`; the second is for `POST /v1/bulk`. Use `operation: links` to obtain matched platform links. Debug mode remains optional and follows the server's endpoint/override policy.

## Accepted track links

| Provider | Accepted form | Normalized input |
| --- | --- | --- |
| Spotify | `https://open.spotify.com/track/<22-character-ID>`, locale/embed variants, `spotify:track:<ID>` | `spotify_id` |
| YouTube / YouTube Music | Official watch URLs with one `v`, `youtu.be/<11-character-ID>`, shorts/live/embed video paths | `youtube_id` |
| Apple Music | Country-prefixed `/song/.../<ID>` or `/album/.../<album-ID>?i=<song-ID>` | `apple_id` (the song, never the album ID) |
| Deezer | `/track/<numeric-ID>` with optional country segment | `deezer_id` |
| JioSaavn | Official `/song/<slug>/<share-token>` page | `jiosaavn_url` (page first, existing share API fallback) |
| SoundCloud | Official `/artist/track` permalink | `soundcloud_id` |
| MusicBrainz | `https://musicbrainz.org/recording/<UUID>` | `musicbrainz_id` |
| TIDAL | Official `/track/<numeric-ID>` or `/browse/track/<numeric-ID>` | `tidal_id` |
| Qobuz | `https://open.qobuz.com/track/<numeric-ID>` or `play.qobuz.com` equivalent | `qobuz_id` |

URL parsing itself makes no HTTP request. Supported hosts and track routes are explicit; short links requiring redirect expansion, playlists, albums without a selected song, artist pages, userinfo, ports, HTTP links, malformed paths and duplicate identity query values are rejected. Known tracking parameters and fragments are discarded; only required identity values survive. Encoded path separators/dot segments are rejected. Query text is never turned into a caller-chosen fetch target.

`url` cannot be combined with explicit provider identifiers or ISRC. Artist/title/album context continues to follow existing direct-input behavior. A repeated or empty REST `url` returns HTTP 400. A malformed bulk item fails independently while valid items continue. URL-like values in legacy typed provider fields are now checked against the same host/route parser, so a foreign URL cannot masquerade as a Spotify/YouTube/etc. ID. Raw legacy ID behavior remains compatible.

New explicit resolver fields are `jiosaavn_id`, `jiosaavn_url` and `musicbrainz_id`; they also work in bulk and the JSON gateway. JioSaavn IDs remain typed internally: eight-character IDs use native `song.getDetails`, other tokens use share lookup, and a page URL always identifies a share token regardless of its length. `jiosaavn_url` cannot be combined with another provider identifier. A native-ID match can use the provider's actual `jiosaavn_track_id`; a share lookup cannot borrow that native alias as proof. MusicBrainz recording IDs are validated UUIDs and normalized to lowercase.

The resolver, matcher and seed filter share a single direct-input definition. Exact seed ownership, accepted-only identity emission, title/artist/version/ISRC gates and cross-provider matching thresholds remain in force. Related song links do not become recording identity evidence. A valid URL does not establish provider availability or global identity on its own. Include the originating provider when explicitly restricting `sources`; an unavailable/excluded seed cannot be replaced by an unrelated search result.

## Cache behavior

Tracking variants normalize to the same typed query before cache lookup and coalescing. For example, a Deezer track URL and its `deezer_id` use the same cache identity when other query/source fields agree. Debug/production sharing and bypass semantics remain unchanged. A Spotify URI and its URL normalize to the same track ID. Native versus page-first JioSaavn inputs can produce different source detail and therefore remain separate query variants; alternate JioSaavn slugs are not claimed to share a cache entry. Resolved caching now uses `rich-v14` so older entries cannot hide the added [recording and credit graph](MUSIC_GRAPH.md). Link normalization and matching gates remain unchanged.

## Capability discovery

```http
GET /v1/capabilities
```

```json
{"operation":"capabilities"}
```

Discovery performs no upstream probes and returns:

- `capabilities_version: 1` for this discovery schema.
- `api.default_operation`, `allow_debug`, `allow_overrides`, and endpoint path/method, configured `enabled`, and dependency-based `runtime_available` flags.
- Sorted provider entries containing adapter version when registered, enablement, track registration/readiness, typed ID field, track-link parser support, and currently selectable operations.
- Configured bulk/request limits plus implemented search/artist/catalog bounds. The bulk worker count reflects the handler's fallback when configured nonpositive.

`track_url_input` and `track_id_field` describe implemented input syntax, even when the provider is disabled. `operations` respects endpoint enablement, track registration, public artist-service availability, source circuits and optional worker dependencies. Native artist profiles can be available independently of a registered track adapter. TIDAL/Qobuz URL syntax support does not load an unconfigured credentialed provider. Readiness is a local snapshot, not a provider network/authentication probe or a guarantee of upstream metadata. Expired circuit cooldowns may permit a probe; callers should still handle errors. Neither yearly play counts nor unsupported catalog operations are inferred.

The response includes no credentials, headers, local paths, upstream bodies or account data. It uses `Cache-Control: no-store` because runtime health can change. The existing detailed/redacted lifecycle status remains under `/v1/providers`.

```yaml
api:
  default_operation: resolve # capabilities is also a supported default operation.
  endpoints:
    capabilities: false # Disable both GET and JSON gateway discovery.
```

Existing `allow_overrides` and endpoint policy apply to JSON operation selection. Capability discovery does not enable a disabled provider, endpoint or Python worker.

## Verification

Regression tests cover provider-owned direct identity, malformed/foreign/non-track URLs, GET/JSON/bulk consistency, error isolation, typed field validation, tracked-link cache reuse, disabled/suspended sources, worker absence, secret exclusion and discovery configuration. A real public-only server smoke run resolved the Apple Music Boyfriend share URL to ID `1829591986`, reused the cached response through the gateway, produced platform links, rejected a foreign URL, isolated that bulk failure and served discovery through both access paths. This does not claim all nine services were live-retested from this environment. See [saved smoke results](test-results/2026-10-19-share-links-smoke.json).

Build, vet, full Go tests, focused Linux race checks and all eight Python worker tests passed. [Check results](test-results/2026-10-19-share-links-checks.json).
