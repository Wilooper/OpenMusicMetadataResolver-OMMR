# Live public album artifacts

See ../../YOUTUBE_KNOWN_UNKNOWNS_VERIFICATION.md for findings and reproduction.

- album-complete.json: all album metadata, each final resolved track, source-provided credit sections, verified source observations across runs, observed ISRCs and audio/video identity distinctions.
- album-before/after/post/limited.json: actual OMMR album catalog requests, including GET/POST parity and bounded truncation.
- NN-catalog-song.json and NN-youtube-extract.json: original supplied album-row video IDs, metadata-only. Rows 03 and 10 are official videos, not the independently resolved album audio.
- NN-search-resolve.json: baseline text resolution, including the bad Apple FOLK MAFIA match. Never count the contaminated baseline composite as a correct result.
- NN-resolve-after.json and NN-credits.json: final guarded text resolution and public YouTube credits for each track.
- 09-apple-exact*.json: verified Apple FOLK MAFIA lookup using the correct track ID from the public album catalog; separate from failing text search.
- NN-short-title-resolve.json: additional searches omitting featured display text, with explicit request inputs; all returned recordings still require identity verification.
- credits-post/invalid.json: JSON gateway parity and input validation.
- apple-explicit-probes.json: safe public search/album-lookup observations. explicit=Yes did not repair the observed search ranking.
- requests*.json: exact API inputs, HTTP status and elapsed times.
- assertions.json, checks.json: live assertions and final test outcomes/limitations.
- page-probes.json: body hashes/sizes for plain-curl unsupported-browser HTML and the browser-user-agent public application shell; neither is counted as album extraction.
- config.yaml: non-secret public-only verification configuration.

Reports omit preview/media fields and signed preview URLs. Source metadata can disagree on dates, display names, rounding or credit roles; preserve those differences rather than inventing agreement. Full canonical provider diagnostics remain separate from production responses. Failed/unrelated candidates do not count as successful provider matches. No account data, credentials or media downloads were used.
