package ytmusic

import (
	"net/http"
	"testing"
)

func TestCookieIsRestrictedToYouTubeHTTPS(t *testing.T) {
	i := &InnerTube{cookie: "__Secure-3PAPISID=secret; other=ok"}
	for _, target := range []string{"https://music.youtube.com.evil.test/youtubei/v1/player", "http://www.youtube.com/youtubei/v1/player"} {
		req, _ := http.NewRequest("POST", target, nil)
		i.setAuth(req)
		if req.Header.Get("Cookie") != "" || req.Header.Get("Authorization") != "" {
			t.Fatalf("cookie leaked to %s", target)
		}
	}
	req, _ := http.NewRequest("POST", "https://www.youtube.com/youtubei/v1/player", nil)
	i.setAuth(req)
	if req.Header.Get("Cookie") == "" || req.Header.Get("Authorization") == "" {
		t.Fatal("authenticated YouTube request missing headers")
	}
}
