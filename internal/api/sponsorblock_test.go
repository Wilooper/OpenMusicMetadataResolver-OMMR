package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/cache"
	"github.com/ommr/ommr/internal/cache/disk"
	"github.com/ommr/ommr/internal/config"
	"github.com/ommr/ommr/internal/enrichment/catalog"
	"github.com/ommr/ommr/internal/merger"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/providerhealth"
	"github.com/ommr/ommr/internal/ratelimit"
	"github.com/ommr/ommr/internal/resolver"
)

type resolvedTTLCache struct {
	cache.Cache
	ttl time.Duration
}

func (c *resolvedTTLCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if strings.HasPrefix(key, "resolved:") {
		c.ttl = ttl
	}
	return c.Cache.Set(ctx, key, value, ttl)
}

func TestSponsorBlockResolveMetadataCacheFailureAndRejectedVideos(t *testing.T) {
	for _, mode := range []string{"available", "missing", "unavailable", "rejected", "custom_budget"} {
		t.Run(mode, func(t *testing.T) {
			reg := adapters.NewRegistry()
			video := canonical.Track{Title: "Song", Artists: []canonical.Artist{{Name: "Singer"}}, DurationMS: 281000, IDs: map[string]string{"ytmusic": "kJQP7kiw5Fk"}}
			if mode == "rejected" {
				video.Artists = []canonical.Artist{{Name: "Unrelated Artist"}}
				reg.Register(extractionFixtureAdapter{name: "applemusic", track: canonical.Track{Title: "Song", Artists: []canonical.Artist{{Name: "Singer"}}, DurationMS: 281000, IDs: map[string]string{"applemusic": "123"}}})
			}
			reg.Register(extractionFixtureAdapter{name: "ytmusic", track: video})
			backend, err := disk.NewDiskCache(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer backend.Close()
			cached := &resolvedTTLCache{Cache: backend}
			client := catalog.NewClient(nil)
			client.Cache = backend
			calls := 0
			client.HTTP.Transport = artistTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if mode == "custom_budget" {
					deadline, ok := r.Context().Deadline()
					if !ok || time.Until(deadline) < 6*time.Second {
						t.Error("configured timeout lost in health/resolver/HTTP layers")
					}
				}
				if r.URL.Host != "sponsor.ajay.app" || r.Method != "GET" {
					t.Fatal("unexpected enrichment request")
				}
				status, body := 200, `[{"videoID":"kJQP7kiw5Fk","segments":[{"UUID":"point","segment":[21.808,21.808],"category":"poi_highlight","actionType":"poi","videoDuration":281.521,"votes":9,"locked":1}]}]`
				if mode == "missing" {
					status, body = 404, "Not found"
				}
				if mode == "unavailable" {
					status, body = 503, "Unavailable"
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
			})
			res := resolver.New(reg, cached, ratelimit.NewProviderLimiter(), 24*time.Hour)
			source := catalog.NewSponsorBlock(client)
			timeout := time.Second
			if mode == "custom_budget" {
				source.RequestTimeout = 8 * time.Second
				timeout = 20 * time.Second
			}
			manager := providerhealth.New(3, time.Minute)
			manager.Set("sponsorblock", "active", "public", "absent", "")
			res.Configure(merger.Policy{Mode: "consensus"}, providerhealth.WrapEnricher(source, manager))
			h := NewHandler(res, reg, &config.Config{ProviderTimeout: timeout})
			target := "/v1/resolve?youtube_id=kJQP7kiw5Fk"
			if mode == "rejected" {
				target = "/v1/resolve?apple_id=123"
			}
			public, _ := readResponse(t, h.HandleResolve, "GET", target, "")
			debug, _ := readResponse(t, h.HandleResolve, "GET", target+"&debug=true", "")
			track := public["track"].(map[string]any)
			if track["title"] != "Song" {
				t.Fatal("optional annotation broke resolution")
			}
			if mode == "available" || mode == "custom_budget" {
				data := track["video_annotations"].(map[string]any)["sponsorblock"].(map[string]any)
				point := data["points_of_interest"].([]any)[0].(map[string]any)
				if data["video_id"] != "kJQP7kiw5Fk" || point["time_ms"] != float64(21808) || point["duration_checked"] != true {
					t.Fatalf("video annotation lost: %+v", data)
				}
				debugData := debug["track"].(map[string]any)["extensions"].(map[string]any)["video_annotations"]
				a, _ := json.Marshal(track["video_annotations"])
				b, _ := json.Marshal(debugData)
				if string(a) != string(b) {
					t.Fatal("public/debug annotation mismatch")
				}
				if cached.ttl != time.Hour {
					t.Fatalf("positive freshness cap: %s", cached.ttl)
				}
				if track["provider_data"].(map[string]any)["sponsorblock"] != nil {
					t.Fatal("annotation duplicated as raw provider data")
				}
			} else if track["video_annotations"] != nil {
				t.Fatal("missing/failed/foreign POI emitted")
			}
			if mode == "missing" && cached.ttl != 5*time.Minute {
				t.Fatalf("negative freshness cap: %s", cached.ttl)
			}
			if mode == "unavailable" && cached.ttl != 30*time.Second {
				t.Fatalf("failure retry cap: %s", cached.ttl)
			}
			if mode == "rejected" {
				if calls != 0 {
					t.Fatal("rejected video queried for points")
				}
			} else if calls != 1 {
				t.Fatal("annotation/debug cache repeated source call")
			}
		})
	}
}
