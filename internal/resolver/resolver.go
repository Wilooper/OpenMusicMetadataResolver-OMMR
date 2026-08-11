package resolver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/cache"
	"github.com/ommr/ommr/internal/identity"
	"github.com/ommr/ommr/internal/matcher"
	"github.com/ommr/ommr/internal/merger"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/ratelimit"
	"github.com/ommr/ommr/pkg/query"
	"golang.org/x/sync/errgroup"
)

type ResolveRequest struct {
	Query       adapters.Query
	Sources     []string
	BypassCache bool
}

type ResolverStats struct {
	ProvidersQueried     int `json:"providers_queried"`
	ProvidersMatched     int `json:"providers_matched"`
	ProvidersContributed int `json:"providers_contributed"`
	CandidatesEvaluated  int `json:"candidates_evaluated"`
}

type ResolveResponse struct {
	Track              *canonical.Track             `json:"track"`
	MetadataSources    []string                     `json:"metadata_sources"`
	ProviderStatus     []canonical.SourceStatus     `json:"provider_status"`
	ResolutionStrategy canonical.ResolutionStrategy `json:"resolution_strategy"`
	ResolverStats      ResolverStats                `json:"resolver_stats"`
}

type Resolver struct {
	registry                  *adapters.Registry
	cache                     cache.Cache
	limiter                   *ratelimit.ProviderLimiter
	matcher                   *matcher.Matcher
	merger                    *merger.Merger
	identityEngine            *identity.IdentityEngine
	ttl                       time.Duration
	maxCandidatesPerProvider int
	logger                    *slog.Logger
}

func New(r *adapters.Registry, c cache.Cache, l *ratelimit.ProviderLimiter, ttl time.Duration) *Resolver {
	return &Resolver{
		registry:                  r,
		cache:                     c,
		limiter:                   l,
		matcher:                   matcher.New(),
		merger:                    merger.New(),
		identityEngine:            identity.New(),
		ttl:                       ttl,
		maxCandidatesPerProvider: 10,
		logger:                    slog.Default(),
	}
}

func (res *Resolver) Resolve(ctx context.Context, req ResolveRequest) (*ResolveResponse, error) {
	// Clean text inputs
	req.Query.Title = query.CleanTitle(req.Query.Title)
	req.Query.Artist = query.CleanArtist(req.Query.Artist)

	inputType := determineInputType(req.Query)
	cacheKey := "resolved:" + hashQuery(req.Query)

	// 1. Check cache unless bypass_cache is true
	if !req.BypassCache && res.cache != nil {
		if val, found, err := res.cache.Get(ctx, cacheKey); err == nil && found {
			var cachedResp ResolveResponse
			if err := json.Unmarshal(val, &cachedResp); err == nil {
				for i := range cachedResp.ProviderStatus {
					cachedResp.ProviderStatus[i].Cached = true
				}
				return &cachedResp, nil
			}
		}
	}

	// 2. Select target adapters
	activeAdapters, err := res.registry.ListActive(req.Sources)
	if err != nil {
		return nil, err
	}
	if len(activeAdapters) == 0 {
		return nil, fmt.Errorf("no provider adapters available")
	}

	// Stage 1: Direct Provider Lookup & Initial Parallel Fetch
	g, gCtx := errgroup.WithContext(ctx)
	candidatesMu := sync.Mutex{}
	candidates := make([]canonical.TrackCandidate, 0, len(activeAdapters))
	sourcesStatus := make([]canonical.SourceStatus, len(activeAdapters))
	primarySource := ""

	for i, adapter := range activeAdapters {
		idx := i
		ad := adapter
		g.Go(func() error {
			start := time.Now()
			status := canonical.SourceStatus{
				Name:    ad.Name(),
				Version: ad.Version(),
			}

			if err := res.limiter.Wait(gCtx, ad.Name()); err != nil {
				status.Success = false
				status.Error = err.Error()
				status.RejectionReason = "provider_error"
				status.LatencyMS = time.Since(start).Milliseconds()
				sourcesStatus[idx] = status
				return nil
			}

			var fetched []canonical.TrackCandidate
			var fetchErr error

			if req.Query.SpotifyID != "" && ad.Name() == "spotify" {
				cand, err := ad.FetchByID(gCtx, "spotify", req.Query.SpotifyID)
				fetchErr = err
				if cand != nil {
					fetched = append(fetched, *cand)
					primarySource = ad.Name()
				}
			} else if req.Query.DeezerID != "" && ad.Name() == "deezer" {
				cand, err := ad.FetchByID(gCtx, "deezer", req.Query.DeezerID)
				fetchErr = err
				if cand != nil {
					fetched = append(fetched, *cand)
					primarySource = ad.Name()
				}
			} else if req.Query.AppleID != "" && ad.Name() == "applemusic" {
				cand, err := ad.FetchByID(gCtx, "applemusic", req.Query.AppleID)
				fetchErr = err
				if cand != nil {
					fetched = append(fetched, *cand)
					primarySource = ad.Name()
				}
			} else if req.Query.YouTubeID != "" && ad.Name() == "ytmusic" {
				cand, err := ad.FetchByID(gCtx, "youtube", req.Query.YouTubeID)
				fetchErr = err
				if cand != nil {
					fetched = append(fetched, *cand)
					primarySource = ad.Name()
				}
			} else {
				cands, err := ad.Search(gCtx, req.Query)
				fetchErr = err
				if len(cands) > res.maxCandidatesPerProvider {
					cands = cands[:res.maxCandidatesPerProvider]
				}
				fetched = cands
			}

			status.LatencyMS = time.Since(start).Milliseconds()
			if fetchErr != nil {
				status.Success = false
				status.Error = fetchErr.Error()
				status.RejectionReason = "provider_error"
			} else if len(fetched) == 0 {
				status.Success = true
				status.RejectionReason = "no_results"
			} else {
				status.Success = true
				status.Matched = true
				candidatesMu.Lock()
				candidates = append(candidates, fetched...)
				candidatesMu.Unlock()
			}

			sourcesStatus[idx] = status
			return nil
		})
	}

	_ = g.Wait()

	// Stage 2 & 3: ISRC & Cleaned Metadata Cross-Platform ID Discovery
	crossResolved := false
	if len(candidates) > 0 && inputType != "artist_title" {
		initialMerged := res.merger.MergeCandidates(candidates)
		if initialMerged != nil && (initialMerged.Title != "" || initialMerged.ISRC != "") {
			cleanTitle := query.CleanTitle(initialMerged.Title)
			cleanArtist := query.CleanArtist(getPrimaryArtist(initialMerged.Artists))

			secQuery := adapters.Query{
				Title:  cleanTitle,
				Artist: cleanArtist,
				ISRC:   initialMerged.ISRC,
			}

			secG, secCtx := errgroup.WithContext(ctx)
			for i, adapter := range activeAdapters {
				idx := i
				ad := adapter
				if !sourcesStatus[idx].Matched {
					secG.Go(func() error {
						if err := res.limiter.Wait(secCtx, ad.Name()); err != nil {
							return nil
						}
						cands, err := ad.Search(secCtx, secQuery)
						if err == nil && len(cands) > 0 {
							sourcesStatus[idx].Matched = true
							sourcesStatus[idx].Success = true
							sourcesStatus[idx].RejectionReason = ""
							crossResolved = true
							candidatesMu.Lock()
							candidates = append(candidates, cands...)
							candidatesMu.Unlock()
						}
						return nil
					})
				}
			}
			_ = secG.Wait()
		}
	}

	totalEvaluated := len(candidates)
	if totalEvaluated == 0 {
		return &ResolveResponse{
			Track:           nil,
			MetadataSources: []string{},
			ProviderStatus:  sourcesStatus,
			ResolutionStrategy: canonical.ResolutionStrategy{
				InputType:     inputType,
				PrimarySource: primarySource,
				CrossResolved: false,
			},
			ResolverStats: ResolverStats{
				ProvidersQueried:     len(activeAdapters),
				ProvidersMatched:     0,
				ProvidersContributed: 0,
				CandidatesEvaluated:  0,
			},
		}, nil
	}

	// Construct evaluation query for unified matcher pipeline
	evalQuery := req.Query
	if evalQuery.Title == "" || evalQuery.Artist == "" {
		initMerged := res.merger.MergeCandidates(candidates)
		if initMerged != nil {
			if evalQuery.Title == "" {
				evalQuery.Title = initMerged.Title
			}
			if evalQuery.Artist == "" {
				evalQuery.Artist = getPrimaryArtist(initMerged.Artists)
			}
			if evalQuery.ISRC == "" {
				evalQuery.ISRC = initMerged.ISRC
			}
		}
	}

	// Stage 4 & 5: Candidate Scoring, Rejection Classification & Identity Confidence Calculation
	scored := res.matcher.ScoreCandidates(candidates, evalQuery)
	acceptedCandidates := make([]canonical.TrackCandidate, 0, len(scored))
	identityMatches := make([]canonical.IdentityMatch, 0, len(scored))

	for _, c := range scored {
		method := "exact_title_artist"
		idConf := identity.CalculateIdentityConfidence(c.MatchScore, c.Provider)
		if c.Track.ISRC != "" {
			method = "isrc"
			idConf = identity.CalculateIdentityConfidence(1.0, "isrc")
		} else if c.Track.DurationMS > 0 {
			method = "title_artist_duration"
		}

		for prov, pID := range c.Track.IDs {
			identityMatches = append(identityMatches, canonical.IdentityMatch{
				Provider:   prov,
				ID:         pID,
				Confidence: idConf,
				Method:     method,
			})
		}

		if c.MatchScore < 0.35 {
			res.logger.Debug("candidate match rejected",
				slog.String("provider", c.Provider),
				slog.Float64("match_score", c.MatchScore),
				slog.String("title", c.Track.Title),
			)
			for idx := range sourcesStatus {
				if strings.EqualFold(sourcesStatus[idx].Name, c.Provider) && !sourcesStatus[idx].Matched {
					sourcesStatus[idx].RejectionReason = "below_threshold"
					sourcesStatus[idx].CandidateScore = c.MatchScore
				}
			}
		} else {
			acceptedCandidates = append(acceptedCandidates, c)
		}
	}

	if len(acceptedCandidates) == 0 {
		acceptedCandidates = scored
	}

	// Stage 6: Final Merge, Post-Enrichment Identity Stability & Canonical ID Generation
	mergedTrack := res.merger.MergeCandidates(acceptedCandidates)
	if mergedTrack != nil {
		mergedTrack.IdentityMatches = deduplicateIdentityMatches(identityMatches)
		isrcCount := len(mergedTrack.FieldSources["isrc"])
		hasMBID := mergedTrack.IDs["musicbrainz"] != ""
		hasISRC := mergedTrack.ISRC != ""

		mergedTrack.IdentityStatus, mergedTrack.IdentityReasons = canonical.GetIdentityStatus(mergedTrack.MatchScore, isrcCount, hasMBID, hasISRC)
		mergedTrack.CanonicalID = res.identityEngine.GenerateID(*mergedTrack)
	}

	metadataSources := make([]string, 0)
	matchedCount := 0
	contributedCount := 0

	if mergedTrack != nil {
		metadataSources = mergedTrack.Sources
		contributedMap := make(map[string]bool)
		for _, src := range mergedTrack.Sources {
			contributedMap[strings.ToLower(src)] = true
		}
		for i := range sourcesStatus {
			if sourcesStatus[i].Matched {
				matchedCount++
			}
			if contributedMap[strings.ToLower(sourcesStatus[i].Name)] {
				sourcesStatus[i].Contributed = true
				contributedCount++
			}
		}
	}

	if primarySource == "" && len(metadataSources) > 0 {
		primarySource = metadataSources[0]
	}

	resp := &ResolveResponse{
		Track:           mergedTrack,
		MetadataSources: metadataSources,
		ProviderStatus:  sourcesStatus,
		ResolutionStrategy: canonical.ResolutionStrategy{
			InputType:     inputType,
			PrimarySource: primarySource,
			CrossResolved: crossResolved,
		},
		ResolverStats: ResolverStats{
			ProvidersQueried:     len(activeAdapters),
			ProvidersMatched:     matchedCount,
			ProvidersContributed: contributedCount,
			CandidatesEvaluated:  totalEvaluated,
		},
	}

	// Write back to cache
	if res.cache != nil && mergedTrack != nil {
		if data, err := json.Marshal(resp); err == nil {
			_ = res.cache.Set(ctx, cacheKey, data, res.ttl)
		}
	}

	return resp, nil
}

func deduplicateIdentityMatches(matches []canonical.IdentityMatch) []canonical.IdentityMatch {
	seen := make(map[string]bool)
	unique := make([]canonical.IdentityMatch, 0, len(matches))
	for _, m := range matches {
		key := fmt.Sprintf("%s:%s", m.Provider, m.ID)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, m)
		}
	}
	return unique
}

func determineInputType(q adapters.Query) string {
	if q.SpotifyID != "" {
		return "spotify_id"
	}
	if q.YouTubeID != "" {
		return "youtube_id"
	}
	if q.DeezerID != "" {
		return "deezer_id"
	}
	if q.AppleID != "" {
		return "apple_id"
	}
	if q.ISRC != "" {
		return "isrc"
	}
	return "artist_title"
}

func getPrimaryArtist(artists []canonical.Artist) string {
	if len(artists) > 0 {
		return artists[0].Name
	}
	return ""
}

func hashQuery(q adapters.Query) string {
	raw := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s", q.SpotifyID, q.YouTubeID, q.DeezerID, q.AppleID, q.ISRC, q.Artist, q.Title)
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
