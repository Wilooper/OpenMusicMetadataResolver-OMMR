# Resolve tracks and accepted links

[API](../API_SPEC.md) · [Responses](../guides/responses.md)

`GET /v1/resolve` accepts one known recording reference, or both artist and title.
Use a source's actual track ID/link when edition identity matters.

```sh
curl --compressed --get http://localhost:8080/v1/resolve \
  --data-urlencode 'artist=sufr, Arpit Bala, toorjo dey' \
  --data-urlencode 'title=bargad' \
  --data-urlencode 'sources=applemusic,ytmusic'
curl 'http://localhost:8080/v1/resolve?youtube_id=BBM57gr9fFU&sources=ytmusic&debug=true'
```

## Inputs

| Input | Meaning |
| --- | --- |
| `url` | Supported direct music track URL; [formats](../SHARE_LINKS_AND_CAPABILITIES.md) |
| `artist`, `title` | Both required for text resolution |
| `album` | Optional title disambiguation |
| `isrc` | Recording code |
| `spotify_id`, `youtube_id`, `apple_id`, `deezer_id` | Corresponding native track/video ID or supported track URL |
| `soundcloud_id` | Public track permalink |
| `jiosaavn_id`, `jiosaavn_url` | Native/share ID or supported public song page |
| `musicbrainz_id` | Recording UUID |
| `qobuz_id`, `tidal_id` | Native track IDs |
| `amazonmusic_id` (alias `amazon_music_id`) | Amazon track ID |
| `pandora_id` | Pandora track entity ID |
| `bugs_id`, `genie_id`, `melon_id` | Numeric song IDs |
| `sources` | Comma-separated recording adapter filter |
| `debug` | Opt-in diagnostics, subject to operator policy |
| `bypass_cache` | Refresh resolved lookup; [layer-specific behavior](../CACHING.md) |

Use one entry reference per request rather than supplying unrelated provider IDs.
Supported adapter names are listed in [providers](../PROVIDERS.md). Credentialed
sources are selectable only when startup access checks succeed. Unknown source
names return 400; no selectable adapters can yield 503.

## Response

An abbreviated illustrative response (omitted fields are not a full schema):

```json
{
  "track": {
    "title": "bargad",
    "duration_ms": 175385,
    "disc_number": 1,
    "track_number": 1,
    "ids": {"applemusic": "1853034368"}
  }
}
```

Exact metadata varies with source observations. No accepted candidate returns
`{"track": null}`. Candidate rejections and provider access failures remain visible
in debug `provider_status`; post-merge outcomes are in `enrichment_status`.
Accepted candidates alone contribute unified IDs and provider records.

`GET /v1/links` accepts the same resolution inputs and returns a `links` catalog
plus identity/display fields. It is useful when the client needs share links;
it still performs matching. [Link schema and namespaces](../PLATFORM_LINKS.md)

Related references: [edition placement](../DETAILED_PRODUCTION_METADATA.md),
[music graph](../MUSIC_GRAPH.md), [POI](../SPONSORBLOCK.md).
