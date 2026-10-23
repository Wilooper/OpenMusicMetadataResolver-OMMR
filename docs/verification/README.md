# Verification and investigation archive

[Docs](../README.md) · [How to reproduce](../guides/testing.md)

These files record specific test runs and networks. Their source counts, timings,
versions and availability statements are historical observations. Use current
reference guides and your instance's diagnostics for present behavior.

| Report | Scope |
| --- | --- |
| [Documentation revision, October 22](DOCUMENTATION_2026-10-22.md) | Links, examples, Go checks and running-server smoke checks |
| [Accuracy improvements](../ACCURACY_IMPROVEMENTS.md) | Candidate/version acceptance and detail hydration |
| [Boyfriend live test](../LIVE_TEST_BOYFRIEND.md) | Keyless source behavior and metadata comparisons |
| [Provider diagnostics](../PROVIDER_DIAGNOSTICS.md) | Individual source errors and JioSaavn protocol investigation |
| [Song matrix](../LIVE_SONG_MATRIX.md) | Multi-song identity and metadata coverage |
| [Rich catalogs](../LIVE_RICH_CATALOGS.md) | Korean adapters and Python catalog observations |
| [Termux provider verification](../TERMUX_PROVIDER_VERIFICATION.md) | Device-network results and page/parser fixes |
| [Artist-profile captures](../test-results/artist-profiles-2026-10-18/README.md) | Ambiguity, metrics and exact lookups |
| [Full verification, October 19](../FULL_VERIFICATION_2026-10-19.md) | Whole-service fixtures and live source limits |
| [Entity matrix, October 19](../ENTITY_MATRIX_2026-10-19.md) | Songs, albums, artists and edge cases |
| [Status, October 19](../STATUS_2026-10-19.md) | Progress snapshot |
| [YouTube Known Unknowns](../YOUTUBE_KNOWN_UNKNOWNS_VERIFICATION.md) | Album songs, credits and recording-version checks |
| [Native album verification](../NATIVE_ALBUMS.md#verification-on-2026-10-19) | Exact releases and partial-list behavior |
| [Production metadata verification](../DETAILED_PRODUCTION_METADATA.md#live-verification-2026-10-21) | Credits, placement and gzip |
| [SponsorBlock verification](../SPONSORBLOCK.md#verification-2026-10-21) | Exact-video POIs and source budget |

Raw sanitized captures/check summaries remain in [test-results](../test-results/).
A fixture pass establishes behavior against a saved shape; a live success is a
separate observation. Neither guarantees every song or authenticated provider.
