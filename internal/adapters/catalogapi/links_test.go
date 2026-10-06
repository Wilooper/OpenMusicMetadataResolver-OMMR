package catalogapi

import "testing"

func TestNormalizeRetainsOnlyMatchingProviderSongURL(t *testing.T) {
	for _, tt := range []struct{ name, id, url string }{
		{"amazonmusic", "B084KPC3Q7", "https://music.amazon.com/albums/B084KP4NBH/?trackAsin=B084KPC3Q7"},
		{"tidal", "123", "https://tidal.com/track/123"},
		{"qobuz", "123", "https://open.qobuz.com/track/123"},
		{"pandora", "TR:151362660", "https://www.pandora.com/artist/album/song/TR%3A151362660"},
	} {
		track := normalize(map[string]any{"id": tt.id, "title": "Song", "url": tt.url}, tt.name)
		if track.IDs[tt.name+"_url"] != tt.url {
			t.Errorf("%s share URL discarded", tt.name)
		}
		track = normalize(map[string]any{"id": "other", "title": "Song", "url": tt.url}, tt.name)
		if track.IDs[tt.name+"_url"] != "" {
			t.Errorf("%s wrong track URL retained", tt.name)
		}
	}
	track := normalize(map[string]any{"id": "123", "attributes": map[string]any{"name": "Song", "externalLinks": []any{map[string]any{"href": "https://tidal.com/track/123"}}}}, "tidal")
	if track.IDs["tidal_url"] != "https://tidal.com/track/123" {
		t.Fatal("TIDAL external link lost")
	}
}
