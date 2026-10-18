package korean

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ommr/ommr/internal/adapters"
)

func TestCatalogSearchAndExactDetail(t *testing.T) {
	for _, name := range []string{"bugs", "genie", "melon"} {
		t.Run(name, func(t *testing.T) {
			search := map[string]string{
				"bugs":  `<table><tr trackId="123"><th><p class="title"><a>Song</a></p></th><td><a href="/artist/7">Singer</a><a class="album" href="/album/9">Album</a></td></tr></table>`,
				"genie": `<table><tr songid="123"><td><a class="title"><span class="icon">TITLE</span>Song</a><a onclick="fnViewArtist('7')">Singer</a><a class="albumtitle" onclick="fnViewAlbumLayer('9')">Album</a></td></tr></table>`,
				"melon": `<table><tr><td><button data-song-no="123"></button><a href="javascript:playSong('1',123)">Song</a><a href="javascript:goArtistDetail('7')">Singer</a><a href="javascript:goAlbumDetail('9')">Album</a></td></tr></table>`,
			}[name]
			detail := `<script type="application/ld+json">{"@type":"MusicRecording","sameAs":"https://music.bugs.co.kr/track/123","name":"Song","duration":"PT3M19S","datePublished":"2020-08-21","byArtist":{"name":"Singer"},"inAlbum":{"name":"Album","url":"https://music.bugs.co.kr/album/9"}}</script><table class="basicInfo"><tr><th>아티스트</th><td><a href="/artist/7">Singer</a></td></tr><tr><th>작사</th><td>Writer</td></tr><tr><th>Extra catalog field</th><td>Retained value</td></tr></table>`
			if name == "melon" {
				detail = strings.ReplaceAll(detail, "https://music.bugs.co.kr/track/123", "http://www.melon.com/song/detail.htm?songId=123")
			}
			if name == "genie" {
				detail = `<script>var iSnsSeq = '123';</script><div class="song-main-infos"><h2 class="name">Song</h2><ul><li><span class="attr"><img src="txt_5.png"></span><span class="value"><a onclick="fnGoMore('artistInfo','7')">Singer</a></span></li><li><span class="attr"><img src="txt_6.png"></span><span class="value"><a onclick="fnGoMore('albumInfo','9')">Album</a></span></li><li><span class="attr"><img src="txt_8.png"></span><span class="value">03:19</span></li><li><span class="attr"><img src="txt_20.png"></span><span class="value">Writer</span></li></ul></div>`
			}
			body := search
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer server.Close()
			p := New(name)
			p.base = server.URL
			p.client = server.Client()
			result, err := p.Search(context.Background(), adapters.Query{Artist: "Singer", Title: "Song"})
			if err != nil || len(result) != 1 || result[0].Track.Artists[0].IDs[name] != "7" || result[0].Track.Album.IDs[name] != "9" {
				t.Fatalf("search: %+v %v", result, err)
			}
			body = detail
			rich, err := p.FetchByID(context.Background(), name, "123")
			if err != nil || rich.Track.DurationMS != 199000 || rich.Track.IDs[name] != "123" || len(rich.Track.Credits) == 0 {
				t.Fatalf("detail: %+v %v", rich, err)
			}
			var record map[string]any
			_ = json.Unmarshal(rich.RawResponse, &record)
			if name != "genie" && record["fields"].(map[string]any)["Extra catalog field"] != "Retained value" {
				t.Fatal("source extras discarded")
			}
			if _, err = p.FetchByID(context.Background(), name, "999"); err == nil {
				t.Fatal("wrong recording accepted")
			}
			body = `<html><h1>Site Unavailable</h1></html>`
			if _, err = p.Search(context.Background(), adapters.Query{Title: "Song"}); err == nil {
				t.Fatal("HTML failure became empty success")
			}
			if _, err = p.FetchByID(context.Background(), name, "../123"); err == nil {
				t.Fatal("unsafe catalog identifier")
			}
		})
	}
}
