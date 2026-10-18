package api

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCatalogValidationAndEndpointPolicy(t *testing.T) {
	h, _ := responseFixture(t)
	router := NewRouter(h, slog.New(slog.NewTextHandler(io.Discard, nil)), 100)
	for _, test := range []struct {
		url    string
		status int
	}{
		{"/v1/catalog?provider=ytmusic&type=song&id=abcdefghijk", 503},
		{"/v1/catalog?provider=ytmusic&type=song&id=https://evil.test", 400},
		{"/v1/catalog?provider=ytmusic&type=artist&id=UCxxxxxxxxxxxxxxxxxxxxxx&limit=101", 400},
		{"/v1/catalog?provider=unknown&type=song&id=abcdefghijk", 400},
	} {
		r := httptest.NewRecorder()
		router.ServeHTTP(r, httptest.NewRequest("GET", test.url, nil))
		if r.Code != test.status {
			t.Fatalf("%s: %d %s", test.url, r.Code, r.Body)
		}
	}
	h.cfg.API.Endpoints = map[string]bool{"catalog": false}
	for _, method := range []string{"GET", "POST"} {
		r := httptest.NewRecorder()
		url := "/v1/catalog"
		body := ""
		if method == "POST" {
			url = "/v1/query"
			body = `{"operation":"catalog","provider":"ytmusic","type":"song","id":"abcdefghijk"}`
		}
		router.ServeHTTP(r, httptest.NewRequest(method, url, strings.NewReader(body)))
		if r.Code != 404 {
			t.Fatalf("disabled catalog bypass: %d %s", r.Code, r.Body)
		}
	}
}
