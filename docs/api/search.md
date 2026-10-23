# Search for songs

[API](../API_SPEC.md)

`GET /v1/search` provides lightweight discovery results. It does not return the
complete metadata/graph of `/v1/resolve` and is not a globally complete catalog.

```sh
curl --get http://localhost:8080/v1/search \
  --data-urlencode 'q=Karan Aujla Boyfriend' \
  --data-urlencode 'sources=applemusic,ytmusic' \
  --data-urlencode 'page=1' --data-urlencode 'limit=5'
```

| Parameter | Behavior |
| --- | --- |
| `q` | Required nonempty search text |
| `sources` | Optional comma-separated recording adapter names |
| `page` | Defaults/clamps to 1 for values below 1 |
| `limit` | Default 10; accepted range 1–50; invalid/out-of-range input uses 10 |
| `debug` | Adds scores, confidence and identity diagnostics |

Production returns `query`, `results`, `page`, `limit` and `total`; results include
available title, artists, album, images, canonical ID and `available_on` fields.
Pagination covers the bounded collected result pool, not an upstream exhaustive
track count. Configuration can replace default sources and limits.

Follow up with a known provider ID or artist/title resolution for full accepted
metadata. Providers may have different search and exact-lookup availability;
SoundCloud's public adapter and Pandora's implemented path do not offer text
search. Inspect [capabilities](../SHARE_LINKS_AND_CAPABILITIES.md#capability-discovery).
