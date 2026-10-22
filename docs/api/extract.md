# Extract independent provider metadata

[API](../API_SPEC.md) · [Providers](../PROVIDERS.md)

`GET /v1/extract` returns normalized source records independently. Its results do
not assert cross-provider recording equivalence and are not a merged resolution.

```sh
curl 'http://localhost:8080/v1/extract?provider=applemusic&id=1853034368&debug=true'
curl --get http://localhost:8080/v1/extract \
  --data-urlencode 'artist=Karan Aujla' --data-urlencode 'title=Boyfriend' \
  --data-urlencode 'sources=applemusic,ytmusic' --data-urlencode 'debug=true'
```

Use `provider` plus `id`, or `artist` plus `title` with optional `sources`/`album`/
`isrc`. Production returns `{"results": [...]}` of public tracks. Debug retains
`query` and per-source `sources`, execution status/error, adapter version, result
fields, year/thumbnail and `present_fields`/`missing_fields`.
An empty production list alone cannot distinguish source failure from no result.

## JioSaavn identifier forms

Use `id_type=native` or `id_type=share` for exact JioSaavn IDs when known. Without
it, eight-character IDs use native lookup and other IDs use share lookup for
compatibility. Share tokens and native IDs are distinct namespaces.

```sh
curl --get http://localhost:8080/v1/extract \
  --data-urlencode 'provider=jiosaavn' \
  --data-urlencode 'url=https://www.jiosaavn.com/song/mera-mann/ERIkRAwJf38' \
  --data-urlencode 'debug=true'
```

This URL form is specific to JioSaavn extraction and cannot accompany `id` or
`id_type`. Only validated official HTTPS song URLs are accepted. See
[page structure and limits](../JIOSAAVN_PUBLIC_PAGES.md).

Context enrichers such as `sponsorblock`, `lastfm` and `discogs` are not extraction
adapters. Use resolution with the corresponding configured enrichment service.
