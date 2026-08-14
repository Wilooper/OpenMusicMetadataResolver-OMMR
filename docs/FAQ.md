# OMMR Frequently Asked Questions (FAQ)

## General Questions

### 1. How does OMMR resolve metadata without API keys?
OMMR uses public web oEmbed endpoints (YouTube Music, SoundCloud), public REST endpoints (Deezer, Apple Music iTunes Search, JioSaavn's public `api.php`), the public InnerTube web API for YouTube, and open community REST APIs (MusicBrainz v2). It parses open metadata payloads without requiring developer registrations, OAuth tokens, or secret keys. Spotify runs zero-key by default (anonymous web-player token + embed scraping); official Web API keys are an optional upgrade.

### 2. How does OMMR prevent canonical ID collisions?
OMMR generates canonical IDs using a path-independent recording hash seed: `meta:cleanTitle|cleanPrimaryArtist`. If a clean title and primary artist fold to non-empty strings, resolving via `youtube_id`, `spotify_id`, or `artist+title` produces the **exact same canonical_id** for the same recording.

### 3. What happens if a provider is down or returns an HTTP 503?
OMMR executes provider fetches concurrently using `errgroup`. If a provider fails (e.g. MusicBrainz status 503), OMMR records `success: false` and the error message in `provider_status[].error`, while successfully merging metadata from remaining active providers.

### 4. How does OMMR prevent third-party rate limit bans?
OMMR uses `internal/ratelimit/provider_limiter.go` which enforces token-bucket rate limits per provider (e.g. MusicBrainz is limited to 1 RPS, Deezer to 50 RPS).

---

## Technical Questions

### 5. How is `completeness` calculated?
Completeness is a float score from `0.00` to `1.00` based on populated field weights: Title (0.12), Artists (0.12), ISRC (0.12), Album Title (0.08), DurationMS (0.08), Cover Artwork (0.08), Genres (0.07), Credits (0.07), Release Date (0.05), Preview URL (0.05), Label (0.04), Track Number (0.03), Copyrights (0.03), ISWC (0.03), Play Count (0.03). The score is capped at 1.00.

### 6. Can I add custom providers like Last.fm or Discogs?
Yes! Register your new provider in `internal/adapters/` by implementing the `adapters.ProviderAdapter` interface:
```go
type ProviderAdapter interface {
    Name() string
    Version() string
    FetchByID(ctx context.Context, provider string, id string) (*canonical.TrackCandidate, error)
    Search(ctx context.Context, query Query) ([]canonical.TrackCandidate, error)
}
```
Then register it in `cmd/server/main.go` using `registry.Register(myAdapter)`, add a rate limit in `internal/ratelimit/provider_limiter.go`, and a trust weight in `internal/identity/graph.go`. Note that Shazam's public web search now returns HTTP 405 for non-browser clients; prefer providers with stable public endpoints like JioSaavn.
