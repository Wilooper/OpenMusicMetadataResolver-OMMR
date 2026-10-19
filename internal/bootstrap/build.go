package bootstrap

import (
	"context"
	"fmt"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/adapters/amazonmusic"
	"github.com/ommr/ommr/internal/adapters/applemusic"
	"github.com/ommr/ommr/internal/adapters/deezer"
	"github.com/ommr/ommr/internal/adapters/jiosaavn"
	"github.com/ommr/ommr/internal/adapters/korean"
	"github.com/ommr/ommr/internal/adapters/musicbrainz"
	"github.com/ommr/ommr/internal/adapters/pandora"
	"github.com/ommr/ommr/internal/adapters/qobuz"
	"github.com/ommr/ommr/internal/adapters/soundcloud"
	"github.com/ommr/ommr/internal/adapters/spotify"
	"github.com/ommr/ommr/internal/adapters/tidal"
	"github.com/ommr/ommr/internal/adapters/ytmusic"
	"github.com/ommr/ommr/internal/adapters/ytmusicrich"
	"github.com/ommr/ommr/internal/albums"
	"github.com/ommr/ommr/internal/artists"
	"github.com/ommr/ommr/internal/cache"
	"github.com/ommr/ommr/internal/config"
	"github.com/ommr/ommr/internal/enrichment/catalog"
	"github.com/ommr/ommr/internal/extractors"
	"github.com/ommr/ommr/internal/merger"
	"github.com/ommr/ommr/internal/providerhealth"
	"github.com/ommr/ommr/internal/ratelimit"
	"github.com/ommr/ommr/internal/resolver"
)

type Runtime struct {
	Albums    *albums.Service
	Artists   *artists.Service
	Registry  *adapters.Registry
	Resolver  *resolver.Resolver
	Health    *providerhealth.Manager
	Extractor *extractors.Pool
}

func Build(ctx context.Context, cfg *config.Config, backend cache.Cache) (*Runtime, error) {
	reg := adapters.NewRegistry()
	manager := providerhealth.New(cfg.Lifecycle.FailureThreshold, cfg.Lifecycle.Cooldown)
	limiter := ratelimit.NewProviderLimiter()
	res := resolver.New(reg, backend, limiter, cfg.CacheTTL)
	candidate := func(source adapters.ProviderAdapter, mode string, check func(context.Context) error) Candidate {
		return Candidate{source, mode, func(ctx context.Context) error {
			probe, cancel := context.WithTimeout(ctx, cfg.Lifecycle.ProbeTimeout)
			defer cancel()
			key := source.Name()
			if keyed, ok := source.(interface{ RateLimitKey() string }); ok {
				key = keyed.RateLimitKey()
			}
			if err := limiter.Wait(probe, key); err != nil {
				return err
			}
			return check(probe)
		}}
	}
	plans := []Plan{}
	for _, source := range []adapters.ProviderAdapter{musicbrainz.New(), deezer.New(), soundcloud.New(), jiosaavn.New(), korean.New("bugs"), korean.New("genie"), korean.New("melon")} {
		if fallback, ok := source.(interface {
			SetFallbackWait(func(context.Context) error)
		}); ok {
			key := source.Name()
			fallback.SetFallbackWait(func(ctx context.Context) error { return limiter.Wait(ctx, key) })
		}
		plans = append(plans, Plan{Name: source.Name(), Enabled: cfg.Lifecycle.ProviderEnabled(source.Name()), Public: source})
	}
	sp := Plan{Name: "spotify", Enabled: cfg.Lifecycle.ProviderEnabled("spotify"), Public: spotify.New(spotify.Config{}), Configured: cfg.SpotifyClientID != "" || cfg.SpotifyClientSecret != "" || cfg.SpotifyCookie != "" || cfg.SpotifyAccessToken != ""}
	if cfg.SpotifyClientID != "" && cfg.SpotifyClientSecret != "" {
		source := spotify.New(spotify.Config{ClientID: cfg.SpotifyClientID, ClientSecret: cfg.SpotifyClientSecret})
		sp.Candidates = append(sp.Candidates, candidate(source, "official", source.CheckCredentials))
	}
	if cfg.SpotifyCookie != "" || cfg.SpotifyAccessToken != "" {
		source := spotify.New(spotify.Config{Cookie: cfg.SpotifyCookie, AccessToken: cfg.SpotifyAccessToken, ExpiresAt: cfg.SpotifyTokenExpiry})
		sp.Candidates = append(sp.Candidates, candidate(source, "browser_session", source.CheckCredentials))
	}
	plans = append(plans, sp)
	apple := Plan{Name: "applemusic", Enabled: cfg.Lifecycle.ProviderEnabled("applemusic"), Public: applemusic.NewWithCatalog("", cfg.AppleStorefront), Configured: cfg.AppleDeveloperToken != "" || cfg.AppleUserToken != "" || cfg.AppleCookie != ""}
	if cfg.AppleUserToken != "" {
		source := applemusic.NewWithSession(cfg.AppleDeveloperToken, cfg.AppleUserToken, cfg.AppleStorefront)
		apple.Candidates = append(apple.Candidates, candidate(source, "catalog_session", source.CheckCredentials))
	}
	if cfg.AppleDeveloperToken != "" {
		source := applemusic.NewWithSession(cfg.AppleDeveloperToken, "", cfg.AppleStorefront)
		apple.Candidates = append(apple.Candidates, candidate(source, "catalog", source.CheckCredentials))
	}
	plans = append(plans, apple)
	youtube := Plan{Name: "ytmusic", Enabled: cfg.Lifecycle.ProviderEnabled("ytmusic"), Public: ytmusic.New(), Configured: cfg.YTMusicCookie != ""}
	if youtube.Configured {
		source := ytmusic.NewWithCookie(cfg.YTMusicCookie)
		youtube.Candidates = append(youtube.Candidates, candidate(source, "browser_session", source.CheckCredentials))
	}
	var pythonPool *extractors.Pool
	built := false
	defer func() {
		if !built && pythonPool != nil {
			pythonPool.Close()
		}
	}()
	if cfg.Extractor.Enabled && cfg.Lifecycle.ProviderEnabled("ytmusic") {
		var err error
		pythonPool, err = extractors.New(extractors.Config{Python: cfg.Extractor.Python, Script: cfg.Extractor.Script, Workers: cfg.Extractor.Workers, Timeout: cfg.Extractor.Timeout, TTL: cfg.Extractor.TTL}, backend)
		if err != nil {
			manager.Set("ytmusic_python", "disabled", "public", "absent", "worker_unavailable")
		} else {
			manager.Set("ytmusic_python", "active", "public", "absent", "")
			youtube.Public = &ytmusicrich.Provider{ProviderAdapter: youtube.Public, Pool: pythonPool, Health: manager, Profiles: cfg.Extractor.Profiles}
			for i := range youtube.Candidates {
				youtube.Candidates[i].Source = &ytmusicrich.Provider{ProviderAdapter: youtube.Candidates[i].Source, Pool: pythonPool, Health: manager, Profiles: cfg.Extractor.Profiles}
			}
		}
	}
	plans = append(plans, youtube)
	for _, entry := range []struct {
		source               adapters.ProviderAdapter
		configured, complete bool
	}{
		{qobuz.NewWithConfig(cfg.QobuzBaseURL, cfg.QobuzAppID, cfg.QobuzToken), cfg.QobuzAppID != "" || cfg.QobuzToken != "", cfg.QobuzAppID != ""},
		{tidal.NewWithConfig(cfg.TidalBaseURL, cfg.TidalToken), cfg.TidalToken != "", cfg.TidalToken != ""},
		{amazonmusic.NewWithConfig(cfg.AmazonMusicBaseURL, cfg.AmazonMusicToken, cfg.AmazonMusicAPIKey), cfg.AmazonMusicToken != "" || cfg.AmazonMusicAPIKey != "", cfg.AmazonMusicToken != "" && cfg.AmazonMusicAPIKey != ""},
		{pandora.NewWithConfig(cfg.PandoraBaseURL, cfg.PandoraToken), cfg.PandoraToken != "", cfg.PandoraToken != ""},
	} {
		source := entry.source
		plan := Plan{Name: source.Name(), Enabled: cfg.Lifecycle.ProviderEnabled(source.Name()), Configured: entry.configured}
		if entry.complete {
			check := func(ctx context.Context) error {
				results, err := source.Search(ctx, adapters.Query{Artist: "Rick Astley", Title: "Never Gonna Give You Up"})
				if err != nil {
					return err
				}
				if len(results) == 0 {
					return fmt.Errorf("catalog probe returned no track capability")
				}
				return nil
			}
			if source.Name() == "pandora" {
				check = func(ctx context.Context) error {
					if cfg.PandoraProbeID == "" {
						return fmt.Errorf("Pandora validation requires a known track ID")
					}
					result, err := source.FetchByID(ctx, "pandora", cfg.PandoraProbeID)
					if err != nil {
						return err
					}
					if result == nil {
						return fmt.Errorf("empty Pandora probe")
					}
					return nil
				}
			}
			plan.Candidates = []Candidate{candidate(source, "credentialed", check)}
		}
		plans = append(plans, plan)
	}
	known := map[string]bool{}
	for _, plan := range plans {
		known[plan.Name] = true
	}
	for _, name := range []string{"coverartarchive", "musicbrainz_release", "lastfm", "discogs", "acoustid"} {
		known[name] = true
	}
	for name := range cfg.Lifecycle.Enabled {
		if !known[name] {
			return nil, fmt.Errorf("unknown provider enable flag %q", name)
		}
	}
	for _, name := range cfg.API.DefaultSources {
		if !known[name] {
			return nil, fmt.Errorf("unknown default source %q", name)
		}
	}
	policy, err := merger.ParsePolicy(cfg.MetadataPolicy)
	if err != nil {
		return nil, err
	}
	for _, names := range policy.Priorities {
		for _, name := range names {
			if !known[name] {
				return nil, fmt.Errorf("unknown metadata policy provider %q", name)
			}
		}
	}
	startup, cancel := context.WithTimeout(ctx, cfg.Lifecycle.StartupTimeout)
	defer cancel()
	Activate(startup, plans, cfg.Lifecycle.ProbeWorkers, manager, reg)
	client := catalog.NewClient(res.WaitProvider)
	client.Cache = backend
	// Credential checks deliberately use an uncached client.
	probeClient := catalog.NewClient(res.WaitProvider)
	enrichers := []catalog.Enricher{}
	for _, source := range []catalog.Enricher{catalog.NewCoverArt(client), catalog.NewMusicBrainzRelease(client)} {
		name := source.Name()
		if cfg.EnableEnrichment && cfg.Lifecycle.ProviderEnabled(name) {
			manager.Set(name, "active", "public", "absent", "")
			enrichers = append(enrichers, providerhealth.WrapEnricher(source, manager))
		} else {
			manager.Set(name, "disabled", "off", "unused", "disabled_by_config")
		}
	}
	for _, entry := range []struct {
		name       string
		configured bool
		source     catalog.Enricher
		check      func(context.Context) error
	}{
		{"lastfm", cfg.LastFMKey != "", catalog.NewLastFM(client, cfg.LastFMKey), catalog.NewLastFM(probeClient, cfg.LastFMKey).Validate},
		{"discogs", cfg.DiscogsToken != "", catalog.NewDiscogs(client, cfg.DiscogsToken), catalog.NewDiscogs(probeClient, cfg.DiscogsToken).Validate},
		{"acoustid", cfg.AcoustIDKey != "", nil, catalog.NewAcoustID(probeClient, cfg.AcoustIDKey).Validate},
	} {
		if !cfg.Lifecycle.ProviderEnabled(entry.name) || (!cfg.EnableEnrichment && entry.name != "acoustid") {
			manager.Set(entry.name, "disabled", "off", "unused", "disabled_by_config")
			continue
		}
		if !entry.configured {
			manager.Set(entry.name, "disabled", "off", "absent", "credentials_missing")
			continue
		}
		probe, stop := context.WithTimeout(startup, cfg.Lifecycle.ProbeTimeout)
		err := entry.check(probe)
		stop()
		if err != nil {
			manager.Set(entry.name, "disabled", "off", "rejected", "credential_probe_failed")
			continue
		}
		manager.Set(entry.name, "active", "credentialed", "validated", "")
		if entry.source != nil {
			enrichers = append(enrichers, providerhealth.WrapEnricher(entry.source, manager))
		}
	}
	res.Configure(policy, enrichers...)
	res.SetCacheVariant(fmt.Sprintf("python:%t/profiles:%t", pythonPool != nil, cfg.Extractor.Profiles))
	res.SetProviderHealth(manager)
	built = true
	artistService := artists.New(limiter.Wait, backend)
	artistService.Health = manager
	if pythonPool != nil {
		artistService.Pool = pythonPool
	}
	artistService.Storefront = cfg.AppleStorefront
	artistService.Available = func(provider string) bool {
		_, registered := reg.Get(provider)
		return registered && cfg.Lifecycle.ProviderEnabled(provider) && manager.Available(provider)
	}
	albumService := albums.New(limiter.Wait, backend)
	albumService.Health = manager
	albumService.Storefront = cfg.AppleStorefront
	albumService.Available = artistService.Available
	return &Runtime{Registry: reg, Resolver: res, Health: manager, Extractor: pythonPool, Artists: artistService, Albums: albumService}, nil
}
