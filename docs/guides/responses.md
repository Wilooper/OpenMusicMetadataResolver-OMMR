# Understand the response

[Docs](../README.md) · [Schema](../SCHEMA.md) · [API](../API_SPEC.md)

## Production and debug

Production is the default. Resolution returns `{"track": {...}}` or `{"track": null}`.
It keeps music metadata, accepted IDs, source catalog records and useful graphs.
It omits internal scoring and execution diagnostics. Optional fields can be absent.

| Data | Production track | Debug track |
| --- | --- | --- |
| Unified title, artists, album, duration, IDs | Root fields | Root fields |
| Source alternatives/catalog | `provider_data` | `extensions.provider_data` |
| Recording/work/release graph | `music_graph` | `extensions.music_graph` |
| Contributor index and scoped roles | `credit_details` | `extensions.credit_details` |
| Source edition positions | `album_placements` | `extensions.album_placements` |
| YouTube POIs | `video_annotations` | `extensions.video_annotations` |
| Accepted platform links | `extensions.platform_links` | `extensions.platform_links` |
| Match scores and field provenance | Omitted | Track diagnostics |
| Provider execution and enrichment status | Omitted | Response diagnostics |

`debug=true` changes presentation, not the underlying cached resolution. Native
album and artist responses have their own envelopes; see their references.

## Interpret identity and missingness

`identity_status` describes accepted evidence. It is separate from completeness,
which is a debug coverage score. A well-identified song may have few credits.
`track: null` is not a server error and does not prove the song does not exist.

Durations and POI offsets use milliseconds; fingerprint input duration uses seconds.
Unknown positions are omitted instead of set to one. `explicit_status` distinguishes
`explicit`, `clean` and `unknown`; the legacy false boolean alone is insufficient.
Preserve source ID strings, including large numeric-looking IDs, without float conversion.

## Recording, edition, contributor

Keep the selected album ID and its disc/track positions together. Same-title album
IDs are not proof of edition equivalence. A graph relationship to another version
is not an accepted platform match. A contributor's credit applies to its referenced
subject; a composition writer is not automatically a release artwork contributor.
Local name-only contributor IDs do not establish real-world identity across sources.

## Smaller transport

```sh
curl --compressed 'http://localhost:8080/v1/resolve?apple_id=1829591986'
```

Eligible JSON under `/v1/` at least 1024 bytes can use negotiated gzip. Compression
preserves metadata; production mode additionally removes diagnostics. Provider data
can still be large. There is no general field-selection query parameter.
[Compression behavior and measured examples](../DETAILED_PRODUCTION_METADATA.md)

## Render responsibly

Use optional-field handling and retain provider attribution for biography/POI data.
Do not turn counters with text such as `162K` into exact integers or combine views,
followers and listeners. Community highlights have votes and video-specific timing;
they do not prove a chorus boundary. [Licensing](licensing.md)
