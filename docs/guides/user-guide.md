# Use OMMR

[Docs](../README.md) · [API](../API_SPEC.md) · [Responses](responses.md)

## 1. Resolve what is playing

Use the actual song link or native track ID when you have one. This anchors
resolution to the source recording and, where available, its release edition.

```sh
curl --compressed --get http://localhost:8080/v1/resolve \
  --data-urlencode 'url=https://music.apple.com/us/album/boyfriend/1829591921?i=1829591986'
```

For text input, provide both artist and title; add album to disambiguate:

```sh
curl --compressed --get http://localhost:8080/v1/resolve \
  --data-urlencode 'artist=Karan Aujla' \
  --data-urlencode 'title=Boyfriend' \
  --data-urlencode 'album=P-POP CULTURE' \
  --data-urlencode 'sources=applemusic,ytmusic'
```

Read `track`, handle `null`, and retain provider IDs. An OMMR ID derived from
metadata is not an immutable external recording identifier. See [identity](../IDENTITY_ENGINE.md).

## 2. Show links and album placement

`/v1/links` accepts the same inputs and returns accepted platform URLs. Unknown
services are omitted; a missing URL does not prove the song is absent there.
For disc/track position, read the selected `album_placements` entry. Its provider,
album ID and track ID belong together. Then fetch that exact edition:

```sh
curl --compressed 'http://localhost:8080/v1/albums?provider=applemusic&id=1829591921'
```

Check `album.track_list.complete` and each disc's completion before displaying
"complete album". Unknown totals and omitted positions remain unknown. Native
album support covers Apple Music, Deezer, JioSaavn and MusicBrainz releases;
YouTube album context is exposed separately through the optional catalog worker.

## 3. Display credits and relationships

Use `credit_details.contributors` to label referenced people and
`credit_details.credits` to find their roles and subject IDs. Follow the subject
in `music_graph.nodes` to distinguish recording, work and release credits.
The legacy `credits` array is a convenience view. Source description text may
contain credits that are not normalized into either structured representation.
[Credit scope and graph](../MUSIC_GRAPH.md)

## 4. Get profiles and catalog context

```sh
curl --get http://localhost:8080/v1/artists \
  --data-urlencode 'artist=Gur Sidhu' --data-urlencode 'limit=5'
curl 'http://localhost:8080/v1/catalog?provider=ytmusic&type=song&id=BBM57gr9fFU'
```

Artist profiles stay separate by provider; ambiguous names return candidates.
YouTube catalog requests require the [Python worker](python-worker.md).
[Artist counters](../ARTIST_PROFILES.md) preserve source meaning and precision;
listeners, subscribers, views and plays are different measurements.

## 5. Use video highlights

Resolve an exact YouTube video ID. When compatible community data exists, read
`track.video_annotations.sponsorblock.points_of_interest[].time_ms`.

```sh
curl --compressed 'http://localhost:8080/v1/resolve?youtube_id=BBM57gr9fFU&sources=ytmusic'
```

A POI is a point on that video's timeline. It does not supply an end time or
permission to transfer it to another upload. Retain votes, duration-check state
and attribution. Absence is valid. [SponsorBlock](../SPONSORBLOCK.md)

## 6. Investigate missing data

Add `debug=true` to inspect `provider_status`, `enrichment_status` and field
provenance. Use `/v1/extract` for independent source records, `/v1/search` for a
lightweight discovery list, and `/v1/bulk` for bounded batches. Their contracts
are separate: [API index](../API_SPEC.md).
