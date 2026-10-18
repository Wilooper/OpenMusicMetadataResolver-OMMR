package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ommr/ommr/internal/credentials"
	"github.com/spf13/viper"
)

type APIConfig struct {
	DefaultOperation string          `mapstructure:"default_operation"`
	DefaultSources   []string        `mapstructure:"default_sources"`
	Debug            bool            `mapstructure:"debug"`
	BypassCache      bool            `mapstructure:"bypass_cache"`
	SearchLimit      int             `mapstructure:"search_limit"`
	AllowOverrides   *bool           `mapstructure:"allow_overrides"`
	AllowDebug       *bool           `mapstructure:"allow_debug"`
	Endpoints        map[string]bool `mapstructure:"endpoints"`
}

func (a APIConfig) OverridesAllowed() bool { return a.AllowOverrides == nil || *a.AllowOverrides }
func (a APIConfig) DebugAllowed() bool     { return a.AllowDebug == nil || *a.AllowDebug }
func (a APIConfig) Enabled(name string) bool {
	enabled, exists := a.Endpoints[name]
	return !exists || enabled
}

type LifecycleConfig struct {
	FailureThreshold int             `mapstructure:"failure_threshold"`
	Cooldown         time.Duration   `mapstructure:"cooldown"`
	ProbeTimeout     time.Duration   `mapstructure:"probe_timeout"`
	StartupTimeout   time.Duration   `mapstructure:"startup_timeout"`
	ProbeWorkers     int             `mapstructure:"probe_workers"`
	Enabled          map[string]bool `mapstructure:"enabled"`
}

func (l LifecycleConfig) ProviderEnabled(name string) bool {
	enabled, exists := l.Enabled[name]
	return !exists || enabled
}

func runtimeDefaults(v *viper.Viper) {
	defaults := map[string]any{"extractor.enabled": false, "extractor.python": "python3", "extractor.script": "workers/ytmusic/worker.py", "extractor.workers": 2, "extractor.timeout": "5s", "extractor.ttl": "6h", "extractor.profiles": true, "credentials_dir": "./credentials", "api.default_operation": "resolve", "api.search_limit": 10, "api.allow_overrides": true, "api.allow_debug": true, "providers.failure_threshold": 3, "providers.cooldown": "1m", "providers.probe_timeout": "8s", "providers.startup_timeout": "45s", "providers.probe_workers": 4}
	for key, value := range defaults {
		v.SetDefault(key, value)
	}
}
func discover(v *viper.Viper) error {
	dir := v.GetString("credentials_dir")
	if dir != "" && !filepath.IsAbs(dir) && v.ConfigFileUsed() != "" {
		dir = filepath.Join(filepath.Dir(v.ConfigFileUsed()), dir)
	}
	if dir != "" {
		absolute, err := filepath.Abs(dir)
		if err != nil {
			return fmt.Errorf("invalid credential directory")
		}
		dir = absolute
	}
	values, err := credentials.Discover(dir)
	if err != nil {
		return err
	}
	for key, value := range values {
		_, explicit := os.LookupEnv("OMMR_" + strings.ToUpper(key))
		if !v.IsSet(key) || (!explicit && v.GetString(key) == "") {
			v.SetDefault(key, value)
		}
	}
	return nil
}
func loadRuntime(v *viper.Viper, cfg *Config) error {
	cfg.Extractor = ExtractorConfig{Enabled: v.GetBool("extractor.enabled"), Python: v.GetString("extractor.python"), Script: v.GetString("extractor.script"), Workers: v.GetInt("extractor.workers"), Timeout: v.GetDuration("extractor.timeout"), TTL: v.GetDuration("extractor.ttl"), Profiles: v.GetBool("extractor.profiles")}
	if cfg.Extractor.Enabled && (cfg.Extractor.Workers < 1 || cfg.Extractor.Workers > 8 || cfg.Extractor.Timeout <= 0 || cfg.Extractor.Timeout > 30*time.Second || cfg.Extractor.TTL <= 0) {
		return fmt.Errorf("invalid extractor limits")
	}

	if err := v.UnmarshalKey("api", &cfg.API); err != nil {
		return fmt.Errorf("invalid API configuration")
	}
	// AutomaticEnv is not included by Unmarshal unless the keys are already bound.
	cfg.API.DefaultOperation = v.GetString("api.default_operation")
	cfg.API.DefaultSources = v.GetStringSlice("api.default_sources")
	sources := []string{}
	for _, source := range cfg.API.DefaultSources {
		for _, name := range strings.Split(source, ",") {
			name = strings.ToLower(strings.TrimSpace(name))
			if name != "" {
				sources = append(sources, name)
			}
		}
	}
	cfg.API.DefaultSources = sources
	cfg.API.Debug = v.GetBool("api.debug")
	cfg.API.BypassCache = v.GetBool("api.bypass_cache")
	cfg.API.SearchLimit = v.GetInt("api.search_limit")
	overrides, debug := v.GetBool("api.allow_overrides"), v.GetBool("api.allow_debug")
	cfg.API.AllowOverrides = &overrides
	cfg.API.AllowDebug = &debug
	cfg.Lifecycle = LifecycleConfig{FailureThreshold: v.GetInt("providers.failure_threshold"), Cooldown: v.GetDuration("providers.cooldown"), ProbeTimeout: v.GetDuration("providers.probe_timeout"), StartupTimeout: v.GetDuration("providers.startup_timeout"), ProbeWorkers: v.GetInt("providers.probe_workers"), Enabled: map[string]bool{}}
	if err := v.UnmarshalKey("providers.enabled", &cfg.Lifecycle.Enabled); err != nil {
		return fmt.Errorf("invalid provider enable flags")
	}
	for name := range cfg.API.Endpoints {
		if !map[string]bool{"resolve": true, "links": true, "search": true, "extract": true, "bulk": true, "fingerprint": true, "catalog": true, "artists": true, "health": true, "providers": true, "query": true, "metrics": true}[name] {
			return fmt.Errorf("unknown API endpoint flag %q", name)
		}
	}
	if cfg.API.DefaultOperation == "" {
		cfg.API.DefaultOperation = "resolve"
	}
	operations := map[string]bool{"resolve": true, "links": true, "search": true, "extract": true, "bulk": true, "fingerprint": true, "catalog": true, "artists": true}
	if !operations[cfg.API.DefaultOperation] {
		return fmt.Errorf("invalid API default operation")
	}
	if cfg.API.SearchLimit < 1 || cfg.API.SearchLimit > 50 || cfg.Lifecycle.FailureThreshold < 1 || cfg.Lifecycle.Cooldown < 0 || cfg.Lifecycle.ProbeTimeout <= 0 || cfg.Lifecycle.StartupTimeout <= 0 || cfg.Lifecycle.ProbeWorkers < 1 || cfg.Lifecycle.ProbeWorkers > 32 {
		return fmt.Errorf("invalid API or provider lifecycle limits")
	}
	cfg.PandoraProbeID = v.GetString("providers.pandora_probe_id")
	cfg.SpotifyTokenExpiry = v.GetInt64("spotify_token_expiry")
	cfg.SpotifySessionFile = v.GetString("spotify_session_file")
	cfg.SpotifyCookie = v.GetString("spotify_cookie")
	cfg.SpotifyCookieFile = v.GetString("spotify_cookie_file")
	cfg.SpotifyAccessToken = v.GetString("spotify_access_token")
	cfg.AppleSessionFile = v.GetString("applemusic_session_file")
	cfg.AppleCookie = v.GetString("applemusic_cookie")
	cfg.AppleCookieFile = v.GetString("applemusic_cookie_file")
	cfg.AppleUserToken = v.GetString("apple_music_user_token")
	cfg.YTMusicSessionFile = v.GetString("ytmusic_session_file")
	for _, entry := range []struct {
		sessionFile, cookieFile, domain string
		cookie, access, developer, user *string
	}{
		{cfg.SpotifySessionFile, cfg.SpotifyCookieFile, "spotify.com", &cfg.SpotifyCookie, &cfg.SpotifyAccessToken, nil, nil},
		{cfg.AppleSessionFile, cfg.AppleCookieFile, "apple.com", &cfg.AppleCookie, nil, &cfg.AppleDeveloperToken, &cfg.AppleUserToken},
		{cfg.YTMusicSessionFile, cfg.YTMusicCookieFile, "youtube.com", &cfg.YTMusicCookie, nil, nil, nil},
	} {
		path := entry.sessionFile
		if path == "" {
			path = entry.cookieFile
		}
		if path != "" {
			if !filepath.IsAbs(path) && v.ConfigFileUsed() != "" {
				path = filepath.Join(filepath.Dir(v.ConfigFileUsed()), path)
			}
			session, err := credentials.Load(path, entry.domain)
			if err != nil {
				return err
			}
			cookieKey := "ytmusic_cookie"
			if entry.domain == "spotify.com" {
				cookieKey = "spotify_cookie"
			}
			if entry.domain == "apple.com" {
				cookieKey = "applemusic_cookie"
			}
			_, cookieOverride := os.LookupEnv("OMMR_" + strings.ToUpper(cookieKey))
			if *entry.cookie == "" && !cookieOverride {
				*entry.cookie = session.Cookie
			}
			_, accessOverride := os.LookupEnv("OMMR_SPOTIFY_ACCESS_TOKEN")
			if entry.access != nil && *entry.access == "" && !accessOverride {
				*entry.access = session.AccessToken
				cfg.SpotifyTokenExpiry = session.ExpiresAt
			}
			_, developerOverride := os.LookupEnv("OMMR_APPLE_DEVELOPER_TOKEN")
			if entry.developer != nil && *entry.developer == "" && !developerOverride {
				*entry.developer = session.DeveloperToken
			}
			_, userOverride := os.LookupEnv("OMMR_APPLE_MUSIC_USER_TOKEN")
			if entry.user != nil && *entry.user == "" && !userOverride {
				*entry.user = session.UserToken
			}
		}
	}
	if cfg.AppleCookie != "" && cfg.AppleUserToken == "" {
		session, err := credentials.Parse([]byte(cfg.AppleCookie), "apple.com")
		if err != nil {
			return err
		}
		cfg.AppleUserToken = session.UserToken
	}
	for _, value := range []string{cfg.SpotifyCookie, cfg.SpotifyAccessToken, cfg.SpotifyClientID, cfg.SpotifyClientSecret, cfg.AppleCookie, cfg.AppleUserToken, cfg.AppleDeveloperToken, cfg.YTMusicCookie, cfg.TidalToken, cfg.QobuzToken, cfg.PandoraToken, cfg.AmazonMusicToken, cfg.LastFMKey, cfg.DiscogsToken, cfg.AcoustIDKey} {
		if strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("credential contains an invalid line break")
		}
	}
	return nil
}
