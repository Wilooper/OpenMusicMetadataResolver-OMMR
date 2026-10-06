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
	registry                 *adapters.Registry
	cache                    cache.Cache
	limiter                  *ratelimit.ProviderLimiter
	matcher                  *matcher.Matcher
	merger                   *merger.Merger
	identityEngine           *identity.IdentityEngine
	ttl                      time.Duration
	maxCandidatesPerProvider int
	logger                   *slog.Logger
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
	}
}

func (res *Resolver) WaitProvider(ctx context.Context, provider string) error {
	if res == nil || res.limiter == nil {
		return nil
	}
	return res.limiter.Wait(ctx, provider)
}

func (res *Resolver) Resolve(ctx context.Context, req ResolveRequest) (*ResolveResponse, error) {
	// Clean text inputs
	req.Query.Title = query.CleanTitle(req.Query.Title)
	req.Query.Artist = query.CleanArtist(req.Query.Artist)

	inputType := determineInputType(req.Query)
	cacheKey := "resolved:rich-v1:" + hashQuery(req.Query, req.Sources)

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
				}
			} else if req.Query.DeezerID != "" && ad.Name() == "deezer" {
				cand, err := ad.FetchByID(gCtx, "deezer", req.Query.DeezerID)
				fetchErr = err
				if cand != nil {
					fetched = append(fetched, *cand)
				}
			} else if req.Query.AppleID != "" && ad.Name() == "applemusic" {
				cand, err := ad.FetchByID(gCtx, "applemusic", req.Query.AppleID)
				fetchErr = err
				if cand != nil {
					fetched = append(fetched, *cand)
				}
			} else if req.Query.SoundCloudID != "" && ad.Name() == "soundcloud" {
				cand, err := ad.FetchByID(gCtx, "soundcloud", req.Query.SoundCloudID)
				fetchErr = err
				if cand != nil {
					fetched = append(fetched, *cand)
				}
			} else if req.Query.QobuzID != "" && ad.Name() == "qobuz" {
				cand, err := ad.FetchByID(gCtx, "qobuz", req.Query.QobuzID)
				fetchErr = err
				if cand != nil {
					fetched = append(fetched, *cand)
				}
			} else if req.Query.TidalID != "" && ad.Name() == "tidal" {
				cand, err := ad.FetchByID(gCtx, "tidal", req.Query.TidalID)
				fetchErr = err
				if cand != nil {
					fetched = append(fetched, *cand)
				}
			} else if req.Query.AmazonMusicID != "" && ad.Name() == "amazonmusic" {
				cand, err := ad.FetchByID(gCtx, "amazonmusic", req.Query.AmazonMusicID)
				fetchErr = err
				if cand != nil {
					fetched = append(fetched, *cand)
				}
			} else if req.Query.PandoraID != "" && ad.Name() == "pandora" {
				cand, err := ad.FetchByID(gCtx, "pandora", req.Query.PandoraID)
				fetchErr = err
				if cand != nil {
					fetched = append(fetched, *cand)
				}
			} else if req.Query.YouTubeID != "" && ad.Name() == "ytmusic" {
				cand, err := ad.FetchByID(gCtx, "youtube", req.Query.YouTubeID)
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
		if directProvider(req.Query) == "" {
			return true
		}
		const minIdentityComponentScore = 0.72
		return c.Track.MatchBreakdown.Title != nil && *c.Track.MatchBreakdown.Title >= minIdentityComponentScore &&
			c.Track.MatchBreakdown.Artists != nil && *c.Track.MatchBreakdown.Artists >= minIdentityComponentScore
	}

	emitIdentityMatches := func(c canonical.TrackCandidate) {
		method := "exact_title_artist"
		idConf := identity.CalculateIdentityConfidence(c.MatchScore, c.Provider)
		if evalQuery.ISRC != "" && c.Track.ISRC != "" && strings.EqualFold(strings.TrimSpace(evalQuery.ISRC), strings.TrimSpace(c.Track.ISRC)) {
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
	res.enrichAccepted(ctx, acceptedCandidates, sourcesStatus)
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
	if q.SoundCloudID != "" {
		return "soundcloud_id"
	}
	if q.QobuzID != "" {
		return "qobuz_id"
	}
	if q.TidalID != "" {
		return "tidal_id"
	}
	if q.AmazonMusicID != "" {
		return "amazonmusic_id"
	}
	if q.PandoraID != "" {
		return "pandora_id"
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

func directProvider(q adapters.Query) string {
	switch {
	case q.SpotifyID != "":
		return "spotify"
	case q.YouTubeID != "":
		return "ytmusic"
	case q.DeezerID != "":
		return "deezer"
	case q.AppleID != "":
		return "applemusic"
	case q.SoundCloudID != "":
		return "soundcloud"
	case q.QobuzID != "":
		return "qobuz"
	case q.TidalID != "":
		return "tidal"
	case q.AmazonMusicID != "":
		return "amazonmusic"
	case q.PandoraID != "":
		return "pandora"
	default:
		return ""
	}
}

func trustedSeedCandidates(q adapters.Query, candidates []canonical.TrackCandidate) []canonical.TrackCandidate {
	provider := directProvider(q)
	if provider != "" {
		id := ""
		switch provider {
		case "spotify":
			id = q.SpotifyID
		case "ytmusic":
			id = q.YouTubeID
		case "deezer":
			id = q.DeezerID
		case "applemusic":
			id = q.AppleID
		case "soundcloud":
			id = q.SoundCloudID
		case "qobuz":
			id = q.QobuzID
		case "tidal":
			id = q.TidalID
		case "amazonmusic":
			id = q.AmazonMusicID
		case "pandora":
			id = q.PandoraID
		}
		trusted := make([]canonical.TrackCandidate, 0, 1)
		for _, candidate := range candidates {
			if !strings.EqualFold(candidate.Provider, provider) || candidate.Track.IDs[provider] != id {
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
	provider := directProvider(q)
	if provider == "" || !strings.EqualFold(candidate.Provider, provider) {
		return false
	}
	id := map[string]string{
		"spotify":     q.SpotifyID,
		"ytmusic":     q.YouTubeID,
		"deezer":      q.DeezerID,
		"applemusic":  q.AppleID,
		"soundcloud":  q.SoundCloudID,
		"qobuz":       q.QobuzID,
		"tidal":       q.TidalID,
		"amazonmusic": q.AmazonMusicID,
		"pandora":     q.PandoraID,
	}[provider]
	return id != "" && candidate.Track.IDs[provider] == id
}
