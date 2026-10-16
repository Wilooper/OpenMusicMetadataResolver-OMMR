# Rich metadata and provider data

## Response contract

`/v1/resolve` and bulk tracks expose unified trusted fields plus `provider_data` in production mode. Diagnostics remain behind `debug=true`. Debug retains the immutable internal Track format: the same extra data is in `track.extensions.provider_data`, and the tri-state marker is in `track.extensions.explicit_status`.

```json
{
  "track": {
    "title": "Example Song",
    "explicit": false,
    "explicit_status": "unknown",
    "provider_data": {
      "spotify": {
        "metadata": {"title": "Example Song", "ids": {"spotify": "accepted-id"}},
        "catalog": {"id": "accepted-id", "popularity": 73, "new_provider_field": "retained"}
      },
      "lastfm": {"scope": "track_context", "track": {"listeners": "1234", "toptags": {"tag": []}}}
    }
  }
}
```

This is a partial illustration, not a complete returned track. Source names group the data supplied by each provider; they are useful metadata rather than execution traces. Recording candidates enter this map **only after** the resolver accepts them. In `/extract`, records are independent extractions and do not assert cross-provider equivalence.

Each accepted provider contributes:

- `metadata`: its normalized fields, including alternatives to the unified values. Matching diagnostics and foreign provider IDs are removed.
- `catalog`, when the original JSON can be tied to that provider's exact track ID: the matching entity's public fields, including unknown/new fields.
- `extra`: provider extension metadata such as full accepted MusicBrainz recording relationships.

Full search envelopes, unrelated results, player streaming payloads, request headers, authentication tokens, cookies, signatures, HTML widgets and tracking fields are excluded. Raw entity selection is bounded to 8 MiB input and 16 levels; field cleanup is bounded to 24 levels. If a source cannot be safely associated with an exact entity, its normalized metadata is still retained. This is not a promise to echo arbitrary upstream responses byte-for-byte. `metadata.explicit=false` alone does not establish confirmed clean content.

## New enrichment sources

| Source | Activation | Data retained | Identity scope |
| --- | --- | --- | --- |
| Cover Art Archive | Default when an accepted album has a MusicBrainz release ID | Original image URLs, thumbnails, types, comments, approval/edit metadata; image union in unified `images` | Exact release ID; does not create recording IDs |
| MusicBrainz release lookup | Default for linked releases | Barcode, label info, release groups, media, tracks, relationships and provider fields | Release ID/title must match, and the accepted recording MBID must occur on that release |
| Last.fm | `OMMR_LASTFM_API_KEY` | Tags, wiki/context, listeners, play counts and track fields | Exact title/primary artist; rejects conflicting known MBIDs; no autocorrection |
| Discogs | `OMMR_DISCOGS_TOKEN` | Matching edition context: styles, genres, notes, companies, formats, identifiers, images and track details | Exact album/title/artist; known duration must agree within 2 seconds. Returns edition context, never a claimed recording ID |
| AcoustID | `OMMR_ACOUSTID_API_KEY`; `POST /v1/fingerprint` | All acoustic candidates and linked recording/release metadata | Separate candidate lookup, never automatic selection of an ambiguous match |

These sources are enrichment services, not additional streaming adapters. `/v1/extract?provider=lastfm` and `sources=discogs` are not supported. Resolution uses the existing adapter acceptance gates first. Set `OMMR_ENABLE_ENRICHMENT=false` to skip the post-merge catalog services; accepted MusicBrainz recording/work enrichment remains part of its adapter.

MusicBrainz release IDs are retained from same-title accepted album records even when another provider supplies the preferred album display fields. Edition context is not promoted into canonical recording facts. A source with no access, no catalog match or no remaining request time is omitted; debug `enrichment_status` distinguishes `ok`, `no_data` and `unavailable`.

Catalog services run concurrently after the merge, with a maximum 5-second window bounded by the remaining request deadline. MusicBrainz recording and release requests share its one-per-second local budget. Successful catalog GET JSON is cached for 24 hours. Cover Art Archive's public JSON redirect is followed only over HTTPS to `archive.org` or its subdomains; credentialed services do not follow redirects.

## Fingerprint API

```http
POST /v1/fingerprint
Content-Type: application/json

{"fingerprint":"CHROMAPRINT_OUTPUT","duration":180}
```

Duration is the whole file's length in seconds, 1–86400. The caller supplies a Chromaprint fingerprint; the server does not download or process audio. The endpoint returns the AcoustID `status` and all `results`. It has a 128 KiB body limit and 65,536-character fingerprint limit. Missing configuration returns 503, bad input 400, upstream failure 502. The server sends the operator key upstream in POST form data; clients do not put it into request URLs.

AcoustID requires an application key and documents a 3 requests/second limit; its free service is non-commercial. Last.fm requires an API key. Discogs search uses a personal access token. These access conditions cannot be removed by the resolver.

## Field selection

Default mode is consensus: each accepted provider gets one vote per populated field; ties use stable provider priorities, then score/name ordering. Empty/missing values do not vote. Artist names and album titles are compared independently of provider-specific IDs. Scalar duration values vote exactly; close-but-different durations are retained in provider data and use the deterministic tie-break, not a guessed equivalence. This is provider agreement, not proof that all catalogs are independent.

Override individual fields or select priority mode using a JSON string:

```bash
export OMMR_METADATA_POLICY='{"mode":"consensus","priorities":{"title":["musicbrainz","spotify"],"release_date":["musicbrainz","applemusic"]}}'
```

An explicit field priority overrides consensus **for that field**. `mode=priority` uses priorities throughout. Supported fields: `title`, `artists`, `album`, `duration_ms`, `release_date`, `isrc`, `iswc`, `language`, `label`, `barcode`, `preview_url`, `track_number`, `disc_number`, `explicit`. Unknown modes/fields, duplicate priority names and unknown configured providers fail startup. Union fields (credits, images, genres, copyrights) retain contributions rather than choosing one value.

`explicit_status` is `explicit`, `clean` or `unknown`. Missing false flags do not vote against positively known explicit/clean flags. The legacy boolean remains for compatibility. Provider data preserves conflicts instead of erasing them.

## Validation and live access

Deterministic tests exercise exact-identity checks, rejected artists/editions, extra-field preservation, credential handling, each new source's request format, shared caching/cancellation, and consensus ordering. Live checks observed Discogs release JSON HTTP 200, Cover Art Archive HTTP 307 to its Archive.org JSON location, and expected missing-credential responses from Last.fm/AcoustID. The Archive.org fetch timed out in this environment; complete Cover Art Archive retrieval is fixture-tested. No operator keys were available for live Last.fm/Discogs search/AcoustID equivalence tests.

Official references: [Cover Art Archive](https://musicbrainz.org/doc/Cover_Art_Archive/API), [MusicBrainz API](https://musicbrainz.org/doc/MusicBrainz_API), [Last.fm track.getInfo](https://www.last.fm/api/show/track.getInfo), [Discogs](https://www.discogs.com/developers/), [AcoustID](https://acoustid.org/webservice).

## Lifecycle activation

The activation variables above are necessary but no longer sufficient: configured Last.fm, Discogs and AcoustID access must pass uncached startup probes. Failed/missing credentials disable the optional service; runtime circuits can suspend repeatedly failing services. Config and redacted status are described in [PROVIDER_LIFECYCLE.md](PROVIDER_LIFECYCLE.md). No live credential reload is implemented.
