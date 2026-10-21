package api

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/cache/disk"
	"github.com/ommr/ommr/internal/config"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/ratelimit"
	"github.com/ommr/ommr/internal/resolver"
)

func TestProductionCompressionAndDirectAlbumPlacement(t *testing.T) {
	reg := adapters.NewRegistry()
	for _, provider := range []string{"applemusic", "spotify", "deezer"} {
		album, disc, number := "Other edition", 1, 19
		if provider == "applemusic" {
			album, disc, number = "Played edition", 2, 4
		}
		track := canonical.Track{Title: "Song", Artists: []canonical.Artist{{Name: "Singer"}}, DurationMS: 160000, IDs: map[string]string{provider: "123"}, Album: canonical.Album{Title: album, IDs: map[string]string{provider: "456"}}, TrackNumber: number, DiscNumber: disc, Credits: []canonical.Credit{{Name: "Contributor", Roles: []string{"composer", "producer", "vocal"}}}}
		reg.Register(extractionFixtureAdapter{name: provider, track: track})
	}
	backend, err := disk.NewDiskCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	h := NewHandler(resolver.New(reg, backend, ratelimit.NewProviderLimiter(), time.Hour), reg, &config.Config{ProviderTimeout: time.Second})
	server := httptest.NewServer(NewRouter(h, slog.New(slog.NewTextHandler(io.Discard, nil)), 1000))
	defer server.Close()
	client := &http.Client{Transport: &http.Transport{DisableCompression: true}}
	defer client.CloseIdleConnections()
	var plain map[string]any
	var plainSize int
	for _, encoding := range []string{"identity", "gzip"} {
		req, _ := http.NewRequest("GET", server.URL+"/v1/resolve?apple_id=123", nil)
		req.Header.Set("Accept-Encoding", encoding)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("response %d %s %v", resp.StatusCode, raw, err)
		}
		wireSize := len(raw)
		if encoding == "gzip" {
			if resp.Header.Get("Content-Encoding") != "gzip" {
				t.Fatal("gzip not negotiated")
			}
			reader, err := gzip.NewReader(bytes.NewReader(raw))
			if err != nil {
				t.Fatal(err)
			}
			raw, err = io.ReadAll(reader)
			reader.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
		var body map[string]any
		if json.Unmarshal(raw, &body) != nil {
			t.Fatal("invalid JSON")
		}
		if encoding == "identity" {
			plain = body
			plainSize = wireSize
		} else {
			if !reflect.DeepEqual(plain, body) {
				t.Fatal("compression lost metadata")
			}
			if wireSize >= plainSize {
				t.Fatal("response did not shrink")
			}
			t.Logf("lossless production bytes %d -> %d", plainSize, wireSize)
		}
		track := body["track"].(map[string]any)
		if track["disc_number"] != float64(2) || track["track_number"] != float64(4) || track["album"].(map[string]any)["title"] != "Played edition" {
			t.Fatalf("wrong edition: %+v", track)
		}
		if track["credit_details"] == nil || track["music_graph"] == nil || track["provider_data"] == nil {
			t.Fatal("rich metadata lost")
		}
		found := false
		for _, raw := range track["album_placements"].([]any) {
			p := raw.(map[string]any)
			if p["selected"] == true {
				found = true
				if p["provider"] != "applemusic" || p["album_id"] != "456" || p["track_id"] != "123" || p["disc_number"] != float64(2) || p["track_number"] != float64(4) {
					t.Fatalf("wrong source placement: %+v", p)
				}
			}
		}
		if !found {
			t.Fatal("selected placement missing")
		}
	}
}
