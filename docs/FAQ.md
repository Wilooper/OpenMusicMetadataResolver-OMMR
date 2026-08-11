# OMMR Frequently Asked Questions (FAQ)

## General Questions

### 1. How does OMMR resolve metadata without API keys?
OMMR uses public web oEmbed endpoints (YouTube Music, Spotify), public REST endpoints (Deezer, Apple Music iTunes Search API), and open community REST APIs (MusicBrainz v2). It parses open metadata payloads without requiring developer registrations, OAuth tokens, or secret keys.

### 2. How does OMMR prevent canonical ID collisions?
OMMR generates canonical IDs using a path-independent recording hash seed: `meta:cleanTitle|cleanPrimaryArtist`. If a clean title and primary artist fold to non-empty strings, both `youtube_id=FVNSACXFAy0` and `artist=Talwiinder&title=Tu` produce the **exact same canonical_id** (`ommr_track_2c15190166124384`).

### 3. What happens if a provider is down or returns an HTTP 503?
OMMR executes provider fetches concurrently using `errgroup`. If a provider fails (e.g. MusicBrainz status 503), OMMR records `success: false` and the error message in `provider_status[].error`, while successfully merging metadata from remaining active providers.

### 4. How does OMMR prevent third-party rate limit bans?
OMMR uses `internal/ratelimit/provider_limiter.go` which enforces token-bucket rate limits per provider (e.g. MusicBrainz is limited to 1 RPS, Deezer to 50 RPS).

---

## Technical Questions

### 5. How is `completeness` calculated?
Completeness is a float score from `0.00` to `1.00` based on populated field weights: Title (0.15), Artists (0.15), ISRC (0.15), Album Title (0.10), DurationMS (0.10), Cover Artwork (0.10), Genres (0.10), Credits (0.10), Release Date (0.05).

### 6. Can I add custom providers like JioSaavn, Last.fm, or Discogs?
Yes! Register your new provider in `internal/adapters/` by implementing the `adapters.ProviderAdapter` interface:
```go
type ProviderAdapter interface {
    Name() string
    Version() string
    FetchByID(ctx context.Context, provider string, id string) (*canonical.TrackCandidate, error)
    Search(ctx context.Context, query Query) ([]canonical.TrackCandidate, error)
}
```
Then register it in `internal/adapters/registry.go` using `registry.Register(myAdapter)`.
