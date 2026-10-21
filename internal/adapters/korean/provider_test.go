package korean

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/musicgraph"
	"golang.org/x/net/html"
)

func TestMelonContributorSectionOutsideSongInfo(t *testing.T) {
	body := `<meta property="og:url" content="https://www.melon.com/song/detail.htm?songId=123"><div class="section_info"><div class="song_name">Song</div><div class="artist"><a href="javascript:melon.link.goArtistDetail('7')">Singer</a></div></div><div class="section_prdcr"><ul class="list_person"><li><a class="artist_name" href="javascript:melon.link.goArtistDetail('10')">Alex</a><span class="type">작곡</span></li><li><a class="artist_name" href="javascript:melon.link.goArtistDetail('11')">Alex</a><span class="type">작사</span></li><li><a class="artist_name" href="javascript:melon.link.goArtistDetail()">Last, First &amp; Partner</a><span class="type">편곡</span></li></ul></div><aside><a class="artist_name">Unrelated</a><span class="type">작곡</span></aside>`
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	p := New("melon")
	record, err := p.detail(doc, []byte(body), "123")
	if err != nil {
		t.Fatal(err)
	}
	if len(record.Credits) != 3 || record.Credits[0].ID != "10" || record.Credits[1].ID != "11" || record.Credits[2].Name != "Last, First & Partner" || record.Credits[2].ID != "" {
		t.Fatalf("contributors lost or guessed: %+v", record.Credits)
	}
	c := p.candidate(record)
	if len(c.Track.Credits) != 3 {
		t.Fatal("normalized credits missing")
	}
	var catalog map[string]any
	_ = json.Unmarshal(c.RawResponse, &catalog)
	g := musicgraph.TrackGraph(c.Track, []musicgraph.Record{{Provider: "melon", Track: c.Track, Catalog: catalog}}, nil)
	d := musicgraph.DetailedCredits(g)
	found := false
	for _, e := range d.Credits {
		if e.SourceRole == "작곡" {
			found = true
			if e.Role != "composer" || e.From != "melon:artist:10" || e.To != "melon:recording:123" {
				t.Fatalf("credit identity/scope wrong: %+v", e)
			}
		}
	}
	if !found {
		t.Fatal("original source role missing")
	}
	if _, err = p.detail(doc, []byte(body), "999"); err == nil {
		t.Fatal("foreign song credits accepted")
	}
}

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
