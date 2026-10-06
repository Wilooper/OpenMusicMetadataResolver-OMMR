package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/config"
	"github.com/ommr/ommr/internal/identity"
	"github.com/ommr/ommr/internal/idresolver"
	"github.com/ommr/ommr/internal/matcher"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/resolver"
	"github.com/ommr/ommr/pkg/query"
)

type Handler struct {
	resolver   *resolver.Resolver
	registry   *adapters.Registry
	idResolver *idresolver.IDResolver
	idEngine   *identity.IdentityEngine
	matcher    *matcher.Matcher
	cfg        *config.Config
	startTime  time.Time
}

func NewHandler(res *resolver.Resolver, reg *adapters.Registry, cfg *config.Config) *Handler {
	return &Handler{
		resolver:   res,
		registry:   reg,
		idResolver: idresolver.New(),
		idEngine:   identity.New(),
		matcher:    matcher.New(),
		cfg:        cfg,
		startTime:  time.Now(),
	}
}

// HandleResolve handles GET /v1/resolve
func (h *Handler) HandleResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	spotifyID := h.idResolver.ExtractSpotifyID(q.Get("spotify_id"))
	youtubeID := h.idResolver.ExtractYouTubeID(q.Get("youtube_id"))
	deezerID := q.Get("deezer_id")
	appleID := q.Get("apple_id")
	soundcloudID := h.idResolver.ExtractSoundCloudID(q.Get("soundcloud_id"))
	qobuzID := q.Get("qobuz_id")
	tidalID := q.Get("tidal_id")
	amazonMusicID := q.Get("amazonmusic_id")
	if amazonMusicID == "" {
		amazonMusicID = q.Get("amazon_music_id")
	}
	pandoraID := q.Get("pandora_id")
	isrc := q.Get("isrc")
	artist := q.Get("artist")
	title := q.Get("title")
	album := q.Get("album")

	if spotifyID == "" && youtubeID == "" && deezerID == "" && appleID == "" && soundcloudID == "" && qobuzID == "" && tidalID == "" && amazonMusicID == "" && pandoraID == "" && isrc == "" && (artist == "" || title == "") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"Provide a supported provider ID, isrc, or both artist and title."}`))
		return
	}

	var sources []string
	if srcParam := q.Get("sources"); srcParam != "" {
		sources = strings.Split(srcParam, ",")
	}

	bypassCache, _ := strconv.ParseBool(q.Get("bypass_cache"))

	req := resolver.ResolveRequest{
		Query: adapters.Query{
			SpotifyID:     spotifyID,
			YouTubeID:     youtubeID,
			DeezerID:      deezerID,
			AppleID:       appleID,
			SoundCloudID:  soundcloudID,
			QobuzID:       qobuzID,
			TidalID:       tidalID,
			AmazonMusicID: amazonMusicID,
			PandoraID:     pandoraID,
			ISRC:          isrc,
			Artist:        artist,
			Title:         title,
			Album:         album,
		},
		Sources:     sources,
		BypassCache: bypassCache,
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.ProviderTimeout)
	defer cancel()

	resp, err := h.resolver.Resolve(ctx, req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"error":%q}`, err.Error())))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// HandleBulk handles POST /v1/bulk
func (h *Handler) HandleBulk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	const maxBulkRequestBytes = 1 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxBulkRequestBytes)
	decoder := json.NewDecoder(r.Body)
	var req BulkResolveRequest
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid JSON payload"}`, http.StatusBadRequest)
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		http.Error(w, `{"error":"Request body must contain a single JSON object"}`, http.StatusBadRequest)
		return
	}

	if len(req.Queries) == 0 {
		http.Error(w, `{"error":"Queries array cannot be empty"}`, http.StatusBadRequest)
		return
	}

	if len(req.Queries) > h.cfg.BulkMaxItems {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"error":"Bulk resolution exceeds maximum limit of %d items"}`, h.cfg.BulkMaxItems)))
		return
	}

	results := make([]BulkItemResult, len(req.Queries))
	workers := h.cfg.BulkWorkers
	if workers <= 0 {
		workers = 10
	}

	jobs := make(chan int, len(req.Queries))
	var wg sync.WaitGroup

	for wIdx := 0; wIdx < workers; wIdx++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				itemQuery := req.Queries[idx]
				itemQuery.SpotifyID = h.idResolver.ExtractSpotifyID(itemQuery.SpotifyID)
				itemQuery.YouTubeID = h.idResolver.ExtractYouTubeID(itemQuery.YouTubeID)
				itemQuery.SoundCloudID = h.idResolver.ExtractSoundCloudID(itemQuery.SoundCloudID)

				resReq := resolver.ResolveRequest{
					Query:   itemQuery,
					Sources: req.Sources,
				}

				ctx, cancel := context.WithTimeout(r.Context(), h.cfg.ProviderTimeout)
				res, err := h.resolver.Resolve(ctx, resReq)
				cancel()

				if err != nil {
					results[idx] = BulkItemResult{
						QueryIndex: idx,
						Success:    false,
						Error:      err.Error(),
					}
				} else {
					results[idx] = BulkItemResult{
						QueryIndex: idx,
						Success:    res.Track != nil,
						Track:      res.Track,
						Sources:    res.ProviderStatus,
					}
				}
			}
		}()
	}

	for i := range req.Queries {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	successful := 0
	failed := 0
	for _, res := range results {
		if res.Success {
			successful++
		} else {
			failed++
		}
	}

	resp := BulkResolveResponse{
		Results:    results,
		Total:      len(results),
		Successful: successful,
		Failed:     failed,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// HandleSearch handles GET /v1/search?q=query&page=1&limit=10
func (h *Handler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	rawQ := r.URL.Query().Get("q")
	if strings.TrimSpace(rawQ) == "" {
		http.Error(w, `{"error":"Query parameter 'q' is required"}`, http.StatusBadRequest)
		return
	}

	cleanedQ := query.CleanTitle(rawQ)

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 50 {
		limit = 10
	}

	var sources []string
	if srcParam := r.URL.Query().Get("sources"); srcParam != "" {
		sources = strings.Split(srcParam, ",")
	}

	activeAdapters, err := h.registry.ListActive(sources)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusBadRequest)
		return
	}

	searchQuery := adapters.Query{Title: cleanedQ}
	allResults := make([]canonical.SearchResult, 0)
	var mu sync.Mutex
	var wg sync.WaitGroup

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.ProviderTimeout)
	defer cancel()

	for _, adapter := range activeAdapters {
		ad := adapter
		wg.Add(1)
		go func() {
			defer wg.Done()
			cands, err := ad.Search(ctx, searchQuery)
			if err == nil && len(cands) > 0 {
				scored := h.matcher.ScoreCandidates(cands, searchQuery)
				mu.Lock()
				for _, c := range scored {
					avail := make([]string, 0, len(c.Track.IDs))
					for k := range c.Track.IDs {
						avail = append(avail, k)
					}
					if len(avail) == 0 {
						avail = []string{c.Provider}
					}
					resItem := canonical.SearchResult{
						CanonicalID: h.idEngine.GenerateID(c.Track),
						Title:       c.Track.Title,
						Artists:     c.Track.Artists,
						Album:       c.Track.Album.Title,
						Images:      c.Track.Images,
						MatchScore:  c.MatchScore,
						Confidence:  c.Track.Confidence,
						SourceCount: len(avail),
						AvailableOn: avail,
					}
					allResults = append(allResults, resItem)
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Sort results descending by match_score
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].MatchScore > allResults[j].MatchScore
	})

	// Paginate results
	total := len(allResults)
	start := (page - 1) * limit
	end := start + limit

	var paginated []canonical.SearchResult
	if start < total {
		if end > total {
			end = total
		}
		paginated = allResults[start:end]
	} else {
		paginated = []canonical.SearchResult{}
	}

	resp := SearchResponse{
		Query:   cleanedQ,
		Results: paginated,
		Page:    page,
		Limit:   limit,
		Total:   total,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// HandleHealth handles GET /v1/health
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	resp := HealthResponse{
		Status:        "healthy",
		Version:       "1.0.0",
		UptimeSeconds: int64(time.Since(h.startTime).Seconds()),
		CacheProvider: h.cfg.CacheProvider,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
