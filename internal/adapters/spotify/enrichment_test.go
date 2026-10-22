package spotify

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ommr/ommr/internal/models/canonical"
)

func TestAlbumHydrationValidatesEditionAndRetainsExtraFields(t *testing.T) {
	for _, wrong := range []bool{false, true} {
		p := New(Config{AccessToken: "fixture", ExpiresAt: time.Now().Add(time.Hour).Unix()})
		p.httpClient = &http.Client{Transport: sessionTransport(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/v1/albums/album" || r.Header.Get("Authorization") != "Bearer fixture" {
				t.Errorf("album request: %s", r.URL)
			}
			id := "album"
			if wrong {
				id = "another"
			}
			body := `{"id":"` + id + `","name":"P-POP CULTURE","label":"Known Label","external_ids":{"upc":"123456789012"},"total_tracks":11,"album_type":"album","copyrights":[{"type":"P","text":"Copyright owner"}],"new_album_field":"retained"}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
		})}
		candidate := canonical.TrackCandidate{Provider: "spotify", Track: canonical.Track{Title: "Boyfriend", IDs: map[string]string{"spotify": "song"}, Album: canonical.Album{Title: "P-POP CULTURE", IDs: map[string]string{"spotify": "album"}}}}
		got, err := p.EnrichCandidate(context.Background(), candidate)
		if wrong {
			if err == nil {
				t.Fatal("different album accepted")
			}
			continue
		}
		if err != nil || got.Track.Label != "Known Label" || got.Track.Barcode != "123456789012" || got.Track.Album.TotalTracks != 11 || len(got.Track.Copyrights) != 1 {
			t.Fatalf("hydration: %+v %v", got, err)
		}
		if !strings.Contains(string(got.Track.Extensions["spotify_album"]), "new_album_field") {
			t.Fatal("extra album field discarded")
		}
		if candidate.Track.Label != "" || candidate.Track.Extensions != nil {
			t.Fatal("source candidate mutated")
		}
	}
}

func TestDirectMetadataRejectsWrongIDAndEmptyEmbed(t *testing.T) {
	p := New(Config{AccessToken: "fixture", ExpiresAt: time.Now().Add(time.Hour).Unix()})
	p.httpClient = &http.Client{Transport: sessionTransport(func(r *http.Request) (*http.Response, error) {
		body := `{"id":"wrong","name":"Other song"}`
		if strings.Contains(r.URL.Path, "/embed/") {
			body = "<html>Unavailable</html>"
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})}
	if _, err := p.fetchByWebAPI(context.Background(), "requested"); err == nil {
		t.Fatal("wrong track ID accepted")
	}
	if _, err := p.fetchByEmbed(context.Background(), "requested"); err == nil {
		t.Fatal("empty embed accepted")
	}
	if _, err := ParseEmbedNextData("other", []byte(sampleNextData)); err == nil {
		t.Fatal("embed's own ID ignored")
	}
}
