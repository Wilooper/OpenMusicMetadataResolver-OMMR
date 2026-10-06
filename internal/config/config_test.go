package config

import (
	"os"
	"path/filepath"
	"strings"
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
		t.Errorf("expected default cache provider disk, got %s", cfg.CacheProvider)
	}
	if cfg.BulkMaxItems != 100 {
		t.Errorf("expected bulk max items 100, got %d", cfg.BulkMaxItems)
	}
	if cfg.BulkWorkers != 10 {
		t.Errorf("expected bulk workers 10, got %d", cfg.BulkWorkers)
	}
}

func TestPrivateConfigAndEnvironmentPrecedence(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("OMMR_APPLE_STOREFRONT=gb\nOMMR_APPLE_DEVELOPER_TOKEN=private-token\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OMMR_ENV_FILE", path)
	t.Setenv("OMMR_APPLE_STOREFRONT", "us")
	got, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got.AppleStorefront != "us" || got.AppleDeveloperToken != "private-token" {
		t.Fatalf("wrong precedence or missing token")
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	_, err = LoadConfig()
	if err == nil || !strings.Contains(err.Error(), "chmod 600") || strings.Contains(err.Error(), "private-token") {
		t.Fatalf("expected private-file error without secret: %v", err)
	}
}

func TestInvalidCredentialsFailWithoutEchoingSecrets(t *testing.T) {
	for _, cookie := range []string{"SID=private-secret", "SAPISID=private-secret\r\nInjected: bad"} {
		t.Setenv("OMMR_YTMUSIC_COOKIE", cookie)
		_, err := LoadConfig()
		if err == nil || strings.Contains(err.Error(), "private-secret") {
			t.Fatalf("invalid cookie was not safely rejected: %v", err)
		}
	}
}

func TestEmptyProcessEnvironmentDisablesFileCredential(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("OMMR_APPLE_DEVELOPER_TOKEN=file-token\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OMMR_ENV_FILE", path)
	t.Setenv("OMMR_APPLE_DEVELOPER_TOKEN", "")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppleDeveloperToken != "" {
		t.Fatal("empty process credential did not override file")
	}
}
