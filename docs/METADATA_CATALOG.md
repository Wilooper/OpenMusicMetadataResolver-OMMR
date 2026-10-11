# Metadata catalog and source policy

The unit of identity is a **recording**, not a song title. A composition (work), a recording, a release, and a music video may have different IDs and dates. Empty fields mean unknown, never a guessed value. `/v1/extract` exposes each source separately; `/v1/resolve` accepts cross-platform IDs only after matching checks.

| Field | Meaning / JSON location | Best available source | Caveat |
| --- | --- | --- | --- |
| Title, performing artists | `title`, `artists[]` | Spotify, Apple catalog, MusicBrainz recording | A YouTube channel name is not necessarily an artist. |
| Alternate title, version | `extensions` when verified | MusicBrainz recording/release | Remix, live, remaster and cover recordings must stay distinct. |
| Recording identifiers | `isrc`, `ids` (MBID and platform IDs) | Spotify/Apple ISRC, MusicBrainz MBID | An ISRC can occur in multiple catalog editions; check artist/title/duration. |
| Work identifier | `iswc`, work MBID in `extensions` | MusicBrainz work | Applies to the composition, not a particular recording. |
| Album/release name and ID | `album.title`, `album.ids` | Apple, Spotify, MusicBrainz release | Multiple editions can have different dates and art. |
| Release date and year | `release_date`, first four characters for year | Apple/Spotify album, MusicBrainz release | Keep partial date precision; album date is not necessarily recording's first release. |
| Release type | `album.type` (`album`, `single`, `compilation`) | Spotify album type, MusicBrainz release group | Distinct from audio/video or remix/live recording type. |
| Duration, explicit flag | `duration_ms`, `explicit` | Apple/Spotify | Missing explicit status must not be interpreted as clean. |
| Cover/thumbnail | `images[]` | Apple/Spotify; MusicBrainz Cover Art Archive | YouTube thumbnail belongs to a video, not necessarily the album. |
| Track and disc number | `track_number`, `disc_number` | Apple/Spotify | Edition-specific. |
| Lyricist, composer | `credits[]` with explicit roles | MusicBrainz work relations | Apple's `composerName` is a display string, not structured lyricist credit. |
| Producer, instrumentalist, engineer | `credits[]` with explicit roles | MusicBrainz recording relations | Only when credited; no inferred roles. |
| Genre, language | `genres[]`, `language` | Apple genre, MusicBrainz tags/work | Tags are community supplied and subjective. |
| Label, barcode, copyright | `label`, `barcode`, `copyrights[]` | Spotify album and MusicBrainz release | Release-level facts. |
| Preview and availability | `preview_url`, provider extension | Apple/Spotify | Storefront and territory-dependent; previews may be absent. |
| Background/context | `extensions.wikipedia` (article URL and summary) | Wikipedia via a MusicBrainz work's Wikipedia/Wikidata relation | Only use a linked article; prose is context, not recording identity or credits. |

## Provider capability and trust

| Provider | Credential / access | Reliable catalog fields | Limits |
| --- | --- | --- | --- |
| Spotify Web API | Client ID and secret; developer access rules apply | Track/album IDs, artist, title, duration, ISRC, album type/date/art, explicit, track/disc, label/copyright via album | No general lyricist field; cookie scraping is unnecessary. |
| Apple Music API | Apple developer token, official catalog API | Song ID, title, artist, album, artwork, date, ISRC, composer display, genre, duration | No structured lyricist; storefront may alter results. Public iTunes lookup is a limited keyless fallback. |
| MusicBrainz | Public API with User-Agent and rate limits | Recording/work/release MBIDs, ISRC, artist, date, credited relationships including lyricist/composer | Community curated: often sparse. Work credits must come from linked works, not artist credit. |
| YouTube Music | Optional browser cookie; unofficial InnerTube | Video ID, title, channel, duration, thumbnail | Auth and schema can change; channel/video is not a verified recording credit. Public mode remains available. |
| Amazon Music | Approved developer access, bearer token and API key | Catalog track/album/artist identifiers and core metadata | Web API V2 is a closed beta; no live guarantee without approval. |
| Deezer | Public catalog API | Track, ISRC, album, art, artist, duration | Coverage and rich credits vary. |
| Cover Art Archive | Public, linked by MusicBrainz release MBID | Release front/back cover | Requires a verified release link. |
| Wikipedia/Wikidata | Public API | Linked article context, composition background, structured topic IDs | Editorial/community content; never use free-text page search to assert recording identity. |

Qobuz, Tidal, Pandora and SoundCloud adapters remain optional secondary sources. Their presence in `/v1/extract` is not evidence that they return the same recording or support every field. Credentials belong in environment variables or a local ignored config file. Never paste cookies or tokens into request URLs.

## Request URLs and JSON shape

```text
GET /v1/extract?provider=applemusic&id=123456789
GET /v1/extract?provider=musicbrainz&id=<recording-mbid>
GET /v1/extract?artist=<artist>&title=<title>&sources=spotify,applemusic,musicbrainz
GET /v1/resolve?youtube_id=<video-id>
```

The first three return `query` and `sources[]`; each source has `provider`, `version`, `status`, and `results[]` containing `track`, `year`, `thumbnail_url`, `present_fields`, `missing_fields`. A `track` is the canonical JSON object in [SCHEMA.md](SCHEMA.md). An optional context extension looks like:

```json
{
  "extensions": {
    "wikipedia": {
      "title": "Article title",
      "description": "Brief topic description",
      "summary": "Article summary",
      "article_url": "https://en.wikipedia.org/wiki/Article_title",
      "source": "musicbrainz_work_wikipedia_link"
    }
  },
  "field_sources": { "extensions.wikipedia": ["musicbrainz", "wikipedia"] }
}
```

This is an example of the extension within a resolved track; absent article links omit it. Release year derives from the date string and does not imply day precision.

Normal resolution hydrates an accepted MusicBrainz recording by its MBID before
merging credits and work context. The lookup must agree with the accepted MBID,
title, primary artist and any known ISRC. Only credits, ISWC, language and context
are enriched; IDs and the recording identity stay fixed. Missing or conflicting
enrichment leaves the accepted result available, with `enrichment_status` on its
source status. Search-only `/v1/extract` results remain the search response;
direct MusicBrainz recording extraction retrieves work relationships.

Wikipedia context requires a single linked work and a standard article with a
summary. Disambiguation pages and mismatched Wikidata entities are rejected.
When supplied by Wikipedia, `wikidata_id` and `revision` accompany the article
context. Recordings linked to multiple works do not assert an arbitrary single
work MBID, ISWC, language or article. The extractor chooses the largest reported
thumbnail, falling back to the largest other image; it does not upscale images.

References: [Spotify track](https://developer.spotify.com/documentation/web-api/reference/get-track), [Spotify album](https://developer.spotify.com/documentation/web-api/reference/get-an-album), [Apple song attributes](https://developer.apple.com/documentation/applemusicapi/songs/attributes-data.dictionary), [Apple developer token](https://developer.apple.com/documentation/applemusicapi/generating-developer-tokens), [MusicBrainz API](https://musicbrainz.org/doc/MusicBrainz_API), [MusicBrainz works](https://musicbrainz.org/doc/How_to_Use_Works), [Amazon Music API V2](https://developer.amazon.com/docs/music/API_web_overview_v2.html).
