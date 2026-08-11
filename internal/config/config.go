package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Port            int           `mapstructure:"port"`
	CacheProvider   string        `mapstructure:"cache_provider"` // "disk" or "redis"
	CacheDir        string        `mapstructure:"cache_dir"`
	CacheTTL        time.Duration `mapstructure:"cache_ttl"`
	RedisURL        string        `mapstructure:"redis_url"`
	RateLimitRPS    float64       `mapstructure:"rate_limit_rps"`
	ProviderTimeout time.Duration `mapstructure:"provider_timeout"`
	BulkMaxItems    int           `mapstructure:"bulk_max_items"`
	BulkWorkers     int           `mapstructure:"bulk_workers"`
}

func LoadConfig() (*Config, error) {
	v := viper.New()
	v.SetEnvPrefix("OMMR")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("port", 8080)
	v.SetDefault("cache_provider", "disk")
	v.SetDefault("cache_dir", "./cache_data")
	v.SetDefault("cache_ttl", "24h")
	v.SetDefault("redis_url", "redis://localhost:6379/0")
	v.SetDefault("rate_limit_rps", 50.0)
	v.SetDefault("provider_timeout", "3s")
	v.SetDefault("bulk_max_items", 100)
	v.SetDefault("bulk_workers", 10)

	var cfg Config
	cfg.Port = v.GetInt("port")
	cfg.CacheProvider = v.GetString("cache_provider")
	cfg.CacheDir = v.GetString("cache_dir")
	cfg.CacheTTL = v.GetDuration("cache_ttl")
	cfg.RedisURL = v.GetString("redis_url")
	cfg.RateLimitRPS = v.GetFloat64("rate_limit_rps")
	cfg.ProviderTimeout = v.GetDuration("provider_timeout")
	cfg.BulkMaxItems = v.GetInt("bulk_max_items")
	cfg.BulkWorkers = v.GetInt("bulk_workers")

	return &cfg, nil
}
