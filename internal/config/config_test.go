package config

import (
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Port)
	}
	if cfg.CacheProvider != "disk" {
		t.Errorf("expected default cache provider 'disk', got %s", cfg.CacheProvider)
	}
	if cfg.BulkMaxItems != 100 {
		t.Errorf("expected bulk max items 100, got %d", cfg.BulkMaxItems)
	}
	if cfg.BulkWorkers != 10 {
		t.Errorf("expected bulk workers 10, got %d", cfg.BulkWorkers)
	}
}
