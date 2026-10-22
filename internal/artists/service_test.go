package artists

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ommr/ommr/internal/cache/disk"
	"github.com/ommr/ommr/internal/extractors"
	"github.com/ommr/ommr/internal/providerhealth"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func captured(t *testing.T, name string) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/" + name + ".json")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func response(body []byte, status int) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(string(body)))}
}

func TestCapturedNativeProfilesAndAmbiguousNames(t *testing.T) {
	fixtures := map[string][]byte{}
	for _, name := range []string{"deezer-search", "deezer-profile", "deezer-top", "jio-search", "jio-profile", "apple-search", "apple-profile"} {
		fixtures[name] = captured(t, name)
	}
	s := New(nil, nil)
	s.HTTP.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Accept-Language") != "en" || !strings.Contains(r.Header.Get("User-Agent"), "https://github.com/") {
			t.Error("missing source headers")
		}
		key := ""
		switch r.URL.Host {
		case "api.deezer.com":
			switch r.URL.Path {
			case "/search/artist":
				key = "deezer-search"
			case "/artist/5810099":
				key = "deezer-profile"
			case "/artist/5810099/top":
				key = "deezer-top"
			}
		case "www.jiosaavn.com":
			switch r.URL.Query().Get("__call") {
			case "search.getArtistResults":
				key = "jio-search"
			case "artist.getArtistPageDetails":
				key = "jio-profile"
			}
		case "itunes.apple.com":
			if r.URL.Path == "/search" {
				key = "apple-search"
			} else {
				key = "apple-profile"
				if r.URL.Query().Get("sort") != "recent" {
					t.Error("lookup list must be labeled recent")
				}
			}
		}
		if key == "" {
			t.Errorf("unexpected request: %s", r.URL)
			return nil, errors.New("unexpected")
		}
		return response(fixtures[key], 200), nil
	})
	r := s.Resolve(context.Background(), Query{Artist: "Gur Sidhu", Sources: []string{"jiosaavn", "deezer", "applemusic"}, Limit: 2})
	if len(r.Profiles) != 2 || len(r.Candidates) != 3 {
		t.Fatalf("profiles/candidates: %+v", r)
	}
	for _, p := range r.Profiles {
		if p.Name != "Gur Sidhu" || len(p.TopTracks) != 2 || p.Counts.Tracks != nil {
			t.Fatalf("invented identity or total: %+v", p)
		}
		if p.Provider == "jiosaavn" {
			if p.Biography != "" {
				t.Fatal("empty biography JSON emitted as prose")
			}
			found := false
			for _, m := range p.Metrics {
				if m.Name == "listeners" {
					found = true
					if m.Period != "unspecified" || m.Value == nil || *m.Value != 874944 {
						t.Fatalf("listener semantics: %+v", m)
					}
				}
			}
			if !found {
				t.Fatal("listeners missing")
			}
		}
		encoded, _ := json.Marshal(p)
		if strings.Contains(string(encoded), "encrypted") || strings.Contains(string(encoded), "preview") {
			t.Fatal("playback data leaked")
		}
	}
	if r.Sources[0].Status != "ambiguous" {
		t.Fatalf("Apple duplicates selected arbitrarily: %+v", r.Sources)
	}
	r = s.Resolve(context.Background(), Query{Provider: "applemusic", ID: "862626825", Limit: 2})
	if len(r.Profiles) != 1 || len(r.Profiles[0].RecentTracks) != 2 || len(r.Profiles[0].TopTracks) != 0 || len(r.Profiles[0].Metrics) != 0 {
		t.Fatalf("Apple list/count semantics: %+v", r)
	}
}

type fakePool struct {
	raw   json.RawMessage
	fresh bool
}

func (p *fakePool) Call(context.Context, string, string, int) (json.RawMessage, error) {
	return p.raw, nil
}
func (p *fakePool) CallFresh(context.Context, string, string, int) (json.RawMessage, error) {
	p.fresh = true
	return p.raw, nil
}

func TestCapturedYouTubeMetricsBelongToArtistAndKeepPrecision(t *testing.T) {
	raw := captured(t, "youtube-profile")
	var payload map[string]any
	_ = json.Unmarshal(raw, &payload)
	payload["observed_at"] = "2026-10-18T09:00:00Z"
	metadata := payload["metadata"].(map[string]any)
	metadata["related"] = map[string]any{"results": []any{map[string]any{"title": "Unrelated", "subscribers": "999M"}}}
	raw, _ = json.Marshal(payload)
	pool := &fakePool{raw: raw}
	s := New(nil, nil)
	s.Pool = pool
	r := s.Resolve(context.Background(), Query{Provider: "ytmusic", ID: "UC5RFfy546sgeEdO03RBqKyg", Limit: 2})
	if len(r.Profiles) != 1 || !pool.fresh {
		t.Fatalf("YouTube profile: %+v", r)
	}
	p := r.Profiles[0]
	if p.ID != "UC5RFfy546sgeEdO03RBqKyg" || p.ProviderData["channelId"] != "UChWa5ghfaiI_yLCbxvya1aw" {
		t.Fatal("topic/channel ownership overwritten")
	}
	metrics := map[string]Metric{}
	for _, m := range p.Metrics {
		metrics[m.Name] = m
	}
	if metrics["subscribers"].DisplayValue != "162K" || metrics["subscribers"].Value != nil {
		t.Fatal("abbreviated subscriber count treated as exact")
	}
	if metrics["monthly_audience"].Period != "monthly" || metrics["monthly_audience"].DisplayValue != "105M" {
		t.Fatal("monthly audience mislabeled")
	}
	if metrics["views"].Value == nil || *metrics["views"].Value != 172808309 {
		t.Fatal("exact views lost")
	}
	if p.ObservedAt.Format(time.RFC3339) != "2026-10-18T09:00:00Z" {
		t.Fatal("cached worker observation time relabeled")
	}
	if p.Counts.Tracks != nil || p.Counts.Albums != nil {
		t.Fatal("bounded list treated as total")
	}
	if !strings.Contains(p.Biography, "From Wikipedia") {
		t.Fatal("biography attribution lost")
	}
}

func TestExactIDRejectsMismatchedProfileAndKeepsPartialTopFailure(t *testing.T) {
	for _, test := range []struct {
		name  string
		wrong bool
	}{{"wrong ID", true}, {"top unavailable", false}} {
		t.Run(test.name, func(t *testing.T) {
			s := New(nil, nil)
			s.HTTP.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
				if strings.HasSuffix(r.URL.Path, "/top") {
					return response([]byte(`{"error":"unavailable"}`), 503), nil
				}
				b := captured(t, "deezer-profile")
				if test.wrong {
					b = []byte(`{"id":999,"name":"Other Artist"}`)
				}
				return response(b, 200), nil
			})
			r := s.Resolve(context.Background(), Query{Provider: "deezer", ID: "5810099", Limit: 5})
			if test.wrong {
				if len(r.Profiles) > 0 || r.Sources[0].Error != "identity_mismatch" {
					t.Fatalf("wrong ID accepted: %+v", r)
				}
			} else if len(r.Profiles) != 1 || !r.Sources[0].Partial {
				t.Fatalf("useful profile discarded: %+v", r)
			}
		})
	}
}

func TestCacheBypassAndDisabledSource(t *testing.T) {
	backend, err := disk.NewDiskCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	s := New(nil, backend)
	var calls atomic.Int32
	var active atomic.Bool
	active.Store(true)
	s.Available = func(string) bool { return active.Load() }
	s.HTTP.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if strings.HasSuffix(r.URL.Path, "/top") {
			return response(captured(t, "deezer-top"), 200), nil
		}
		return response(captured(t, "deezer-profile"), 200), nil
	})
	q := Query{Provider: "deezer", ID: "5810099", Limit: 2}
	first := s.Resolve(context.Background(), q)
	second := s.Resolve(context.Background(), q)
	if calls.Load() != 2 || !second.Sources[0].Cached || !first.Profiles[0].ObservedAt.Equal(second.Profiles[0].ObservedAt) {
		t.Fatal("cache repeated work or rewrote observation time")
	}
	q.BypassCache = true
	s.Resolve(context.Background(), q)
	if calls.Load() != 4 {
		t.Fatal("bypass did not fetch")
	}
	active.Store(false)
	r := s.Resolve(context.Background(), q)
	if len(r.Profiles) > 0 || calls.Load() != 4 {
		t.Fatal("disabled source served/called")
	}
}

func TestCancellationAndHostPolicy(t *testing.T) {
	s := New(func(ctx context.Context, _ string) error { return ctx.Err() }, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := s.Resolve(ctx, Query{Provider: "deezer", ID: "5810099", Limit: 2})
	if len(r.Profiles) > 0 || r.Sources[0].Error != "cancelled" {
		t.Fatalf("cancelled source: %+v", r)
	}
	for _, u := range []string{"http://api.deezer.com/artist/1", "https://api.deezer.com@evil.test/", "https://api.deezer.com:443/", "https://evil.test/"} {
		if _, err := s.get(context.Background(), "deezer", u); err == nil {
			t.Fatalf("unsafe URL accepted: %s", u)
		}
	}
	health := providerhealth.New(1, time.Minute)
	health.Set("deezer", "active", "public", "absent", "")
	s.Health = health
	s.Wait = nil
	s.HTTP.Transport = roundTrip(func(*http.Request) (*http.Response, error) {
		return response([]byte(`{"error":"not usable"}`), 503), nil
	})
	s.Resolve(context.Background(), Query{Provider: "deezer", ID: "5810099", Limit: 2})
	r = s.Resolve(context.Background(), Query{Provider: "deezer", ID: "5810099", Limit: 2})
	if r.Sources[0].Status != "unavailable" {
		t.Fatal("circuit did not suspend")
	}
}

func TestValidation(t *testing.T) {
	for _, q := range []Query{{Artist: "Gur Sidhu", Limit: 51}, {Artist: "https://evil.test", Limit: 2}, {Provider: "deezer", ID: "1/../2", Limit: 2}, {Provider: "jiosaavn", ID: "SHARE_TOKEN", Limit: 2}, {Artist: "Gur Sidhu", Sources: []string{"spotify"}, Limit: 2}, {Provider: "ytmusic", ID: "UC5RFfy546sgeEdO03RBqKyg", Sources: []string{"deezer"}, Limit: 2}} {
		if Validate(q) == nil {
			t.Fatalf("invalid query: %+v", q)
		}
	}
}

func TestCapturedMusicBrainzCatalogIsNotTopTracksOrPlays(t *testing.T) {
	s := New(nil, nil)
	s.HTTP.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "/recording") {
			return response(captured(t, "mb-recordings"), 200), nil
		}
		return response(captured(t, "mb-profile"), 200), nil
	})
	r := s.Resolve(context.Background(), Query{Provider: "musicbrainz", ID: "60bb7169-b83b-4603-a319-7bcbd1130cc9", Limit: 2})
	if len(r.Profiles) != 1 {
		t.Fatalf("missing profile: %+v", r)
	}
	p := r.Profiles[0]
	if p.Counts.Recordings == nil || *p.Counts.Recordings < 2 || p.Counts.Tracks != nil || len(p.TopTracks) != 0 || len(p.CatalogTracks) != 2 || len(p.Metrics) != 0 {
		t.Fatalf("catalog mislabeled as audience/top/track totals: %+v", p)
	}
}

type busyPool struct{}

func (busyPool) Call(context.Context, string, string, int) (json.RawMessage, error) {
	return nil, extractors.ErrBusy
}

func TestLocalCapacityDoesNotTripSourceCircuit(t *testing.T) {
	for _, source := range []string{"deezer", "ytmusic"} {
		t.Run(source, func(t *testing.T) {
			manager := providerhealth.New(1, time.Minute)
			healthName := source
			id := "5810099"
			if source == "ytmusic" {
				healthName = "ytmusic_python"
				id = "UC5RFfy546sgeEdO03RBqKyg"
			}
			manager.Set(healthName, "active", "public", "absent", "")
			s := New(func(context.Context, string) error { return errors.New("local capacity") }, nil)
			s.Health = manager
			s.Pool = busyPool{}
			r := s.Resolve(context.Background(), Query{Provider: source, ID: id, Limit: 2})
			if len(r.Profiles) != 0 || manager.Snapshot()[0].State != "active" || manager.Snapshot()[0].ConsecutiveFailures != 0 {
				t.Fatalf("capacity counted as upstream failure: %+v", r)
			}
			want := "request_budget_exhausted"
			if source == "ytmusic" {
				want = "queue_full"
			}
			if r.Sources[0].Error != want {
				t.Fatalf("error=%s", r.Sources[0].Error)
			}
		})
	}
}

func TestSearchBoundEvenIfProviderIgnoresLimit(t *testing.T) {
	rows := []map[string]any{}
	for i := 1; i <= 21; i++ {
		rows = append(rows, map[string]any{"artistId": i, "artistName": "Other"})
	}
	rows[20]["artistName"] = "Target"
	payload, _ := json.Marshal(map[string]any{"results": rows})
	s := New(nil, nil)
	s.HTTP.Transport = roundTrip(func(*http.Request) (*http.Response, error) { return response(payload, 200), nil })
	r := s.Resolve(context.Background(), Query{Artist: "Target", Sources: []string{"applemusic"}, Limit: 5})
	if len(r.Profiles) > 0 || r.Sources[0].Status != "not_found" {
		t.Fatal("searched outside fixed candidate window")
	}
}
