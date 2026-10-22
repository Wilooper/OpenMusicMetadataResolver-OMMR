package ytmusic

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type sessionTransport func(*http.Request) (*http.Response, error)

func (f sessionTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestYouTubeSessionRequiresAuthenticatedResponseShape(t *testing.T) {
	loggedOut := false
	p := NewWithCookie("SAPISID=private")
	p.httpClient = &http.Client{Transport: sessionTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "music.youtube.com" || r.Header.Get("Cookie") == "" || r.Header.Get("Origin") != "https://music.youtube.com" {
			t.Error("incorrect YouTube cookie request")
		}
		body := `{"actions":[{"openPopupAction":{"popup":{"multiPageMenuRenderer":{"header":{"activeAccountHeaderRenderer":{"accountName":{"runs":[{"text":"Account"}]}}}}}}}]}`
		if loggedOut {
			body = `{"actions":[]}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}, Request: r}, nil
	})}
	if err := p.CheckCredentials(context.Background()); err != nil {
		t.Fatal(err)
	}
	loggedOut = true
	if err := p.CheckCredentials(context.Background()); err == nil {
		t.Fatal("HTTP 200 signed-out page accepted")
	}
}
