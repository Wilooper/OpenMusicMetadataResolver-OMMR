package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/cache/disk"
	"github.com/ommr/ommr/internal/config"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/platformlinks"
	"github.com/ommr/ommr/internal/ratelimit"
	"github.com/ommr/ommr/internal/resolver"
)

type countingResponseAdapter struct {
	extractionFixtureAdapter
	calls *atomic.Int64
}

func (a countingResponseAdapter) Search(ctx context.Context, q adapters.Query) ([]canonical.TrackCandidate, error) {
	a.calls.Add(1)
	return a.extractionFixtureAdapter.Search(ctx, q)
}

func responseFixture(t *testing.T) (*Handler, *atomic.Int64) {
	t.Helper()
	calls := &atomic.Int64{}
	reg := adapters.NewRegistry()
	reg.Register(countingResponseAdapter{extractionFixtureAdapter{name: "deezer", track: canonical.Track{
		Title: "Song", Artists: []canonical.Artist{{Name: "Singer"}}, Album: canonical.Album{Title: "Album"},
		IDs: map[string]string{"deezer": "123"}, ISRC: "USABC2300001", DurationMS: 123000,
		Genres: []string{"Pop"}, Credits: []canonical.Credit{{Name: "Writer", Roles: []string{"lyricist"}}},
	}}, calls})
	cache, err := disk.NewDiskCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cache.Close() })
	return NewHandler(resolver.New(reg, cache, ratelimit.NewProviderLimiter(), time.Hour), reg, &config.Config{ProviderTimeout: time.Second, BulkMaxItems: 100, BulkWorkers: 2}), calls
}

func readResponse(t *testing.T, handler http.HandlerFunc, method, target, body string) (map[string]any, int) {
	t.Helper()
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(method, target, strings.NewReader(body)))
	if rec.Code != 200 {
		t.Fatalf("%s: %d %s", target, rec.Code, rec.Body)
	}
	var result map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result, rec.Body.Len()
}

func assertPublicTrack(t *testing.T, track map[string]any) {
	t.Helper()
	for _, key := range []string{"identity_reasons", "identity_matches", "sources", "field_sources", "match_score", "match_breakdown", "confidence", "completeness", "relations"} {
		if _, exists := track[key]; exists {
			t.Errorf("diagnostic %s leaked", key)
		}
	}
	if track["title"] != "Song" || track["duration_ms"] != float64(123000) || track["ids"].(map[string]any)["deezer"] != "123" {
		t.Fatalf("metadata lost: %+v", track)
	}
	catalog := track["extensions"].(map[string]any)["platform_links"].(map[string]any)
	assertPublicCatalog(t, catalog)
}
func assertPublicCatalog(t *testing.T, catalog map[string]any) {
	t.Helper()
	if _, exists := catalog["unavailable_platforms"]; exists {
		t.Error("unavailable attempts leaked")
	}
	for _, raw := range catalog["platforms"].(map[string]any) {
		link := raw.(map[string]any)
		for _, key := range []string{"url_source", "confidence", "match_method"} {
			if _, exists := link[key]; exists {
				t.Errorf("link diagnostic %s leaked", key)
			}
		}
	}
}

func TestResolveDebugModesShareCacheInBothOrders(t *testing.T) {
	for _, firstDebug := range []bool{false, true} {
		t.Run(map[bool]string{false: "public-first", true: "debug-first"}[firstDebug], func(t *testing.T) {
			h, calls := responseFixture(t)
			var compactSize, debugSize int
			for index, debug := range []bool{firstDebug, !firstDebug, false, true} {
				target := "/v1/resolve?artist=Singer&title=Song"
				if debug {
					target += "&debug=true"
				}
				got, size := readResponse(t, h.HandleResolve, "GET", target, "")
				track := got["track"].(map[string]any)
				if debug {
					debugSize = size
					if got["provider_status"] == nil || got["resolver_stats"] == nil || track["field_sources"] == nil || track["identity_matches"] == nil {
						t.Fatalf("missing debug data: %+v", got)
					}
					if got["provider_status"].([]any)[0].(map[string]any)["cached"] != (index > 0) {
						t.Fatal("incorrect cache diagnostics")
					}
				} else {
					compactSize = size
					if len(got) != 1 {
						t.Fatalf("extra production keys: %+v", got)
					}
					assertPublicTrack(t, track)
				}
			}
			if calls.Load() != 1 {
				t.Fatalf("mode toggle caused %d provider calls", calls.Load())
			}
			if compactSize >= debugSize {
				t.Fatalf("public %d >= debug %d", compactSize, debugSize)
			}
			t.Logf("fixture bytes: public=%d debug=%d reduction=%.1f%%", compactSize, debugSize, 100*(1-float64(compactSize)/float64(debugSize)))
		})
	}
}

func TestPublicResponsesAcrossDataEndpoints(t *testing.T) {
	h, _ := responseFixture(t)
	for _, debug := range []bool{false, true} {
		suffix := "&debug=false"
		if debug {
			suffix = "&debug=true"
		}
		links, _ := readResponse(t, h.HandleLinks, "GET", "/v1/links?artist=Singer&title=Song"+suffix, "")
		_, hasStatus := links["provider_status"]
		if hasStatus != debug {
			t.Fatal("wrong links diagnostic mode")
		}
		if !debug {
			assertPublicCatalog(t, links["links"].(map[string]any))
		}
		bulk, _ := readResponse(t, h.HandleBulk, "POST", "/v1/bulk?x=1"+suffix, `{"queries":[{"artist":"Singer","title":"Song"}]}`)
		item := bulk["results"].([]any)[0].(map[string]any)
		_, hasSources := item["sources"]
		if hasSources != debug || item["success"] != true {
			t.Fatalf("wrong bulk mode: %+v", item)
		}
		if !debug {
			assertPublicTrack(t, item["track"].(map[string]any))
		}
		search, _ := readResponse(t, h.HandleSearch, "GET", "/v1/search?q=Song"+suffix, "")
		result := search["results"].([]any)[0].(map[string]any)
		for _, key := range []string{"match_score", "confidence", "source_count", "identity_status"} {
			_, exists := result[key]
			if exists != debug {
				t.Errorf("wrong search mode for %s", key)
			}
		}
		extracted, _ := readResponse(t, h.HandleExtract, "GET", "/v1/extract?provider=deezer&id=123"+suffix, "")
		if debug {
			if extracted["sources"] == nil {
				t.Fatal("missing extraction diagnostics")
			}
		} else {
			if len(extracted) != 1 {
				t.Fatal("extraction provenance leaked")
			}
			assertPublicTrack(t, extracted["results"].([]any)[0].(map[string]any))
		}
	}
}

func TestInvalidDebugFailsBeforeProviderWork(t *testing.T) {
	h, calls := responseFixture(t)
	for _, handler := range []http.HandlerFunc{h.HandleResolve, h.HandleLinks, h.HandleBulk, h.HandleSearch, h.HandleExtract} {
		for _, raw := range []string{"debug=", "debug=banana", "debug=true&debug=false"} {
			rec := httptest.NewRecorder()
			handler(rec, httptest.NewRequest("GET", "/v1/test?"+raw, nil))
			if rec.Code != 400 || !strings.Contains(rec.Body.String(), "debug") {
				t.Fatalf("invalid debug accepted: %s", rec.Body)
			}
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid debug triggered provider calls")
	}
}

func TestTrackProjectionDoesNotMutateDiagnosticsOrExtensions(t *testing.T) {
	track := canonical.Track{Sources: []string{"deezer"}, IDs: map[string]string{"deezer": "123"}, Extensions: map[string]json.RawMessage{
		"wikipedia":      json.RawMessage(`{"article_url":"https://en.wikipedia.org/wiki/Song","summary":"Context"}`),
		"internal_trace": json.RawMessage(`{"secret":"diagnostic"}`),
	}}
	platformlinks.Attach(&track)
	before, _ := json.Marshal(track)
	compact, _ := json.Marshal(trackResponse(&track, false))
	after, _ := json.Marshal(trackResponse(&track, true))
	if string(before) != string(after) {
		t.Fatal("projection mutated canonical result")
	}
	if strings.Contains(string(compact), "internal_trace") || !strings.Contains(string(compact), "article_url") {
		t.Fatal("wrong extension filtering")
	}
	if trackResponse(nil, false) != nil || trackResponse(nil, true) != nil {
		t.Fatal("nil track changed")
	}
}

func TestNoMatchStaysNullAndBulkFailureRemainsVisible(t *testing.T) {
	h, _ := responseFixture(t)
	got, _ := readResponse(t, h.HandleResolve, "GET", "/v1/resolve?isrc=CONFLICT", "")
	if len(got) != 1 || got["track"] != nil {
		t.Fatalf("wrong no-match result: %+v", got)
	}
	got, _ = readResponse(t, h.HandleBulk, "POST", "/v1/bulk", `{"queries":[{"isrc":"CONFLICT"}]}`)
	item := got["results"].([]any)[0].(map[string]any)
	if got["failed"] != float64(1) || item["success"] != false {
		t.Fatal("bulk failure hidden")
	}
	if _, exists := item["sources"]; exists {
		t.Fatal("bulk failure diagnostics leaked")
	}
}

func TestDebugBooleanParsing(t *testing.T) {
	for _, tt := range []struct {
		query   string
		enabled bool
	}{
		{"", false}, {"?debug=false", false}, {"?debug=0", false}, {"?debug=FALSE", false},
		{"?debug=true", true}, {"?debug=1", true}, {"?debug=TRUE", true},
	} {
		t.Run(tt.query, func(t *testing.T) {
			enabled, valid := parseDebug(httptest.NewRecorder(), httptest.NewRequest("GET", "/v1/resolve"+tt.query, nil))
			if !valid || enabled != tt.enabled {
				t.Fatalf("parseDebug = %v, %v", enabled, valid)
			}
		})
	}
}

func TestMetadataComparisonRemainsDebugOnly(t *testing.T) {
	track := &canonical.Track{Title: "Song", IDs: map[string]string{}, Extensions: map[string]json.RawMessage{"metadata_comparison": json.RawMessage(`{"duration_ms":{"different":true}}`)}}
	public, _ := json.Marshal(trackResponse(track, false))
	debug, _ := json.Marshal(trackResponse(track, true))
	if strings.Contains(string(public), "metadata_comparison") || !strings.Contains(string(debug), "metadata_comparison") {
		t.Fatal("comparison diagnostics projection failed")
	}
}
