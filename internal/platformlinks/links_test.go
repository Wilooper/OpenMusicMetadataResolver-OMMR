package platformlinks

import (
	"encoding/json"
	"testing"

	"github.com/ommr/ommr/internal/models/canonical"
)

func TestBuildPlatformIdentifiersAndURLs(t *testing.T) {
	track := canonical.Track{CanonicalID: "ommr_track_example", ISRC: "USABC2300001", IDs: map[string]string{
		"spotify": "4cOdK2wGLETKBW3PvgPWqT", "ytmusic": "x-KOXck57lc", "applemusic": "123", "deezer": "456", "qobuz": "789", "tidal": "321", "amazonmusic": "B084KPC3Q7", "pandora": "TR:151362660", "soundcloud": "m83/midnight-city", "soundcloud_track_id": "123456", "jiosaavn": "Rh0ueE1XXQM", "jiosaavn_track_id": "native123", "jiosaavn_url": "https://www.jiosaavn.com/song/get-lucky/Rh0ueE1XXQM", "musicbrainz": "12345678-1234-1234-1234-123456789abc",
	}, IdentityMatches: []canonical.IdentityMatch{{Provider: "spotify", ID: "4cOdK2wGLETKBW3PvgPWqT", Confidence: .97, Method: "isrc"}}}
	got := Build(track)
	if got.OMMRID != track.CanonicalID || got.ISRC != track.ISRC || len(got.Platforms) != 12 || len(got.UnavailablePlatforms) != 0 {
		t.Fatalf("incomplete catalog: %+v", got)
	}
	for _, provider := range []string{"spotify", "youtube", "ytmusic", "applemusic", "deezer", "qobuz", "tidal", "amazonmusic", "soundcloud", "jiosaavn", "musicbrainz"} {
		if got.Platforms[provider].URL == "" {
			t.Fatalf("missing supported URL for %s", provider)
		}
	}
	if got.Platforms["pandora"].URL != "" {
		t.Fatal("guessed Pandora share URL")
	}
	if got.Platforms["soundcloud"].ID != "123456" || got.Platforms["soundcloud"].LookupID != "m83/midnight-city" || got.Platforms["soundcloud"].IDType != "track_id" {
		t.Fatal("native SoundCloud ID lost")
	}
	if got.Platforms["jiosaavn"].ID != "native123" || got.Platforms["jiosaavn"].URLSource != "provider" {
		t.Fatal("JioSaavn native ID or share URL lost")
	}
	if got.Platforms["spotify"].Confidence != .97 || got.Platforms["spotify"].MatchMethod != "isrc" {
		t.Fatal("match evidence lost")
	}
}

func TestProviderURLMustMatchPlatformAndRecording(t *testing.T) {
	for _, tt := range []struct {
		provider, id, url string
		valid             bool
	}{
		{"applemusic", "123", "https://music.apple.com/in/album/song/456?i=123", true},
		{"applemusic", "123", "https://music.apple.com/in/song/song/123", true},
		{"applemusic", "123", "https://music.apple.com/in/album/123?i=999", false},
		{"applemusic", "123", "https://music.apple.com/in/song/song/999", false},
		{"amazonmusic", "B084KPC3Q7", "https://music.amazon.com/albums/B084KP4NBH/?trackAsin=B084KPC3Q7", true},
		{"pandora", "TR:151362660", "https://www.pandora.com/artist/album/title/TR%3A151362660", true},
		{"pandora", "TR:151362660", "https://www.pandora.com/artist/album/title/TR%3A999", false},
		{"tidal", "123", "https://tidal.com/browse/track/123", true},
		{"qobuz", "123", "https://open.qobuz.com/track/123", true},
		{"qobuz", "123", "https://open.qobuz.com.evil.test/track/123", false},
		{"qobuz", "123", "https://user:secret@open.qobuz.com/track/123", false},
		{"qobuz", "123", "https://open.qobuz.com/track/123?token=secret", false},
		{"qobuz", "123", "https://open.qobuz.com/track/123#secret", false},
		{"soundcloud", "m83/midnight-city", "https://soundcloud.com/m83/midnight-city", true},
		{"soundcloud", "m83/midnight-city", "https://soundcloud.com/m83/other", false},
	} {
		if got := validURL(tt.provider, tt.id, tt.url); got != tt.valid {
			t.Errorf("validURL(%s,%s)=%t;want %t", tt.provider, tt.url, got, tt.valid)
		}
	}
}

func TestInvalidIDsAndUnresolvedPlatformsHaveNoInventedLinks(t *testing.T) {
	track := canonical.Track{IDs: map[string]string{"ytmusic": "bad&v=other", "spotify": "not-a-track-id", "deezer": "123/../../evil", "unknown": "123"}}
	got := Build(track)
	if len(got.Platforms) != 0 || len(got.UnavailablePlatforms) != 12 || got.OMMRID != "" || got.ISRC != "" {
		t.Fatalf("fabricated links: %+v", got)
	}
	Attach(&track)
	var stored Catalog
	if err := json.Unmarshal(track.Extensions["platform_links"], &stored); err != nil || len(stored.Platforms) != 0 {
		t.Fatalf("invalid extension: %v", err)
	}
}

func TestWrongProviderURLDoesNotReplaceSafeTemplate(t *testing.T) {
	track := canonical.Track{IDs: map[string]string{"tidal": "123", "tidal_url": "https://tidal.com/track/999"}}
	got := Build(track).Platforms["tidal"]
	if got.URL != "https://tidal.com/track/123" || got.URLSource != "id_template" {
		t.Fatal("wrong recording URL retained")
	}
}

func TestAppleStorefrontURLKeepsSongIDAndDropsTracking(t *testing.T) {
	track := canonical.Track{IDs: map[string]string{"applemusic": "123"}}
	SetProviderURL(&track, "applemusic", "https://music.apple.com/in/album/song/456?i=123&uo=4&at=tracking")
	got := Build(track).Platforms["applemusic"]
	if got.URL != "https://music.apple.com/in/album/song/456?i=123" || got.URLSource != "provider" {
		t.Fatalf("storefront share URL lost: %+v", got)
	}
	track.IDs["applemusic_url"] = "https://music.apple.com/in/song/song/123?token=private"
	if got := Build(track).Platforms["applemusic"]; got.URLSource != "id_template" {
		t.Fatal("secret query retained")
	}
}
