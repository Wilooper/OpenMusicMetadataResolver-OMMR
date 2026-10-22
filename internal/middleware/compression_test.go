package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestGzipNegotiation(t *testing.T) {
	for _, tc := range []struct {
		header   string
		accepted bool
	}{
		{"gzip", true}, {"br, GZIP;q=0.7", true}, {"*;q=1", true}, {"gzip;q=0, *;q=1", false},
		{"gzip;q=0.0", false}, {"gzip;q=NaN", false}, {"gzip;q=2", false}, {"gzip;q=bad", false}, {"br", false}, {"", false},
	} {
		t.Run(tc.header, func(t *testing.T) {
			if acceptsGzip([]string{tc.header}) != tc.accepted {
				t.Fatal("wrong encoding negotiation")
			}
		})
	}
	if !acceptsGzip([]string{"br", "gzip"}) {
		t.Fatal("multiple field values lost")
	}
}

func TestCompressionPoolDoesNotMixConcurrentResponses(t *testing.T) {
	handler := Compression(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, strings.Repeat(r.URL.Query().Get("id"), 500))
	}))
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := strconv.Itoa(100 + i)
			req := httptest.NewRequest("GET", "/v1/resolve?id="+id, nil)
			req.Header.Set("Accept-Encoding", "gzip")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			reader, err := gzip.NewReader(rec.Body)
			if err != nil {
				t.Error(err)
				return
			}
			body, err := io.ReadAll(reader)
			reader.Close()
			if err != nil || string(body) != strings.Repeat(id, 500) {
				t.Error("mixed/corrupted concurrent gzip payload")
			}
		}(i)
	}
	wg.Wait()
}

func TestCompressionPreservesBytesStatusAndHeaders(t *testing.T) {
	body := `{"credits":"` + strings.Repeat("producer, performer, composer ", 200) + `"}`
	for _, tc := range []struct {
		name, encoding, path, contentType, existing, cache, method string
		size                                                       int
		status                                                     int
		compressed                                                 bool
	}{
		{name: "large", encoding: "gzip", size: len(body), status: 200, compressed: true},
		{name: "plain", size: len(body), status: 200},
		{name: "disabled", encoding: "gzip;q=0", size: len(body), status: 200},
		{name: "small", encoding: "gzip", size: 50, status: 200},
		{name: "non-json", encoding: "gzip", contentType: "text/plain", size: len(body), status: 200},
		{name: "existing", encoding: "gzip", existing: "br", size: len(body), status: 200},
		{name: "no-transform", encoding: "gzip", cache: "no-transform", size: len(body), status: 200},
		{name: "metrics", encoding: "gzip", path: "/metrics", size: len(body), status: 200},
		{name: "head", encoding: "gzip", method: "HEAD", size: 0, status: 200},
		{name: "error", encoding: "gzip", size: len(body), status: 422, compressed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := body[:tc.size]
			handler := Compression(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ct := tc.contentType
				if ct == "" {
					ct = "application/json"
				}
				w.Header().Set("Content-Type", ct)
				w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
				if tc.existing != "" {
					w.Header().Set("Content-Encoding", tc.existing)
				}
				w.Header().Set("Cache-Control", tc.cache)
				w.Header().Add("Vary", "Origin")
				w.WriteHeader(tc.status)
				for start := 0; start < len(payload); start += 117 {
					_, _ = io.WriteString(w, payload[start:min(start+117, len(payload))])
				}
			}))
			path := tc.path
			if path == "" {
				path = "/v1/resolve"
			}
			method := tc.method
			if method == "" {
				method = "GET"
			}
			req := httptest.NewRequest(method, path, nil)
			req.Header.Set("Accept-Encoding", tc.encoding)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status %d", rec.Code)
			}
			got := rec.Body.String()
			if tc.compressed {
				if rec.Header().Get("Content-Encoding") != "gzip" || rec.Header().Get("Content-Length") != "" {
					t.Fatal("compression headers")
				}
				reader, err := gzip.NewReader(rec.Body)
				if err != nil {
					t.Fatal(err)
				}
				raw, err := io.ReadAll(reader)
				if err != nil {
					t.Fatal(err)
				}
				reader.Close()
				got = string(raw)
			} else if rec.Header().Get("Content-Encoding") != tc.existing {
				t.Fatal("unexpected encoding")
			}
			if got != payload {
				t.Fatal("response bytes changed")
			}
			if path != "/metrics" && !strings.Contains(strings.Join(rec.Header().Values("Vary"), ","), "Accept-Encoding") {
				t.Fatal("encoding vary missing")
			}
		})
	}
}
