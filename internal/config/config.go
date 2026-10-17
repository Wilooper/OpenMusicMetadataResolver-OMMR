package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	API                APIConfig       `mapstructure:"api"`
	Lifecycle          LifecycleConfig `mapstructure:"providers"`
	SpotifySessionFile string
	SpotifyCookie      string
	SpotifyCookieFile  string
	SpotifyAccessToken string
	SpotifyTokenExpiry int64
	AppleSessionFile   string
	AppleCookie        string
	AppleCookieFile    string
	AppleUserToken     string
	PandoraProbeID     string
	YTMusicSessionFile string
	QobuzAppID         string        `mapstructure:"qobuz_app_id"`
	QobuzToken         string        `mapstructure:"qobuz_token"`
	QobuzBaseURL       string        `mapstructure:"qobuz_base_url"`
	TidalToken         string        `mapstructure:"tidal_token"`
	TidalBaseURL       string        `mapstructure:"tidal_base_url"`
	PandoraToken       string        `mapstructure:"pandora_token"`
	PandoraBaseURL     string        `mapstructure:"pandora_base_url"`
	LastFMKey          string        `mapstructure:"lastfm_api_key"`
	DiscogsToken       string        `mapstructure:"discogs_token"`
	AcoustIDKey        string        `mapstructure:"acoustid_api_key"`
	MetadataPolicy     string        `mapstructure:"metadata_policy"`
	EnableEnrichment   bool          `mapstructure:"enable_enrichment"`
	Port               int           `mapstructure:"port"`
	CacheProvider      string        `mapstructure:"cache_provider"` // "disk" or "redis"
	CacheDir           string        `mapstructure:"cache_dir"`
	CacheTTL           time.Duration `mapstructure:"cache_ttl"`
	RedisURL           string        `mapstructure:"redis_url"`
	RateLimitRPS       float64       `mapstructure:"rate_limit_rps"`
	ProviderTimeout    time.Duration `mapstructure:"provider_timeout"`
	BulkMaxItems       int           `mapstructure:"bulk_max_items"`
	BulkWorkers        int           `mapstructure:"bulk_workers"`

	// Spotify for Developers credentials. When both are set the Spotify
	// adapter uses the official client-credentials OAuth flow; otherwise it
	// falls back to the unofficial anonymous/embed/oEmbed methods.
	SpotifyClientID     string `mapstructure:"spotify_client_id"`
	SpotifyClientSecret string `mapstructure:"spotify_client_secret"`
	AppleDeveloperToken string `mapstructure:"apple_developer_token"`
	AppleStorefront     string `mapstructure:"apple_storefront"`
	YTMusicCookie       string `mapstructure:"ytmusic_cookie"`
	YTMusicCookieFile   string `mapstructure:"ytmusic_cookie_file"`
	AmazonMusicToken    string `mapstructure:"amazon_music_token"`
	AmazonMusicAPIKey   string `mapstructure:"amazon_music_api_key"`
	AmazonMusicBaseURL  string `mapstructure:"amazon_music_base_url"`
}

func LoadConfig() (*Config, error) {
	fileValues, err := loadEnvFile(os.Getenv("OMMR_ENV_FILE"))
	if err != nil {
		return nil, err
	}
	v := viper.New()
	path, explicitConfig := os.LookupEnv("OMMR_CONFIG_FILE")
	if !explicitConfig {
		if _, err := os.Stat("ommr.yaml"); err == nil {
			path = "ommr.yaml"
		}
	}
	if path != "" {
		if err := privateFile(path); err != nil {
			return nil, err
		}
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("configuration file cannot be read or parsed; check its path and syntax")
		}
	}
	v.SetEnvPrefix("OMMR")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	v.AllowEmptyEnv(true)

	runtimeDefaults(v)
	v.SetDefault("port", 8080)
	v.SetDefault("cache_provider", "disk")
	v.SetDefault("cache_dir", "./cache_data")
	v.SetDefault("cache_ttl", "24h")
	v.SetDefault("redis_url", "redis://localhost:6379/0")
	v.SetDefault("rate_limit_rps", 50.0)
	v.SetDefault("provider_timeout", "10s")
	v.SetDefault("enable_enrichment", true)
	v.SetDefault("bulk_max_items", 100)
	v.SetDefault("bulk_workers", 10)
	v.SetDefault("apple_storefront", "us")
	v.SetDefault("qobuz_base_url", "https://www.qobuz.com")
	v.SetDefault("tidal_base_url", "https://openapi.tidal.com")
	v.SetDefault("pandora_base_url", "https://ce.pandora.com/api/v1/graphql")
	for key, value := range fileValues {
		v.SetDefault(key, value)
	}

	if err := discover(v); err != nil {
		return nil, err
	}
	var cfg Config
	cfg.QobuzAppID = v.GetString("qobuz_app_id")
	cfg.QobuzToken = v.GetString("qobuz_token")
	cfg.QobuzBaseURL = v.GetString("qobuz_base_url")
	cfg.TidalToken = v.GetString("tidal_token")
	cfg.TidalBaseURL = v.GetString("tidal_base_url")
	cfg.PandoraToken = v.GetString("pandora_token")
	cfg.PandoraBaseURL = v.GetString("pandora_base_url")
	cfg.LastFMKey = v.GetString("lastfm_api_key")
	cfg.DiscogsToken = v.GetString("discogs_token")
	cfg.AcoustIDKey = v.GetString("acoustid_api_key")
	cfg.MetadataPolicy = v.GetString("metadata_policy")
	cfg.EnableEnrichment = v.GetBool("enable_enrichment")
	cfg.Port = v.GetInt("port")
	cfg.CacheProvider = v.GetString("cache_provider")
	cfg.CacheDir = v.GetString("cache_dir")
	cfg.CacheTTL = v.GetDuration("cache_ttl")
	cfg.RedisURL = v.GetString("redis_url")
	cfg.RateLimitRPS = v.GetFloat64("rate_limit_rps")
	cfg.ProviderTimeout = v.GetDuration("provider_timeout")
	cfg.BulkMaxItems = v.GetInt("bulk_max_items")
	cfg.BulkWorkers = v.GetInt("bulk_workers")
	cfg.SpotifyClientID = v.GetString("spotify_client_id")
	cfg.SpotifyClientSecret = v.GetString("spotify_client_secret")
	cfg.AppleDeveloperToken = v.GetString("apple_developer_token")
	cfg.AppleStorefront = v.GetString("apple_storefront")
	cfg.YTMusicCookie = v.GetString("ytmusic_cookie")
	cfg.YTMusicCookieFile = v.GetString("ytmusic_cookie_file")
	cfg.AmazonMusicToken = v.GetString("amazon_music_token")
	cfg.AmazonMusicAPIKey = v.GetString("amazon_music_api_key")
	cfg.AmazonMusicBaseURL = v.GetString("amazon_music_base_url")
	if cfg.AmazonMusicBaseURL == "" {
		cfg.AmazonMusicBaseURL = "https://api.music.amazon.com"
	}
	if cfg.AmazonMusicBaseURL != "https://api.music.amazon.com" {
		return nil, fmt.Errorf("Amazon Music V2 host must be https://api.music.amazon.com")
	}

	if len(cfg.AppleStorefront) != 2 || strings.Trim(cfg.AppleStorefront, "abcdefghijklmnopqrstuvwxyz") != "" {
		return nil, fmt.Errorf("Apple storefront must be a two-letter lowercase code")
	}
	if err := loadRuntime(v, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func privateFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat private file: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("private file %q must be regular and accessible only by its owner (chmod 600)", path)
	}
	return nil
}

func loadEnvFile(path string) (map[string]string, error) {
	values := make(map[string]string)
	if path == "" {
		return values, nil
	}
	if err := privateFile(path); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open env file: %w", err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || !strings.HasPrefix(key, "OMMR_") {
			return nil, fmt.Errorf("invalid OMMR env file entry")
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '\'' && value[len(value)-1] == '\'' || value[0] == '"' && value[len(value)-1] == '"') {
			value = value[1 : len(value)-1]
		}
		values[strings.ToLower(strings.TrimPrefix(key, "OMMR_"))] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read env file: %w", err)
	}
	return values, nil
}
