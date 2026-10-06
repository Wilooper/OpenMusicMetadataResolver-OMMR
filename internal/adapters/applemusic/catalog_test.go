package applemusic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ommr/ommr/internal/adapters"
)

func TestCatalogExactIDAndStructuredComposer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/catalog/us/songs/123" || r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("incorrect catalog request: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"999","type":"songs","attributes":{"name":"Wrong"}},{"id":"123","type":"songs","attributes":{"name":"Right","artistName":"Singer","albumName":"Album","composerName":"Writer","releaseDate":"2023-04-05","isrc":"USABC2300001","artwork":{"url":"https://art.example/{w}x{h}.jpg"}}}]}`))
	}))
	defer srv.Close()
	p := NewWithCatalog("test-token", "us")
	p.catalogBaseURL = srv.URL
	got, err := p.FetchByID(context.Background(), "applemusic", "123")
	if err != nil {
		t.Fatal(err)
	}
	if got.Track.Title != "Right" || got.Track.IDs["applemusic"] != "123" || got.Track.Credits[0].Roles[0] != "Composer" || !strings.Contains(got.Track.Images[0].URL, "600x600") {
		t.Fatalf("bad normalized catalog track: %+v", got.Track)
	}
}

func TestCatalogISRCRejectsOtherRecordings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("filter[isrc]") != "USABC2300001" {
			t.Errorf("wrong ISRC filter: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"1","type":"songs","attributes":{"isrc":"OTHER"}},{"id":"2","type":"songs","attributes":{"name":"Right","isrc":"USABC2300001"}}]}`))
	}))
	defer srv.Close()
	p := NewWithCatalog("token", "us")
	p.catalogBaseURL = srv.URL
	got, err := p.Search(context.Background(), adapters.Query{ISRC: "USABC2300001"})
	if err != nil || len(got) != 1 || got[0].Track.IDs["applemusic"] != "2" {
		t.Fatalf("bad ISRC candidates: %v %v", got, err)
	}
}

func TestCatalogDoesNotFollowCredentialedRedirect(t *testing.T) {
	followed := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { followed = true }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer source.Close()
	p := NewWithCatalog("private-token", "us")
	p.catalogBaseURL = source.URL
	_, err := p.FetchByID(context.Background(), "applemusic", "123")
	if err == nil || followed {
		t.Fatal("credentialed catalog redirect was followed")
	}
}
