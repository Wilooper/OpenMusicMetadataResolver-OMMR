package idresolver

import (
	"errors"
	"reflect"
	"testing"

	"github.com/ommr/ommr/internal/adapters"
)

func TestSupportedTrackLinks(t *testing.T) {
	for _, tc := range []struct{ raw, provider, field, id string }{
		{"https://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT?si=tracking", "spotify", "spotify_id", "4cOdK2wGLETKBW3PvgPWqT"},
		{"spotify:track:4cOdK2wGLETKBW3PvgPWqT", "spotify", "spotify_id", "4cOdK2wGLETKBW3PvgPWqT"},
		{"https://open.spotify.com/intl-en/track/4cOdK2wGLETKBW3PvgPWqT", "spotify", "spotify_id", "4cOdK2wGLETKBW3PvgPWqT"},
		{"https://music.youtube.com/watch?v=dQw4w9WgXcQ&si=tracking", "ytmusic", "youtube_id", "dQw4w9WgXcQ"},
		{"https://youtu.be/dQw4w9WgXcQ?t=30", "ytmusic", "youtube_id", "dQw4w9WgXcQ"},
		{"https://www.youtube.com/shorts/dQw4w9WgXcQ", "ytmusic", "youtube_id", "dQw4w9WgXcQ"},
		{"https://music.apple.com/us/album/p-pop-culture/1829591921?i=1829591986&ls=1", "applemusic", "apple_id", "1829591986"},
		{"https://music.apple.com/in/song/mera-mann/12345", "applemusic", "apple_id", "12345"},
		{"https://www.deezer.com/ja/track/3495188951?utm_source=share", "deezer", "deezer_id", "3495188951"},
		{"https://www.jiosaavn.com/song/mera-mann/ERIkRAwJf38?utm_source=share#top", "jiosaavn", "jiosaavn_url", "https://www.jiosaavn.com/song/mera-mann/ERIkRAwJf38"},
		{"https://soundcloud.com/m83/midnight-city?in=user/sets/favorites", "soundcloud", "soundcloud_id", "m83/midnight-city"},
		{"https://musicbrainz.org/recording/047EA202-B98D-46AE-97F7-0180A20EE5CF", "musicbrainz", "musicbrainz_id", "047ea202-b98d-46ae-97f7-0180a20ee5cf"},
		{"https://listen.tidal.com/browse/track/12345", "tidal", "tidal_id", "12345"},
		{"https://open.qobuz.com/track/12345", "qobuz", "qobuz_id", "12345"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := ParseTrackURL(tc.raw)
			if err != nil || got != (Link{tc.provider, tc.field, tc.id}) {
				t.Fatalf("%+v %v", got, err)
			}
		})
	}
}

func TestRejectNonTrackAndAmbiguousLinks(t *testing.T) {
	for _, raw := range []string{
		"http://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT", "https://open.spotify.com.evil.example/track/4cOdK2wGLETKBW3PvgPWqT",
		"https://open.spotify.com@127.0.0.1/track/4cOdK2wGLETKBW3PvgPWqT", "https://www.deezer.com:443/track/123",
		"https://open.spotify.com/album/4cOdK2wGLETKBW3PvgPWqT", "https://music.apple.com/us/album/p-pop-culture/1829591921",
		"https://music.youtube.com/watch?v=dQw4w9WgXcQ&v=abcdefghijk", "https://music.apple.com/us/album/x/123?i=123&i=456",
		"https://www.youtube.com/playlist?list=PLabcd1234", "https://on.soundcloud.com/short", "https://soundcloud.com/m83/sets/album",
		"https://www.jiosaavn.com/song/name%2Fother/ERIkRAwJf38", "https://www.jiosaavn.com/song/%2e%2e/ERIkRAwJf38",
		"https://musicbrainz.org/artist/047ea202-b98d-46ae-97f7-0180a20ee5cf", "https://127.0.0.1/track/123", "file:///etc/passwd", "spotify:artist:4cOdK2wGLETKBW3PvgPWqT",
	} {
		if _, err := ParseTrackURL(raw); !errors.Is(err, ErrInput) {
			t.Errorf("accepted %s: %v", raw, err)
		}
	}
}

func TestNormalizeQueryAndCacheIdentity(t *testing.T) {
	got, err := NormalizeQuery(adapters.Query{URL: "https://www.deezer.com/track/123?utm_source=share"})
	want := adapters.Query{DeezerID: "123"}
	if err != nil || got != want {
		t.Fatalf("%+v %v", got, err)
	}
	again, err := NormalizeQuery(got)
	if err != nil || !reflect.DeepEqual(got, again) {
		t.Fatal("normalization not idempotent")
	}
	for _, q := range []adapters.Query{
		{URL: "https://www.deezer.com/track/123", SpotifyID: "other"},
		{URL: "https://www.deezer.com/track/123", ISRC: "USABC2300001"},
		{SpotifyID: "https://evil.example/spotify.com/track/4cOdK2wGLETKBW3PvgPWqT"},
		{YouTubeID: "https://evil.example/watch?v=dQw4w9WgXcQ"},
		{AppleID: "https://www.deezer.com/track/123"},
		{MusicBrainzID: "not-a-recording-uuid"},
		{JioSaavnID: "azOux9HL", JioSaavnURL: "https://www.jiosaavn.com/song/mera-mann/ERIkRAwJf38"},
	} {
		if _, err := NormalizeQuery(q); !errors.Is(err, ErrInput) {
			t.Fatalf("bad query accepted %+v", q)
		}
	}
}
