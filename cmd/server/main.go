package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/ommr/ommr/internal/bootstrap"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ommr/ommr/internal/api"
	"github.com/ommr/ommr/internal/cache"
	"github.com/ommr/ommr/internal/cache/disk"
	"github.com/ommr/ommr/internal/cache/redis"
	"github.com/ommr/ommr/internal/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	configPath := flag.String("config", "", "private YAML configuration file")
	flag.Parse()
	if *configPath != "" {
		_ = os.Setenv("OMMR_CONFIG_FILE", *configPath)
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Error("failed to load configuration", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("starting Open Music Metadata Resolver (OMMR)",
		slog.Int("port", cfg.Port),
		slog.String("cache_provider", cfg.CacheProvider),
	)

	// Initialize Cache Backend
	var cacheBackend cache.Cache
	if cfg.CacheProvider == "redis" {
		rc, err := redis.NewRedisCache(cfg.RedisURL, "ommr:")
		if err != nil {
			logger.Warn("failed to initialize Redis cache, falling back to disk cache", slog.Any("error", err))
			dc, dErr := disk.NewDiskCache(cfg.CacheDir)
			if dErr != nil {
				logger.Error("failed to initialize disk cache fallback", slog.Any("error", dErr))
				os.Exit(1)
			}
			cacheBackend = dc
		} else {
			cacheBackend = rc
		}
	} else {
		dc, err := disk.NewDiskCache(cfg.CacheDir)
		if err != nil {
			logger.Error("failed to initialize disk cache", slog.Any("error", err))
			os.Exit(1)
		}
		cacheBackend = dc
	}
	defer cacheBackend.Close()

	runtime, err := bootstrap.Build(context.Background(), cfg, cacheBackend)
	if err != nil {
		logger.Error("failed to initialize providers", slog.Any("error", err))
		os.Exit(1)
	}
	for _, status := range runtime.Health.Snapshot() {
		logger.Info("provider startup status", slog.String("provider", status.Name), slog.String("state", status.State), slog.String("mode", status.Mode), slog.String("credentials", status.Credentials), slog.String("reason", status.Reason))
	}
	handler := api.NewHandler(runtime.Resolver, runtime.Registry, cfg)
	handler.SetProviderHealth(runtime.Health)

	router := api.NewRouter(handler, logger, cfg.RateLimitRPS)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("server listening", slog.String("addr", srv.Addr))
		serverErrors <- srv.ListenAndServe()
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		logger.Error("error starting HTTP server", slog.Any("error", err))
	case sig := <-shutdown:
		logger.Info("initiating graceful shutdown", slog.String("signal", sig.String()))

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			logger.Error("failed to gracefully shutdown server", slog.Any("error", err))
			_ = srv.Close()
		}
	}
	logger.Info("server stopped gracefully")
}
