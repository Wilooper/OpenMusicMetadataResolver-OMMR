# Recording, edition and structured credit graph

OMMR exposes `music_graph` in production track responses from resolve, bulk and extraction, and in native album responses. Debug track responses store the same graph under `track.extensions.music_graph`, preserving the immutable canonical model. Existing flat credits and provider data remain available. The graph is descriptive metadata; it does not alter acceptance, canonical identity, or provider-ID matching.

```sh
curl 'http://localhost:8080/v1/resolve?musicbrainz_id=<recording-UUID>'
curl 'http://localhost:8080/v1/albums?provider=musicbrainz&id=<release-UUID>'
```

The existing JSON operations `resolve`, `extract`, `bulk` and `albums` expose the same data. No separate endpoint or credentials are required. Source enablement, timeouts, health and debug policy remain in force.

## Graph format

| Field | Meaning |
| --- | --- |
| `version` | Graph schema version, currently 1 |
| `root` | Graph-local ID of the resolved recording or requested album/release |
| `nodes` | Namespaced entities with type, provider, native ID, name and selected properties |
| `edges` | Directed relationships and scoped contributor credits |
| `truncated` | A graph depth/node/edge bound was reached; this is not an assertion of source completeness |

Native nodes use `<provider>:<type>:<native-id>`. The resolved root uses `ommr:recording:<canonical-id>` where a canonical ID exists; extraction uses the source recording as its root. Source recording nodes remain separate and connect to the accepted resolved root with `matched_catalog_entity`. These edges describe accepted catalog identity, not a new match decision. A related recording never becomes a platform ID or an accepted identity match.

Entity types include `recording`, `release`, `release_group`, `work`, `artist`, `label` and `track_entry`. An album track entry is distinct from its recording: the same recording can occur in multiple editions or positions. Source-owned artist IDs are retained. Names alone never join people across providers. A contributor without a native ID gets `local: true` and an opaque ID local to its source and credited entity. This local ID is not a verified global person identity. Structured source credits take precedence over redundant name-only fallback rows of the same name/role/context.

Node properties preserve available disambiguation, date, release status, barcode, release type, ISRC/ISWC arrays, language and video status. They do not claim that every provider returns these properties.

## Relationships

| Edge type | Direction / interpretation |
| --- | --- |
| `matched_catalog_entity` | Resolved canonical recording → accepted provider catalog entity |
| `appears_on` | Recording/track entry → exact release edition |
| `instance_of` | Album track entry → MusicBrainz recording |
| `edition_of` | Exact release → release group |
| `recording_of` | Recording → composition/work; attributes can identify live, cover, medley, instrumental, etc. |
| `remix_of`, `edit_of`, `remaster_of` | Source-reported related entity → original entity, respecting relationship direction |
| `samples_material` | Sampling entity → sampled entity |
| `replaced_by` | Withdrawn/old release → replacement |
| `supports_release` | Supporting release → promoted release |
| `released_by` | Release → label, with catalog number when reported |
| `credit` | Contributor → the entity actually credited |
| `relationship` | Unnormalized relationship; inspect `source_role`, `source_type_id`, `source_direction` and attributes |

MusicBrainz `direction: backward` reverses entity relationship edges. Unknown relationship direction is retained as an unnormalized relationship rather than guessed. Credit edges consistently run from contributor to credited entity and retain the original direction separately.

The graph never infers a relationship from a title suffix, similar artist name, duration, or shared ISRC. A remastered **release** does not prove that every contained recording is a new remaster. A cover attribute can link a recording to a work without identifying an original recording. Live/acoustic/clean/explicit variants are not automatically connected when sources provide no relationship evidence. No album or recording search is performed to invent missing relatives.

## Structured credits

Credit edges contain normalized `role`, original `source_role`, source provider and `credited_as` when reported. Artist-credit join phrases are preserved. MusicBrainz relationship credits additionally retain type UUID, instruments/attributes, attribute credit names, begin/end dates and ended state where available.

Examples of normalized roles include composer, lyricist, writer, producer, mixing/mastering/recording engineer, vocalist, instrumentalist, arranger, conductor, remixer and artwork. Unrecognized artist relationships such as dedications remain relationships; they do not automatically become musical contributor credits.

Scope is encoded by the target node:

- Work composer/lyricist credits point to that work, including each separate work in a medley.
- Recording producer/performer/instrument credits point to that recording.
- Release artwork/design credits point to the release and are not copied onto its tracks.
- Album track artist credits point to the exact track entry, preserving release-specific credited names.
- A provider's flat normalized track credits remain attached to its catalog recording context. They do not establish a separately identified composition or a global contributor identity.

`source` on an edge is evidence attribution, not an execution trace. The public graph contains no request attempts, latency, match scores, cookies, headers or raw upstream errors.

## Source coverage

MusicBrainz exact recording hydration requests recording relationships, work relationships/work-level credits and release-group context. Existing release enrichment and native release lookup request release relationships plus recording/work-level credits. Only exact-ID records and identity-checked release enrichment enter the resolved graph. Album graphs include returned track entries only, so `limit` cannot be bypassed through the graph.

Other accepted sources contribute their normalized artists/credits and native release membership. Structured JioSaavn artist maps and Deezer contributor IDs/roles add source-specific identity where available. The graph builder itself performs no network requests. It does not crawl related entities, enumerate every recording of a composition, or promise complete credits. Missing source data remains missing; `truncated: false` means the builder did not hit its own bounds, not that the provider's catalog is exhaustive.

Nodes and edges are deterministically ordered and duplicate identical edges are removed. Graphs are bounded to 4096 nodes, 8192 edges and four nested MusicBrainz expansions. Existing response-body and request limits still apply. Resolved cache advances to `rich-v14`; native album cache advances to `albums:v3`, preventing older cache entries from hiding the new metadata.

## Verification

Constructed protocol tests cover reversed remix direction; release-only remaster relationships; two-work medleys with separate composers/lyricists; same-name contributors with distinct IDs; credited aliases, instrument attributes and date ranges; unknown directions/roles; foreign raw records; graph bounds; album limits; and production/debug cache parity.

Public Apple running-server checks verify the graph is delivered alongside Boyfriend and the 11-track/two-disc P-POP CULTURE album, with only two entries in a limited album graph. This verifies actual public delivery and catalog membership, not MusicBrainz-specific relationship extraction. MusicBrainz live access remains unavailable from this runtime; its richer relationship behavior is verified with constructed responses, not a claimed live recovery. See [checks](test-results/2026-10-19-music-graph-checks.json) and [live smoke results](test-results/2026-10-19-music-graph-smoke.json).

References: [MusicBrainz API relationships](https://musicbrainz.org/doc/MusicBrainz_API#Relationships), [recording/work relationships](https://musicbrainz.org/relationships/recording-work), [recording relationships](https://musicbrainz.org/relationships/recording-recording), [release relationships](https://musicbrainz.org/relationships/release-release), [release remaster semantics](https://musicbrainz.org/relationship/48e327b5-2d04-4518-93f1-fed5f0f0fa3c).
