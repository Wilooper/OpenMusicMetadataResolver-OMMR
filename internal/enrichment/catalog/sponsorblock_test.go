package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ommr/ommr/internal/cache/disk"
	"github.com/ommr/ommr/internal/models/canonical"
)

const testVideoID = "kJQP7kiw5Fk"

func videoTrack(t *testing.T, duration int64) canonical.Track {
	t.Helper()
	data, _ := json.Marshal(map[string]any{"ytmusic": map[string]any{"metadata": map[string]any{"ids": map[string]string{"ytmusic": testVideoID}, "duration_ms": duration}}})
	return canonical.Track{IDs: map[string]string{"ytmusic": testVideoID}, DurationMS: 120000, Extensions: map[string]json.RawMessage{"provider_data": data}}
}

func TestSponsorBlockOnlyQueriesAcceptedVideo(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; io.WriteString(w, "[]") }))
	defer server.Close()
	source := NewSponsorBlock(NewClient(nil))
	source.base = server.URL
	for _, id := range []string{"", "invalid", "https://youtube.com/watch?v=" + testVideoID} {
		if result, err := source.Enrich(context.Background(), canonical.Track{IDs: map[string]string{"ytmusic": id}}); result != nil || err != nil {
			t.Fatal("unexpected enrichment")
		}
	}
	if calls != 0 {
		t.Fatal("unaccepted/invalid ID triggered request")
	}
}

func TestSponsorBlockHashBucketVideoScopeAndSourceDuration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sum := sha256.Sum256([]byte(testVideoID))
		if r.URL.Path != "/api/skipSegments/"+hex.EncodeToString(sum[:2]) || r.URL.Query().Get("videoID") != "" || r.URL.Query().Get("categories") != `["poi_highlight"]` || r.URL.Query().Get("actionTypes") != `["poi"]` || r.URL.Query().Get("service") != "YouTube" {
			t.Error("wrong/private lookup parameters")
		}
		io.WriteString(w, `[{"videoID":"wrongrecord","segments":[{"UUID":"foreign","segment":[1,1],"category":"poi_highlight","actionType":"poi"}]},{"videoID":"`+testVideoID+`","segments":[{"UUID":"a","segment":[220.1234,220.1234],"category":"poi_highlight","actionType":"poi","videoDuration":281.521,"votes":9,"locked":1},{"UUID":"b","segment":[0,0],"category":"poi_highlight","actionType":"poi","videoDuration":281.521,"votes":2,"locked":0}]}]`)
	}))
	defer server.Close()
	source := NewSponsorBlock(NewClient(nil))
	source.base = server.URL
	track := videoTrack(t, 281000)
	result, err := source.Enrich(context.Background(), track)
	if err != nil || result == nil {
		t.Fatalf("%+v %v", result, err)
	}
	data := result.Data.(VideoAnnotations)
	if len(data.Points) != 2 || data.Points[0].TimeMS != 0 || data.Points[1].TimeMS != 220123 || !data.Points[1].DurationChecked || data.Points[1].Votes != 9 || data.Points[1].Locked != 1 || data.VideoID != testVideoID || data.License != "CC-BY-NC-SA-4.0" {
		t.Fatalf("POI data lost: %+v", data)
	}
	if track.DurationMS != 120000 {
		t.Fatal("canonical duration overwritten")
	}
}

func TestSponsorBlockFiltersMalformedAndStalePoints(t *testing.T) {
	for _, tc := range []struct{ name, segment string }{
		{"duration mismatch", `{"UUID":"a","segment":[20,20],"category":"poi_highlight","actionType":"poi","videoDuration":180}`},
		{"skip not poi", `{"UUID":"a","segment":[20,20],"category":"poi_highlight","actionType":"skip"}`},
		{"wrong category", `{"UUID":"a","segment":[20,20],"category":"sponsor","actionType":"poi"}`},
		{"range not point", `{"UUID":"a","segment":[20,21],"category":"poi_highlight","actionType":"poi"}`},
		{"negative", `{"UUID":"a","segment":[-1,-1],"category":"poi_highlight","actionType":"poi"}`},
		{"past video", `{"UUID":"a","segment":[300,300],"category":"poi_highlight","actionType":"poi"}`},
		{"past submitted duration", `{"UUID":"a","segment":[281.8,281.8],"category":"poi_highlight","actionType":"poi","videoDuration":281}`},
		{"malformed point", `{"UUID":"a","segment":[20],"category":"poi_highlight","actionType":"poi"}`},
		{"missing ID", `{"segment":[20,20],"category":"poi_highlight","actionType":"poi"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				io.WriteString(w, `[{"videoID":"`+testVideoID+`","segments":[`+tc.segment+`]}]`)
			}))
			defer server.Close()
			source := NewSponsorBlock(NewClient(nil))
			source.base = server.URL
			result, err := source.Enrich(context.Background(), videoTrack(t, 282000))
			if err != nil || result != nil {
				t.Fatalf("invalid annotation returned: %+v %v", result, err)
			}
		})
	}
	for _, n := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), -1, 1e20} {
		if finiteSeconds(n) {
			t.Fatal("unsafe timestamp")
		}
	}
}

func TestSponsorBlockUnknownDurationDeduplicationAndBounds(t *testing.T) {
	body := `[{"videoID":"` + testVideoID + `","segments":[{"UUID":"a","segment":[20,20],"category":"poi_highlight","actionType":"poi","videoDuration":281},{"UUID":"a","segment":[20,20],"category":"poi_highlight","actionType":"poi","videoDuration":281}]}]`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, body) }))
	defer server.Close()
	source := NewSponsorBlock(NewClient(nil))
	source.base = server.URL
	result, err := source.Enrich(context.Background(), videoTrack(t, 0))
	if err != nil || result == nil {
		t.Fatal(err)
	}
	data := result.Data.(VideoAnnotations)
	if len(data.Points) != 1 || data.Points[0].DurationChecked {
		t.Fatal("unknown duration used as evidence or duplicate retained")
	}
	body = strings.Replace(body, `[20,20]`, `[21,21]`, 1)
	if _, err = source.Enrich(context.Background(), videoTrack(t, 0)); err == nil {
		t.Fatal("conflicting same-ID point accepted")
	}
	var points []string
	for i := 0; i < 110; i++ {
		points = append(points, fmt.Sprintf(`{"UUID":"p-%03d","segment":[%d,%d],"category":"poi_highlight","actionType":"poi"}`, i, i, i))
	}
	body = `[{"videoID":"` + testVideoID + `","segments":[` + strings.Join(points, ",") + `]}]`
	result, err = source.Enrich(context.Background(), videoTrack(t, 0))
	if err != nil {
		t.Fatal(err)
	}
	data = result.Data.(VideoAnnotations)
	if len(data.Points) != 100 || !data.Truncated {
		t.Fatal("point bound/truncation missing")
	}
}

func TestSponsorBlockMissingCacheAndUpstreamErrors(t *testing.T) {
	for _, status := range []int{404, 429, 500, 200} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			calls, waits := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(status)
				io.WriteString(w, "invalid JSON")
			}))
			defer server.Close()
			backend, err := disk.NewDiskCache(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer backend.Close()
			client := NewClient(func(context.Context, string) error { waits++; return nil })
			client.Cache = backend
			source := NewSponsorBlock(client)
			source.base = server.URL
			for i := 0; i < 2; i++ {
				result, err := source.Enrich(context.Background(), videoTrack(t, 0))
				if status == 404 {
					if result != nil || err != nil {
						t.Fatal("missing POI became failure")
					}
				} else if err == nil {
					t.Fatal("upstream error hidden")
				}
			}
			if status == 404 && (calls != 1 || waits != 1) {
				t.Fatal("404 not negatively cached before rate limiting")
			}
			if status != 404 && calls != 2 {
				t.Fatal("upstream failure cached")
			}
		})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	source := NewSponsorBlock(NewClient(nil))
	source.base = server.URL
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := source.Enrich(ctx, videoTrack(t, 0)); err == nil {
		t.Fatal("caller deadline ignored")
	}
}
