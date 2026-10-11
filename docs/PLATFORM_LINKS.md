# Cross-platform recording IDs and links

`GET /v1/links` uses the same identity gates as `/v1/resolve` and returns native
platform identifiers with song URLs. It never substitutes a title-search URL
or an unverified candidate's ID for a missing platform.

```text
GET /v1/links?youtube_id=x-KOXck57lc
GET /v1/links?spotify_id=<track-id>
GET /v1/links?artist=<artist>&title=<title>
GET /v1/links?isrc=<isrc>&sources=spotify,applemusic,deezer,musicbrainz
```

All provider-ID arguments, `sources`, and `bypass_cache` work as in `/v1/resolve`.
YouTube and Spotify URL inputs work through their existing parsers. Other ID
arguments require the provider lookup ID. Responses include title, artists,
identity status, provider diagnostics, and `links`:

| JSON field | Meaning |
| --- | --- |
| `links.ommr_id` | Stable `ommr_track_...` recording ID; omitted when unresolved. |
| `links.isrc` | Reported recording ISRC; omitted when unknown. |
| `links.platforms` | Map from platform to accepted ID/link. |
| `links.unavailable_platforms` | Supported platforms without a usable accepted ID. |
| `platforms.<name>.id` | Native ID, ASIN, MBID, token, or permalink. |
| `platforms.<name>.id_type` | Identifier namespace; do not assume every ID is numeric. |
| `platforms.<name>.lookup_id` | OMMR's lookup permalink/token when it differs from the native ID. |
| `platforms.<name>.url` | Optional song/recording URL; never a generic search page. |
| `platforms.<name>.url_source` | `provider` or `id_template`. |
| `platforms.<name>.confidence`, `.match_method` | Accepted-match evidence; `direct_id` means the exact requested provider ID. |

The immutable Track schema is unchanged. `/v1/resolve` and `/v1/bulk` include
the catalog at `track.extensions.platform_links`. `/v1/extract` includes `links`
in each per-provider result, scoped to that source, without asserting a
cross-provider match. Its OMMR ID is absent until resolution assigns one.

## Supported platforms

| Platform key | ID type | URL policy |
| --- | --- | --- |
| `ytmusic` | Video ID | `https://music.youtube.com/watch?v=<id>` |
| `youtube` | Same video ID | `https://www.youtube.com/watch?v=<id>`; alias, not another adapter. |
| `spotify` | Track ID (22 characters) | `https://open.spotify.com/track/<id>` |
| `applemusic` | Numeric song ID | Prefer provider storefront URL; fallback `https://music.apple.com/us/song/<id>`. |
| `deezer` | Numeric track ID | `https://www.deezer.com/track/<id>` |
| `qobuz` | Numeric track ID | `https://open.qobuz.com/track/<id>` |
| `tidal` | Numeric track ID | `https://tidal.com/track/<id>` |
| `amazonmusic` | Track ID / ASIN | Prefer provider URL; ASIN fallback `https://music.amazon.com/?trackAsin=<id>`. Other IDs may have no URL. |
| `pandora` | Music token (`TR:...`/`TR...`) or numeric track ID | Matching provider URL when supplied; otherwise ID only. |
| `soundcloud` | Numeric track ID from oEmbed widget, otherwise permalink | `https://soundcloud.com/<artist>/<track>`; retain lookup permalink separately. |
| `jiosaavn` | API song ID, otherwise share token | Returned song URL; retain lookup token separately. |
| `musicbrainz` | Recording MBID | `https://musicbrainz.org/recording/<id>`; metadata reference. |

Support does not guarantee every song is available. Qobuz, TIDAL, Amazon and
Pandora retain their existing access requirements. SoundCloud and Pandora
currently have no text search in their adapters; direct lookup can expose
their own IDs/links. ISRC and OMMR IDs are recording identifiers, not streaming URLs.

## Accuracy and URL handling

Accepted candidates contribute only their own provider's IDs. `_url` and
`_track_id` helpers do not count as extra identity matches. Provider URLs must
use a supported HTTPS host and identify the same recording. Apple album links
must select the song with `i`; Amazon album links must select it with `trackAsin`.
Recognized tracking parameters are removed. Userinfo, unexpected query fields,
fragments and mismatched recording URLs are rejected.

ID-derived URLs are format-valid routes; they do not claim verified playback
rights or a fetched landing page. Catalog availability can vary by territory.
Source errors, secondary-search latency and match evidence remain visible.

Fixtures cover every supported ID/link shape, foreign hosts, mismatched IDs,
credential-query rejection, native widget IDs, provider ownership, caching,
and no-match results. A public smoke test for `x-KOXck57lc` returned YouTube and
YouTube Music links with an OMMR ID; other services were unavailable in that run.
This is not a live equivalence claim for all catalogs.

References: [Spotify](https://developer.spotify.com/documentation/web-api/reference/get-track),
[Apple sharing URL](https://developer.apple.com/documentation/applemusicapi/songs/attributes-data.dictionary),
[Amazon track schema](https://developer.amazon.com/docs/music/API_web_track_v2.html),
[SoundCloud oEmbed](https://developers.soundcloud.com/docs/oembed),
[TIDAL sharing](https://support.tidal.com/hc/en-us/articles/23553629074193-Sharing-music-links-across-streaming-platforms),
[Qobuz track route](https://open.qobuz.com/track/197320134).
