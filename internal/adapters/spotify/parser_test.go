package spotify

import (
	"strings"
	"testing"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/provider"
)

const sampleNextData = `<!doctype html><html><head><script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"state":{"data":{"entity":{"name":"Tu","uri":"spotify:track:4cOdK2wGLETKBW3PvgPWqT","id":"4cOdK2wGLETKBW3PvgPWqT","duration":218400,"duration_ms":218400,"explicit":false,"isrc":"QZRP52317311","trackNumber":1,"discNumber":1,"preview_url":"https://p.scdn.co/mp3-preview/abc","artists":[{"name":"Talwiinder","uri":"spotify:artist:abc","id":"artist123"}],"album":{"name":"Tu - Single","uri":"spotify:album:alb123","id":"alb123","images":[{"url":"https://i.scdn.co/image/high","height":640,"width":640},{"url":"https://i.scdn.co/image/low","height":64,"width":64}],"releaseDate":{"isoString":"2024-06-21","precision":"day"},"totalTracks":1,"label":"Warner Music","copyright":[{"text":"(C) 2024 Warner Music","type":"C"},{"text":"(P) 2024 Warner Music","type":"P"}]}}}}}}}</script></head></html>`

func TestParseEmbedNextData(t *testing.T) {
	track, err := ParseEmbedNextData("4cOdK2wGLETKBW3PvgPWqT", []byte(sampleNextData))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if track == nil {
		t.Fatalf("expected a parsed track")
	}
	if track.Title != "Tu" {
		t.Errorf("expected title 'Tu', got %q", track.Title)
	}
	if len(track.Artists) != 1 || track.Artists[0].Name != "Talwiinder" {
		t.Errorf("expected artist Talwiinder, got %+v", track.Artists)
	}
	if track.DurationMS != 218400 {
		t.Errorf("expected duration 218400, got %d", track.DurationMS)
	}
	if track.ISRC != "QZRP52317311" {
		t.Errorf("expected ISRC QZRP52317311, got %q", track.ISRC)
	}
	if track.Explicit {
		t.Errorf("expected non-explicit track")
	}
	if track.PreviewURL != "https://p.scdn.co/mp3-preview/abc" {
		t.Errorf("unexpected preview url %q", track.PreviewURL)
	}
	if track.TrackNumber != 1 || track.DiscNumber != 1 {
		t.Errorf("expected track/disc 1/1, got %d/%d", track.TrackNumber, track.DiscNumber)
	}
	if track.Label != "Warner Music" {
		t.Errorf("expected label 'Warner Music', got %q", track.Label)
	}
	if track.Album.Title != "Tu - Single" || track.Album.TotalTracks != 1 {
		t.Errorf("unexpected album %+v", track.Album)
	}
	if len(track.Copyrights) != 2 {
		t.Errorf("expected 2 copyrights, got %d", len(track.Copyrights))
	}
	if track.IDs["spotify"] != "4cOdK2wGLETKBW3PvgPWqT" {
		t.Errorf("expected spotify id in IDs map")
	}
}

func TestParseEmbedHTML(t *testing.T) {
	track, err := ParseEmbedHTML("4cOdK2wGLETKBW3PvgPWqT", sampleNextData)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if track == nil || track.Title != "Tu" {
		t.Fatalf("expected rich __NEXT_DATA__ parse, got %+v", track)
	}
}

func TestParseEmbedHTMLLegacyResource(t *testing.T) {
	html := `<html><body><script id="resource" type="application/json">{"name":"Legacy Song","type":"track"}</script></body></html>`
	track, err := ParseEmbedHTML("legacyid", html)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if track == nil || track.Title != "Legacy Song" {
		t.Fatalf("expected legacy resource parse, got %+v", track)
	}
}

func TestParseOembed(t *testing.T) {
	oembed := provider.SpotifyOembedResponse{
		Title:        "Tu by Talwiinder",
		AuthorName:   "Talwiinder",
		ThumbnailURL: "https://i.scdn.co/image/thumb",
		Width:        300,
		Height:       300,
	}
	track := ParseOembed("4cOdK2wGLETKBW3PvgPWqT", oembed)
	if track.Title != "Tu" {
		t.Errorf("expected title 'Tu', got %q", track.Title)
	}
	if track.Artists[0].Name != "Talwiinder" {
		t.Errorf("expected artist Talwiinder, got %q", track.Artists[0].Name)
	}
	if len(track.Images) != 1 {
		t.Errorf("expected 1 image, got %d", len(track.Images))
	}
}

func TestBuildSearchQuery(t *testing.T) {
	tests := []struct {
		name    string
		q       func() string
		contain string
	}{
		{"isrc", func() string {
			s, _ := buildSearchQuery(adapters.Query{ISRC: "USGBR1300001"})
			return s
		}, "isrc:"},
		{"artist_title", func() string {
			s, _ := buildSearchQuery(adapters.Query{Artist: "Daft Punk", Title: "Get Lucky"})
			return s
		}, "artist:"},
		{"empty", func() string {
			_, err := buildSearchQuery(adapters.Query{})
			if err == nil {
				return ""
			}
			return err.Error()
		}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.q()
			if tt.contain != "" && !strings.Contains(got, tt.contain) {
				t.Errorf("expected query containing %q, got %q", tt.contain, got)
			}
		})
	}
}
