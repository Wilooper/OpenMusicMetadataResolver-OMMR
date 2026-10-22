package credentials

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionFormatsAndDomainIsolation(t *testing.T) {
	for _, raw := range []string{
		`{"headers":{"Cookie":"sp_dc=private","Authorization":"Bearer browser-token"},"expires_at":2000000000}`,
		`{"cookie":"sp_dc=private","access_token":"browser-token","expires_at":2000000000}`,
	} {
		session, err := Parse([]byte(raw), "spotify.com")
		if err != nil || session.Cookie != "sp_dc=private" || session.AccessToken != "browser-token" {
			t.Fatal("session import failed")
		}
	}
	session, err := Parse([]byte(`[{"name":"sp_dc","value":"ok","domain":".spotify.com"},{"name":"wrong","value":"private","domain":"spotify.com.evil.test"}]`), "spotify.com")
	if err != nil || session.Cookie != "sp_dc=ok" {
		t.Fatal("foreign cookie was imported")
	}
	session, err = Parse([]byte("# Netscape HTTP Cookie File\n#HttpOnly_.youtube.com\tTRUE\t/\tTRUE\t0\tSAPISID\tprivate"), "youtube.com")
	if err != nil || session.Cookie != "SAPISID=private" {
		t.Fatal("Netscape import failed")
	}
	session, err = Parse([]byte(`{"cookie":"media-user-token=user-token","headers":{"Authorization":"Bearer developer-token"}}`), "apple.com")
	if err != nil || session.UserToken != "user-token" || session.DeveloperToken != "developer-token" {
		t.Fatal("Apple session import failed")
	}
}
func TestPrivateFilesAndLimitedDiscovery(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	path := filepath.Join(dir, "lastfm.json")
	os.WriteFile(path, []byte(`{"api_key":"private-key"}`), 0600)
	os.WriteFile(filepath.Join(dir, "unrelated.json"), []byte("invalid"), 0600)
	values, err := Discover(dir)
	if err != nil || values["lastfm_api_key"] != "private-key" {
		t.Fatal("key discovery failed")
	}
	os.Chmod(path, 0644)
	if _, err = Discover(dir); err == nil || strings.Contains(err.Error(), "private-key") {
		t.Fatal("unsafe credential file accepted")
	}
	os.Remove(path)
	outside := filepath.Join(t.TempDir(), "secret")
	os.WriteFile(outside, []byte(`{}`), 0600)
	os.Symlink(outside, path)
	if _, err = Discover(dir); err == nil {
		t.Fatal("credential symlink accepted")
	}
	if _, err = Parse([]byte("SAPISID=secret\r\nInjected: true"), "youtube.com"); err == nil {
		t.Fatal("header injection accepted")
	}
}
