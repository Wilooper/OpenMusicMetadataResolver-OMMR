package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestSponsorBlockTimeoutConfiguration(t *testing.T) {
	for _, tc := range []struct {
		value string
		valid bool
	}{
		{"100ms", true}, {"10s", true}, {"30s", true}, {"0s", false}, {"50ms", false}, {"31s", false}, {"invalid", false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("OMMR_SPONSORBLOCK_TIMEOUT", tc.value)
			cfg, err := LoadConfig()
			if (err == nil) != tc.valid {
				t.Fatalf("validation: %v", err)
			}
			if tc.valid {
				expected, _ := time.ParseDuration(tc.value)
				if cfg.SponsorBlockTimeout != expected {
					t.Fatal("timeout override lost")
				}
			}
		})
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
	for _, cookie := range []string{"SAPISID=private-secret\r\nInjected: bad"} {
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

func TestNewCatalogCredentialsLoadFromPrivateEnvFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.env")
	if err := os.WriteFile(path, []byte("OMMR_LASTFM_API_KEY=lastfm-test\nOMMR_DISCOGS_TOKEN=discogs-test\nOMMR_ACOUSTID_API_KEY=acoustid-test\nOMMR_QOBUZ_APP_ID=qobuz-test\nOMMR_TIDAL_TOKEN=tidal-test\nOMMR_PANDORA_TOKEN=pandora-test\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OMMR_ENV_FILE", path)
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LastFMKey != "lastfm-test" || cfg.DiscogsToken != "discogs-test" || cfg.AcoustIDKey != "acoustid-test" || cfg.QobuzAppID != "qobuz-test" || cfg.TidalToken != "tidal-test" || cfg.PandoraToken != "pandora-test" {
		t.Fatal("private-file credentials lost")
	}
}

func TestMalformedCredentialCanReachStartupFallback(t *testing.T) {
	t.Setenv("OMMR_YTMUSIC_COOKIE", "SID=private-secret")
	cfg, err := LoadConfig()
	if err != nil || cfg.YTMusicCookie == "" {
		t.Fatal("credential validity belongs to startup probe, not config load")
	}
}

func TestConfigurationDiscoversSessionsAndControlsAPI(t *testing.T) {
	dir := t.TempDir()
	credentialsDir := filepath.Join(dir, "credentials")
	os.Mkdir(credentialsDir, 0700)
	os.WriteFile(filepath.Join(credentialsDir, "spotify.json"), []byte(`{"cookie":"sp_dc=session-cookie","access_token":"session-bearer","expires_at":2000000000}`), 0600)
	os.WriteFile(filepath.Join(credentialsDir, "applemusic.json"), []byte(`{"headers":{"Cookie":"media-user-token=apple-user","Authorization":"Bearer apple-developer"}}`), 0600)
	os.WriteFile(filepath.Join(credentialsDir, "lastfm.json"), []byte(`{"api_key":"lastfm-key"}`), 0600)
	path := filepath.Join(dir, "ommr.yaml")
	os.WriteFile(path, []byte("credentials_dir: ./credentials\napi:\n  default_operation: links\n  default_sources: [Spotify, deezer]\n  allow_debug: false\n  allow_overrides: false\n  endpoints:\n    search: false\nproviders:\n  failure_threshold: 4\n  cooldown: 2m\n  enabled:\n    tidal: false\n"), 0600)
	t.Setenv("OMMR_CONFIG_FILE", path)
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SpotifyCookie != "sp_dc=session-cookie" || cfg.SpotifyAccessToken != "session-bearer" || cfg.AppleUserToken != "apple-user" || cfg.AppleDeveloperToken != "apple-developer" || cfg.LastFMKey != "lastfm-key" {
		t.Fatal("credential directory discovery failed")
	}
	if cfg.API.DefaultOperation != "links" || cfg.API.DebugAllowed() || cfg.API.OverridesAllowed() || cfg.API.Enabled("search") || cfg.Lifecycle.ProviderEnabled("tidal") || cfg.Lifecycle.FailureThreshold != 4 {
		t.Fatal("startup API policy not applied")
	}
	t.Setenv("OMMR_SPOTIFY_ACCESS_TOKEN", "")
	cfg, err = LoadConfig()
	if err != nil || cfg.SpotifyAccessToken != "" {
		t.Fatal("explicit blank environment did not disable imported bearer")
	}
}

func TestCapabilitiesEndpointAndDefaultOperation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("api:\n  default_operation: capabilities\n  endpoints:\n    capabilities: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OMMR_CONFIG_FILE", path)
	cfg, err := LoadConfig()
	if err != nil || cfg.API.DefaultOperation != "capabilities" || cfg.API.Enabled("capabilities") {
		t.Fatalf("capability configuration: %+v %v", cfg, err)
	}
}

func TestNativeAlbumsConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "albums.yaml")
	if err := os.WriteFile(path, []byte("api:\n  default_operation: albums\n  endpoints:\n    albums: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OMMR_CONFIG_FILE", path)
	cfg, err := LoadConfig()
	if err != nil || cfg.API.DefaultOperation != "albums" || cfg.API.Enabled("albums") {
		t.Fatalf("album config ignored: %+v %v", cfg, err)
	}
}
