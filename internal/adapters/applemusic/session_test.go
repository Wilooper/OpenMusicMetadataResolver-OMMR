package applemusic

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type sessionTransport func(*http.Request) (*http.Response, error)

func (f sessionTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestAppleSessionValidatesUserAndCatalogWithoutForwardingCookies(t *testing.T) {
	calls := 0
	p := NewWithSession("developer-token", "user-token", "in")
	p.httpClient = &http.Client{Transport: sessionTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "api.music.apple.com" || r.Header.Get("Cookie") != "" || r.Header.Get("Music-User-Token") != "user-token" || r.Header.Get("Authorization") != "Bearer developer-token" {
			t.Error("incorrect Apple session headers")
		}
		body := `{"data":[{"id":"in"}]}`
		if strings.Contains(r.URL.Path, "search") {
			body = `{"results":{"songs":{"data":[]}}}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}, Request: r}, nil
	})}
	if err := p.CheckCredentials(context.Background()); err != nil || calls != 2 {
		t.Fatalf("session probe failed: %v", err)
	}
	if err := NewWithSession("", "cookie-user-token", "in").CheckCredentials(context.Background()); err == nil {
		t.Fatal("Apple user token alone was accepted")
	}
}
