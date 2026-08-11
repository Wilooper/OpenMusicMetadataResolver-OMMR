# OMMR Architecture Guide

## Overview

Open Music Metadata Resolver (OMMR) is designed following **Clean Architecture principles**, separating domain models, application use cases (Resolver Engine), provider adapters, and infrastructure primitives (Cache, Rate Limiters, Metrics).

---

## Architecture Layers

```mermaid
graph TD
    subgraph Presentation Layer
        Router[internal/api/router.go]
        Handler[internal/api/handler.go]
    end

    subgraph Domain & Core Resolver Layer
        Resolver[internal/resolver/resolver.go]
        Identity[internal/identity/identity.go & graph.go]
        Matcher[internal/matcher/matcher.go]
        Merger[internal/merger/merger.go]
    end

    subgraph Provider Adapter Layer
        Registry[internal/adapters/registry.go]
        Spotify[internal/adapters/spotify]
        YTMusic[internal/adapters/ytmusic]
        Apple[internal/adapters/applemusic]
        Deezer[internal/adapters/deezer]
        MusicBrainz[internal/adapters/musicbrainz]
    end

    subgraph Infrastructure Layer
        DiskCache[internal/cache/disk]
        RedisCache[internal/cache/redis]
        Limiter[internal/ratelimit]
        Metrics[internal/metrics]
    end

    Handler --> Resolver
    Resolver --> Registry
    Resolver --> Matcher
    Resolver --> Merger
    Resolver --> Identity
    Registry --> Spotify
    Registry --> YTMusic
    Registry --> Apple
    Registry --> Deezer
    Registry --> MusicBrainz
    Resolver --> DiskCache
    Resolver --> RedisCache
    Resolver --> Limiter
```

---

## 6-Stage Identity Resolution Pipeline

Every resolution request processed by `Resolver.Resolve()` executes through a strict 6-stage pipeline:

```mermaid
sequenceDiagram
    autonumber
    participant C as Client
    participant R as Resolver Engine
    participant P as Provider Adapters
    participant M as Matcher & Merger
    participant I as Identity Engine

    C->>R: Resolve Request (ID or Artist+Title)
    Note over R: Stage 1: Direct Provider Lookup & Cache Check
    R->>P: Fetch Primary ID Candidate (e.g. YouTube ID)
    P-->>R: Primary Candidate
    Note over R: Stage 2: Parallel Secondary Adapter Fetch
    R->>P: Query active non-matched adapters in parallel (errgroup)
    P-->>R: Provider Candidates
    Note over R: Stage 3: Noise Cleaning & ISRC Discovery
    R->>M: Clean diacritics, strip topic/VEVO suffixes, extract ISRC
    Note over R: Stage 4: Composite Scoring & Trust Weights
    R->>M: Calculate Jaro-Winkler, Token Set Ratio & Trust Weights
    M-->>R: Scored Candidates & Nullable Match Breakdown
    Note over R: Stage 5: Multi-Source Priority Merger
    R->>M: Merge field priorities, multi-role credits & sorted images
    M-->>R: Unified Track Record
    Note over R: Stage 6: Post-Enrichment Identity Stability
    R->>I: Assign IdentityMatches, IdentityStatus & CanonicalID
    I-->>R: Finalized Canonical Track
    R-->>C: JSON Response with Diagnostics & Provenance
```

---

## Concurrency & Rate Limiting Model

1. **Parallel Provider Execution**: Provider queries run concurrently using `golang.org/x/sync/errgroup`. A failure in one provider (e.g., MusicBrainz 503) does not abort other providers; errors are recorded in `provider_status[].error`.
2. **Provider-Specific Rate Limiting**: `internal/ratelimit/provider_limiter.go` maintains per-provider token buckets to prevent HTTP 429 rate limit bans from third-party services.
3. **Bulk Resolution Concurrency**: `POST /v1/bulk` processes array queries across a bounded worker pool controlled by `config.BulkWorkers` (default: 10 workers).
