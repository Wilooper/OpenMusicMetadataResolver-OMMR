package resolver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/cache"
	"github.com/ommr/ommr/internal/enrichment/catalog"
	"github.com/ommr/ommr/internal/identity"
	"github.com/ommr/ommr/internal/idresolver"
	"github.com/ommr/ommr/internal/matcher"
	"github.com/ommr/ommr/internal/merger"
	"github.com/ommr/ommr/internal/metadata"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/platformlinks"
	"github.com/ommr/ommr/internal/providerhealth"
	"github.com/ommr/ommr/internal/ratelimit"
	"github.com/ommr/ommr/pkg/query"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
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
	EnrichmentStatus   map[string]string            `json:"enrichment_status,omitempty"`
	ResolverStats      ResolverStats                `json:"resolver_stats"`
}

type Resolver struct {
	registry                 *adapters.Registry
	cache                    cache.Cache
	limiter                  *ratelimit.ProviderLimiter
	matcher                  *matcher.Matcher
	merger                   *merger.Merger
	identityEngine           *identity.IdentityEngine
	ttl                      time.Duration
	maxCandidatesPerProvider int
	logger                   *slog.Logger
	health                   *providerhealth.Manager
	flights                  singleflight.Group
	cacheNamespace           string
	enrichers                []catalog.Enricher
}

func New(r *adapters.Registry, c cache.Cache, l *ratelimit.ProviderLimiter, ttl time.Duration) *Resolver {
	return &Resolver{
		registry:                 r,
		cache:                    c,
		limiter:                  l,
		matcher:                  matcher.New(),
		merger:                   merger.New(),
		identityEngine:           identity.New(),
		ttl:                      ttl,
		maxCandidatesPerProvider: 10,
		logger:                   slog.Default(),
		cacheNamespace:           "resolved:rich-v14:default:",
	}
}

func (res *Resolver) WaitProvider(ctx context.Context, provider string) error {
	if res == nil || res.limiter == nil {
		return nil
	}
	if adapter, ok := res.registry.Get(provider); ok {
		if key, ok := adapter.(interface{ RateLimitKey() string }); ok {
			provider = key.RateLimitKey()
		}
	}
	return res.limiter.Wait(ctx, provider)
}

func (res *Resolver) Resolve(ctx context.Context, req ResolveRequest) (*ResolveResponse, error) {
	var err error
	req.Query, err = idresolver.NormalizeQuery(req.Query)
	if err != nil {
		return nil, err
	}
	req.Query.Title = query.CleanTitle(req.Query.Title)
	req.Query.Artist = query.CleanArtist(req.Query.Artist)
	if res.cache == nil {
		return res.resolve(ctx, req)
	}
	key := res.cacheNamespace + res.healthSignature() + hashQuery(req.Query, req.Sources)
	if req.BypassCache {
		key += "/bypass"
	}
	outcome := res.flights.DoChan(key, func() (any, error) {
		// One disconnected waiter must not cancel the shared provider request.
		budget := 15 * time.Second
		if deadline, ok := ctx.Deadline(); ok {
			budget = time.Until(deadline)
			if budget <= 0 {
				return nil, context.DeadlineExceeded
			}
		}
		work, cancel := context.WithTimeout(context.WithoutCancel(ctx), budget)
		defer cancel()
		response, err := res.resolve(work, req)
		if err != nil {
			return nil, err
		}
		return json.Marshal(response)
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-outcome:
		if result.Err != nil {
			return nil, result.Err
		}
		var response ResolveResponse
		if err := json.Unmarshal(result.Val.([]byte), &response); err != nil {
			return nil, err
		}
		return &response, nil
	}
}

func (res *Resolver) resolve(ctx context.Context, req ResolveRequest) (*ResolveResponse, error) {
	// Clean text inputs
	req.Query.Title = query.CleanTitle(req.Query.Title)
	req.Query.Artist = query.CleanArtist(req.Query.Artist)

	inputType := determineInputType(req.Query)
	signature := res.healthSignature()
	cacheKey := res.cacheNamespace + signature + hashQuery(req.Query, req.Sources)

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

	// Reserve time to merge, cache and serialize usable results after slow sources expire.
	lookupCtx := ctx
	if deadline, ok := ctx.Deadline(); ok {
		reserve := min(time.Until(deadline)/5, 500*time.Millisecond)
		var cancel context.CancelFunc
		lookupCtx, cancel = context.WithDeadline(ctx, deadline.Add(-reserve))
		defer cancel()
	}

	// Stage 1: Direct Provider Lookup & Initial Parallel Fetch
	g, gCtx := errgroup.WithContext(lookupCtx)
	candidatesMu := sync.Mutex{}
	candidates := make([]canonical.TrackCandidate, 0, len(activeAdapters))
	sourcesStatus := make([]canonical.SourceStatus, len(activeAdapters))
	var primarySource string

	for i, adapter := range activeAdapters {
		idx := i
		ad := adapter
		g.Go(func() error {
			start := time.Now()
			status := canonical.SourceStatus{
				Name:    ad.Name(),
				Version: ad.Version(),
			}

			if directProvider(req.Query) != "" && ad.Name() != directProvider(req.Query) && req.Query.Title == "" && req.Query.ISRC == "" {
				sourcesStatus[idx] = status
				return nil
			}
			if err := res.WaitProvider(gCtx, ad.Name()); err != nil {
				status.Success = false
				status.Error = err.Error()
				status.RejectionReason = "provider_error"
				status.LatencyMS = time.Since(start).Milliseconds()
				sourcesStatus[idx] = status
				return nil
			}

			var fetched []canonical.TrackCandidate
			var fetchErr error

			if direct := req.Query.DirectInput(); direct.Provider != "" && ad.Name() == direct.Provider {
				cand, err := ad.FetchByID(gCtx, direct.Kind, direct.ID)
				fetchErr = err
				if cand != nil {
					fetched = append(fetched, *cand)
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

	seedCandidates := trustedSeedCandidates(req.Query, candidates)
	if inputType != "artist_title" {
		candidates = seedCandidates
		trustedProviders := make(map[string]bool, len(seedCandidates))
		for _, candidate := range seedCandidates {
			trustedProviders[strings.ToLower(candidate.Provider)] = true
		}
		for i := range sourcesStatus {
			name := strings.ToLower(sourcesStatus[i].Name)
			if sourcesStatus[i].Matched && !trustedProviders[name] {
				sourcesStatus[i].Matched = false
				sourcesStatus[i].RejectionReason = "unverified_candidate"
			}
		}
	}
	seedTrack := res.merger.MergeCandidates(seedCandidates)

	directSource := directProvider(req.Query)
	if directSource != "" {
		for _, status := range sourcesStatus {
			if strings.EqualFold(status.Name, directSource) && status.Matched {
				primarySource = directSource
				break
			}
		}
	}

	// Stage 2 & 3: ISRC & Cleaned Metadata Cross-Platform ID Discovery
	var crossResolved atomic.Bool
	if seedTrack != nil && len(candidates) > 0 && inputType != "artist_title" {
		if seedTrack.Title != "" || seedTrack.ISRC != "" {
			cleanTitle := query.CleanTitle(seedTrack.Title)
			cleanArtist := query.CleanArtist(getPrimaryArtist(seedTrack.Artists))

			secQuery := adapters.Query{
				Title:  cleanTitle,
				Artist: cleanArtist,
				Album:  query.CleanTitle(seedTrack.Album.Title),
				ISRC:   seedTrack.ISRC,
			}

			secG, secCtx := errgroup.WithContext(lookupCtx)
			for i, adapter := range activeAdapters {
				idx := i
				ad := adapter
				if !sourcesStatus[idx].Matched {
					secG.Go(func() error {
						start := time.Now()
						defer func() { sourcesStatus[idx].LatencyMS += time.Since(start).Milliseconds() }()
						if err := res.WaitProvider(secCtx, ad.Name()); err != nil {
							sourcesStatus[idx].Success = false
							sourcesStatus[idx].Error = err.Error()
							sourcesStatus[idx].RejectionReason = "provider_error"
							return nil
						}
						cands, err := ad.Search(secCtx, secQuery)
						if err != nil {
							sourcesStatus[idx].Success = false
							sourcesStatus[idx].Error = err.Error()
							sourcesStatus[idx].RejectionReason = "provider_error"
							return nil
						}
						sourcesStatus[idx].Error = ""
						sourcesStatus[idx].Success = true
						if len(cands) == 0 {
							sourcesStatus[idx].RejectionReason = "no_results"
						}
						if err == nil && len(cands) > 0 {
							if len(cands) > res.maxCandidatesPerProvider {
								cands = cands[:res.maxCandidatesPerProvider]
							}
							sourcesStatus[idx].Matched = true
							sourcesStatus[idx].Success = true
							sourcesStatus[idx].RejectionReason = ""
							crossResolved.Store(true)
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
		empty := &ResolveResponse{
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
		}
		healthy := true
		for _, status := range sourcesStatus {
			if !status.Success {
				healthy = false
			}
		}
		if healthy && res.cache != nil {
			if data, err := json.Marshal(empty); err == nil {
				_ = res.cache.Set(ctx, cacheKey, data, 30*time.Second)
			}
		}
		return empty, nil
	}

	// Construct evaluation query for unified matcher pipeline
	evalQuery := req.Query
	if seedTrack != nil {
		if evalQuery.Title == "" {
			evalQuery.Title = seedTrack.Title
		}
		if evalQuery.Artist == "" {
			evalQuery.Artist = getPrimaryArtist(seedTrack.Artists)
		}
		if inputType != "artist_title" {
			if evalQuery.Album == "" {
				evalQuery.Album = seedTrack.Album.Title
			}
			if evalQuery.ISRC == "" {
				evalQuery.ISRC = seedTrack.ISRC
			}
		}
	}

	// Stage 4 & 5: Candidate Scoring, Rejection Classification & Identity Confidence Calculation
	scored := res.matcher.ScoreCandidates(candidates, evalQuery)
	acceptedCandidates := make([]canonical.TrackCandidate, 0, len(scored))
	identityMatches := make([]canonical.IdentityMatch, 0, len(scored))

	// A candidate below this score is never merged into the result. The
	// single best candidate is only surfaced (see below) when it still clears
	// an absolute minimum, preventing unrelated tracks from polluting IDs.
	const (
		rejectThreshold    = 0.40
		minReportableScore = 0.30
		minArtistScore     = 0.45
	)

	// When the query carries an explicit artist, reject title-only matches on a
	// different artist (e.g. another "Get Lucky"). This stops wrong-artist
	// candidates from leaking their IDs/metadata even when their combined score
	// clears the overall threshold.
	artistGate := func(c canonical.TrackCandidate) bool {
		if strings.TrimSpace(evalQuery.Artist) == "" {
			return true
		}
		if directProvider(req.Query) == "" && evalQuery.ISRC == "" && !hasArtistEvidence(evalQuery.Artist, c.Track.Artists) {
			return false
		}
		if c.Track.MatchBreakdown.Artists == nil {
			return true
		}
		return *c.Track.MatchBreakdown.Artists >= minArtistScore
	}
	strictIdentityGate := func(c canonical.TrackCandidate) bool {
		if evalQuery.ISRC != "" {
			return c.Track.ISRC != "" && strings.EqualFold(strings.TrimSpace(c.Track.ISRC), strings.TrimSpace(evalQuery.ISRC))
		}
		if candidateMatchesDirectID(evalQuery, c) {
			return true
		}
		if !compatibleVersion(evalQuery.Title, c.Track.Title) {
			return false
		}
		const minIdentityComponentScore = 0.72
		// Artist and album agreement cannot identify a different song on the
		// same release. Apply title evidence to text queries as well as IDs.
		if strings.TrimSpace(evalQuery.Title) != "" && !hasTitleEvidence(evalQuery.Title, c) {
			return false
		}
		if directProvider(req.Query) == "" {
			return true
		}
		return c.Track.MatchBreakdown.Title != nil && *c.Track.MatchBreakdown.Title >= minIdentityComponentScore &&
			c.Track.MatchBreakdown.Artists != nil && *c.Track.MatchBreakdown.Artists >= minIdentityComponentScore
	}

	emitIdentityMatches := func(c canonical.TrackCandidate) {
		method := "exact_title_artist"
		idConf := identity.CalculateIdentityConfidence(c.MatchScore, c.Provider)
		if candidateMatchesDirectID(req.Query, c) {
			method = "direct_id"
		} else if evalQuery.ISRC != "" && c.Track.ISRC != "" && strings.EqualFold(strings.TrimSpace(evalQuery.ISRC), strings.TrimSpace(c.Track.ISRC)) {
			method = "isrc"
			idConf = identity.CalculateIdentityConfidence(1.0, "isrc")
		} else if c.Track.DurationMS > 0 {
			method = "title_artist_duration"
		}

		for prov, pID := range c.Track.IDs {
			if prov != c.Provider || pID == "" {
				continue
			}
			identityMatches = append(identityMatches, canonical.IdentityMatch{
				Provider:   prov,
				ID:         pID,
				Confidence: idConf,
				Method:     method,
			})
		}
	}

	bestCandidate := canonical.TrackCandidate{MatchScore: -1}
	bestScoreByProvider := make(map[string]float64)
	acceptedByProvider := make(map[string]int)
	identityRejectedByProvider := make(map[string]bool)
	for i, c := range scored {
		if c.MatchScore > bestCandidate.MatchScore {
			bestCandidate = c
		}
		provider := strings.ToLower(c.Provider)
		if c.MatchScore > bestScoreByProvider[provider] {
			bestScoreByProvider[provider] = c.MatchScore
		}

		if c.MatchScore < rejectThreshold || !artistGate(c) {
			res.logger.Debug("candidate match rejected",
				slog.String("provider", c.Provider),
				slog.Float64("match_score", c.MatchScore),
				slog.String("title", c.Track.Title),
			)
			continue
		}
		if !strictIdentityGate(c) {
			identityRejectedByProvider[provider] = true
			res.logger.Debug("candidate identity rejected",
				slog.String("provider", c.Provider),
				slog.Float64("match_score", c.MatchScore),
				slog.String("title", c.Track.Title),
			)
			continue
		}

		previousIndex, exists := acceptedByProvider[provider]
		if !exists || c.MatchScore > scored[previousIndex].MatchScore {
			acceptedByProvider[provider] = i
		}
	}

	for i, c := range scored {
		if selectedIndex, ok := acceptedByProvider[strings.ToLower(c.Provider)]; ok && selectedIndex == i {
			acceptedCandidates = append(acceptedCandidates, c)
			emitIdentityMatches(c)
		}
	}

	// If no candidate cleared the acceptance threshold, surface only the single
	// highest-scoring candidate (still guarded by an absolute minimum). This
	// avoids merging a pool of mutually-inconsistent, sub-threshold tracks and
	// returning track/video IDs that do not match what was requested.
	if len(acceptedCandidates) == 0 && bestCandidate.MatchScore >= minReportableScore && artistGate(bestCandidate) && strictIdentityGate(bestCandidate) {
		emitIdentityMatches(bestCandidate)
		acceptedCandidates = append(acceptedCandidates, bestCandidate)
	}

	// Derive per-provider status: a provider is "matched" when at least one of
	// its candidates was accepted; otherwise it is rejected as below_threshold.
	acceptedProviderNames := make(map[string]bool)
	for _, c := range acceptedCandidates {
		acceptedProviderNames[strings.ToLower(c.Provider)] = true
	}
	for idx := range sourcesStatus {
		name := strings.ToLower(sourcesStatus[idx].Name)
		if acceptedProviderNames[name] {
			sourcesStatus[idx].Matched = true
			sourcesStatus[idx].RejectionReason = ""
			sourcesStatus[idx].CandidateScore = 0
		} else if _, ok := bestScoreByProvider[name]; ok {
			sourcesStatus[idx].Matched = false
			sourcesStatus[idx].RejectionReason = "below_threshold"
			if identityRejectedByProvider[name] {
				sourcesStatus[idx].RejectionReason = "identity_mismatch"
			}
			sourcesStatus[idx].CandidateScore = bestScoreByProvider[name]
		}
	}

	// Stage 6: Final Merge, Post-Enrichment Identity Stability & Canonical ID Generation
	res.enrichAccepted(lookupCtx, acceptedCandidates, sourcesStatus)
	mergedTrack := res.merger.MergeCandidates(acceptedCandidates)
	if mergedTrack != nil {
		mergedTrack.IdentityMatches = deduplicateIdentityMatches(identityMatches)
		isrcCount := len(mergedTrack.FieldSources["isrc"])
		hasMBID := mergedTrack.IDs["musicbrainz"] != ""
		hasISRC := mergedTrack.ISRC != ""

		mergedTrack.IdentityStatus, mergedTrack.IdentityReasons = canonical.GetIdentityStatus(mergedTrack.MatchScore, isrcCount, hasMBID, hasISRC)
		mergedTrack.CanonicalID = res.identityEngine.GenerateID(*mergedTrack)
		platformlinks.Attach(mergedTrack)
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

	metadata.Attach(mergedTrack, acceptedCandidates)
	enrichmentStatuses := res.enrichCatalogs(lookupCtx, mergedTrack)
	metadata.AttachGraph(mergedTrack, acceptedCandidates)
	resp := &ResolveResponse{
		EnrichmentStatus: enrichmentStatuses,
		Track:            mergedTrack,
		MetadataSources:  metadataSources,
		ProviderStatus:   sourcesStatus,
		ResolutionStrategy: canonical.ResolutionStrategy{
			InputType:     inputType,
			PrimarySource: primarySource,
			CrossResolved: crossResolved.Load(),
		},
		ResolverStats: ResolverStats{
			ProvidersQueried:     len(activeAdapters),
			ProvidersMatched:     matchedCount,
			ProvidersContributed: contributedCount,
			CandidatesEvaluated:  totalEvaluated,
		},
	}

	// Write back to cache
	if res.cache != nil && mergedTrack != nil && signature == res.healthSignature() {
		if data, err := json.Marshal(resp); err == nil {
			ttl := res.ttl
			for _, status := range sourcesStatus {
				if (!status.Success || status.EnrichmentStatus == "partial" || status.EnrichmentStatus == "unavailable" || status.EnrichmentStatus == "identity_mismatch") && ttl > 30*time.Second {
					ttl = 30 * time.Second
				}
			}
			for _, status := range enrichmentStatuses {
				if status == "unavailable" && ttl > 30*time.Second {
					ttl = 30 * time.Second
				}
			}
			_ = res.cache.Set(ctx, cacheKey, data, ttl)
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
	if direct := q.DirectInput(); direct.Provider != "" {
		return direct.Field
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

func hashQuery(q adapters.Query, sources []string) string {
	normalizedSources := make([]string, 0, len(sources))
	seen := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		source = strings.ToLower(strings.TrimSpace(source))
		if source == "" {
			continue
		}
		if _, ok := seen[source]; ok {
			continue
		}
		seen[source] = struct{}{}
		normalizedSources = append(normalizedSources, source)
	}
	sort.Strings(normalizedSources)
	raw, _ := json.Marshal(struct {
		Query   adapters.Query `json:"query"`
		Sources []string       `json:"sources"`
	}{Query: q, Sources: normalizedSources})
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func directProvider(q adapters.Query) string { return q.DirectInput().Provider }

func trustedSeedCandidates(q adapters.Query, candidates []canonical.TrackCandidate) []canonical.TrackCandidate {
	provider := directProvider(q)
	if provider != "" {

		trusted := make([]canonical.TrackCandidate, 0, 1)
		for _, candidate := range candidates {
			if !adapters.DirectIDMatch(q, candidate) {
				continue
			}
			trusted = append(trusted, candidate)
		}
		return trusted
	}

	if q.ISRC == "" {
		return candidates
	}
	trusted := make([]canonical.TrackCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if strings.EqualFold(strings.TrimSpace(candidate.Track.ISRC), strings.TrimSpace(q.ISRC)) {
			trusted = append(trusted, candidate)
		}
	}
	return trusted
}

func candidateMatchesDirectID(q adapters.Query, candidate canonical.TrackCandidate) bool {
	return adapters.DirectIDMatch(q, candidate)
}

func (res *Resolver) SetProviderHealth(manager *providerhealth.Manager) { res.health = manager }
func (res *Resolver) healthSignature() string {
	if res.health == nil {
		return ""
	}
	sum := sha256.Sum256([]byte(res.health.Signature()))
	return hex.EncodeToString(sum[:8]) + ":"
}
