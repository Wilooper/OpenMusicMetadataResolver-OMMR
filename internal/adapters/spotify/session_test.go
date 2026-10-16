package spotify

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type sessionTransport func(*http.Request) (*http.Response, error)

func (f sessionTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestCookieSessionIsAuthenticatedAndConfinedToWebHost(t *testing.T) {
	anonymous := false
	cookieCalls := 0
	profileCalls := 0
	client := &http.Client{Transport: sessionTransport(func(r *http.Request) (*http.Response, error) {
		body := `{"id":"valid-id"}`
		if r.URL.Host == "open.spotify.com" {
			cookieCalls++
			if r.Header.Get("Cookie") != "sp_dc=private" {
				t.Error("cookie missing")
			}
			body = fmt.Sprintf(`{"accessToken":"session-token","isAnonymous":%v,"accessTokenExpirationTimestampMs":%d}`, anonymous, time.Now().Add(time.Hour).UnixMilli())
		} else {
			if r.Header.Get("Cookie") != "" {
				t.Error("cookie leaked to API host")
			}
			if r.Header.Get("Authorization") != "Bearer session-token" {
				t.Error("missing session bearer")
			}
			profileCalls++
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}, Request: r}, nil
	})}
	p := New(Config{Cookie: "sp_dc=private"})
	p.httpClient = client
	p.tokens.client = client
	if err := p.CheckCredentials(context.Background()); err != nil || cookieCalls != 1 || profileCalls != 2 {
		t.Fatalf("session validation failed: %v", err)
	}
	anonymous = true
	p = New(Config{Cookie: "sp_dc=private"})
	p.httpClient = client
	p.tokens.client = client
	if err := p.CheckCredentials(context.Background()); err == nil {
		t.Fatal("anonymous token accepted as login")
	}
}
func TestImportedSessionTokenMustPassProtectedProfile(t *testing.T) {
	p := New(Config{AccessToken: "expired-browser-token"})
	p.httpClient = &http.Client{Transport: sessionTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 401, Body: io.NopCloser(strings.NewReader(`{}`)), Header: http.Header{}}, nil
	})}
	if err := p.CheckCredentials(context.Background()); err == nil || strings.Contains(err.Error(), "expired-browser-token") {
		t.Fatal("invalid token accepted or leaked")
	}
}

func TestAnonymousAccessFailureIsActionableAndSecretFree(t *testing.T) {
	client := &http.Client{Transport: sessionTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/token" || r.URL.Query().Get("productType") != "web-player" {
			t.Errorf("obsolete endpoint: %s", r.URL)
		}
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Error("anonymous request sent credentials")
		}
		return &http.Response{StatusCode: 400, Body: io.NopCloser(strings.NewReader(`{"error":"private-response-value"}`)), Header: http.Header{}}, nil
	})}
	manager := newTokenManager(client, "", "")
	_, err := manager.Get(context.Background())
	if err == nil || !strings.Contains(err.Error(), "configure validated credentials") || strings.Contains(err.Error(), "private-response-value") {
		t.Fatalf("unexpected failure: %v", err)
	}
}
