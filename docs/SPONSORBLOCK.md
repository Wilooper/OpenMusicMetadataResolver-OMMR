# SponsorBlock points of interest for accepted YouTube videos

OMMR can add SponsorBlock community highlight points to a resolved track when its accepted metadata contains an exact YouTube video ID. It reuses the resolver's existing YouTube ID and video duration; it does not search SponsorBlock by artist/title or make another YouTube metadata request just for this enrichment.

SponsorBlock is a post-merge enrichment source, not a music identity provider. Its points never create accepted video IDs, change a canonical song ID, overwrite track duration or establish recording/album equivalence.

## Response

Production tracks expose `video_annotations.sponsorblock`. Debug tracks keep the same data under `extensions.video_annotations.sponsorblock`. The enrichment is stored once; it is not duplicated in `provider_data`. Existing lossless gzip applies normally.

```json
{
  "video_annotations": {
    "sponsorblock": {
      "source": "SponsorBlock",
      "source_url": "https://sponsor.ajay.app",
      "license": "CC-BY-NC-SA-4.0",
      "license_url": "https://creativecommons.org/licenses/by-nc-sa/4.0/",
      "attribution": "Uses SponsorBlock data from https://sponsor.ajay.app",
      "service": "YouTube",
      "video_id": "kJQP7kiw5Fk",
      "video_duration_ms": 282000,
      "points_of_interest": [{
        "id": "bcf6e75825bca74f69df17bda9e5b7509a4a6957c367080a672d26ccc20aded46",
        "time_ms": 21808,
        "category": "poi_highlight",
        "action_type": "poi",
        "votes": 9,
        "locked": 1,
        "submission_video_duration_ms": 281521,
        "duration_checked": true
      }]
    }
  }
}
```

This excerpt comes from the saved live Despacito video response. Data and votes can change. `time_ms` is a point on this exact video's timeline, not a time range or a universal song/chorus/lyric timestamp. Do not transfer it to Spotify/Apple audio, a different upload, or an edited recording without a separate alignment. An optional source `description` is retained when nonempty.

A point stays a point: SponsorBlock's start and end must be equal, and OMMR does not manufacture a one-second segment. Source seconds are rounded to milliseconds. The UUID, category, action type, votes, lock state and submitted video duration remain available. `duration_checked` is true only when both the accepted YouTube record and SponsorBlock submission report compatible video durations; it is false when one duration is unknown. It is not a content/chorus-quality score.

## Exact-video and timing checks

- Query only an accepted `ytmusic` ID matching the 11-character YouTube ID format. Rejected candidates cannot trigger lookup; absent accepted IDs cause no network call.
- Use SponsorBlock's recommended four-character SHA-256 prefix endpoint with `categories=["poi_highlight"]`, `actionTypes=["poi"]` and `service=YouTube`.
- A hash bucket can contain unrelated videos. Only the entry whose full `videoID` exactly equals the accepted ID is used. Other video records never appear in OMMR output.
- Require the correct category/action, an ID and a finite, nonnegative equal start/end pair. Reject points beyond known video bounds. Reject submission durations that differ by more than one second from the accepted YouTube video's reported duration.
- The duration comes from the owned `provider_data.ytmusic.metadata` record, not the merged song duration. A music video can contain an intro or credits beyond the album audio. Unknown durations stay unknown.
- Deduplicate identical UUID observations and reject conflicting same-UUID data or duplicate matching video entries. Sort points deterministically. Return at most 100 points with an explicit `truncated` flag if that bound is reached; bodies remain bounded to 4 MiB.

These rules validate metadata consistency, not audio fingerprints or the artistic significance of a community highlight.

## Configuration and latency

The keyless source is enabled with the other post-merge enrichers by default. Disable it independently or disable all catalog enrichment:

```yaml
enable_enrichment: true
sponsorblock_timeout: 2s
providers:
  enabled:
    sponsorblock: true
```

`providers.enabled.sponsorblock: false` disables this source. `enable_enrichment: false` also disables it. `/v1/providers` reports its lifecycle state. It is not a track adapter and cannot be selected using `sources=sponsorblock` or `/v1/extract?provider=sponsorblock`.

`OMMR_SPONSORBLOCK_TIMEOUT` / `sponsorblock_timeout` defaults to **2 seconds**, accepts **100 ms–30 seconds**, and is bounded by the overall request deadline. The configured budget is preserved through the health wrapper, resolver and HTTP client. Changing it also changes the resolver cache variant, so an earlier short-budget result is not reused after a configuration change. Other enrichers retain their existing budgets.

For slow networks, increase both the optional source budget and the overall request budget as needed. The live verification in this runtime used `sponsorblock_timeout: 10s` and `provider_timeout: 25s`, because direct SponsorBlock requests took about seven seconds. This is a runtime observation, not expected service latency everywhere. The initial two-second-budget run successfully resolved songs but omitted the timed-out POI; that evidence is retained.

```sh
curl --compressed 'http://localhost:8080/v1/resolve?youtube_id=kJQP7kiw5Fk&sources=ytmusic'
```

Failures/timeouts do not fail the song response. Missing, incompatible or unavailable points are omitted; debug `enrichment_status.sponsorblock` reports `ok`, `no_data` or `unavailable`. Successful source configuration does not guarantee every video has a highlight.

## Caching and source policy

The fixed public HTTPS API uses no credentials and refuses redirects. Each physical request uses the shared local SponsorBlock budget of 2 requests/second, burst 1; this is OMMR's conservative limit, not an asserted upstream quota. Cache hits occur before rate-limit waits. No startup network probe is needed.

Successful raw hash buckets cache for one hour; HTTP 404 becomes an empty result cached for five minutes. HTTP 429/5xx, invalid JSON and network failures are not cached as successful emptiness. Resolved tracks containing an attempted YouTube/SponsorBlock lookup cache for at most one hour with data, five minutes with no data, or the existing thirty-second failure cap when unavailable. Shorter existing cache limits remain shorter. Raw and resolved cache layers can compound; this is not real-time notification of database changes.

Source enable flags, health circuits, cancellation, per-request budgets and production/debug cache sharing apply. No source data is promoted into recording identity. Resolved cache advances to `rich-v17`; native albums remain `albums:v5`.

## Attribution and dataset license

SponsorBlock API/database data is supplied under **CC BY-NC-SA 4.0** unless separate permission applies. The annotation carries source, license link and attribution explicitly; retain that attribution when presenting or redistributing these data. This source's data license is separate from OMMR's code and other providers' metadata.

Official references: [API](https://wiki.sponsor.ajay.app/w/API_Docs), [POI category/action semantics](https://wiki.sponsor.ajay.app/w/Types), [database/API license](https://github.com/ajayyy/SponsorBlock/wiki/Database-and-API-License), [attribution template](https://gist.github.com/ajayyy/4b27dfc66e33941a45aeaadccb51de71).

## Verification, 2026-10-21

The final public-only running-server checks passed:

1. Exact Despacito YouTube metadata resolved; SponsorBlock returned the 21.808-second POI, nine votes and lock state 1. Submitted duration 281.521 seconds matched the accepted 282-second video within tolerance.
2. Production, gzip, JSON gateway and cached responses preserved the same annotations. The observed production response measured 7,083 bytes plain and 2,417 gzip.
3. An Apple-only Boyfriend request with no accepted YouTube ID returned no video annotation.
4. The accepted Boyfriend YouTube video resolved with `no_data`, demonstrating absent highlight coverage.
5. Disabling SponsorBlock preserved successful YouTube metadata and reported its source as disabled.

The initial short-budget source timeouts and an unrelated YouTube metadata failure remain under the saved `initial/` and `short-timeout/` results. The final run uses the documented longer budget; it does not claim that the two-second budget succeeded here.

Fixture regressions cover hash collisions, foreign/rejected IDs, stale and unknown durations, exact-video rather than canonical-audio timing, zero-time points, invalid ranges/bounds, deduplication/conflicts, point/body bounds, negative caching, upstream errors, cancellation, lifecycle flags, configurable budgets and public/debug cache parity.

Build, vet and the full race/coverage suite passed: **275 top-level tests, 420 cases, zero failures/skips**.

See [full checks](test-results/sponsorblock-2026-10-21/checks.json), [final live summary](test-results/sponsorblock-2026-10-21/live-summary.json) and captured responses alongside them.

```sh
go build -o bin/server ./cmd/server
python tools/live_sponsorblock.py --server bin/server --output /tmp/ommr-sponsorblock
```
