package albums

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ommr/ommr/internal/cache/disk"
	"github.com/ommr/ommr/internal/providerhealth"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func reply(body string) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
}
func stub(body string) *Service {
	s := New(nil, nil)
	s.HTTP.Transport = transport(func(*http.Request) (*http.Response, error) { return reply(body) })
	return s
}

// Constructed protocol fixtures exercise boundary cases, not live provider availability.
func TestDeezerPaginationKeepsOrderAndNeverFollowsNextURL(t *testing.T) {
	s := New(nil, nil)
	var calls, waits int
	s.Wait = func(context.Context, string) error { waits++; return nil }
	s.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "api.deezer.com" {
			t.Fatal("followed foreign URL")
		}
		if r.URL.Path == "/album/123" {
			return reply(`{"id":123,"title":"Album","nb_tracks":3,"label":"Label","upc":"00123","genres":{"data":[{"name":"Pop"}]}}`)
		}
		switch r.URL.Query().Get("index") {
		case "0":
			return reply(`{"data":[{"id":10,"title":"First","duration":12},{"id":11,"title":"Second"}],"next":"https://foreign.invalid/private"}`)
		case "2":
			return reply(`{"data":[{"id":12,"title":"Third","isrc":"USABC1234567"}]}`)
		}
		return nil, errors.New("unexpected page")
	})
	r, e := s.Resolve(context.Background(), Query{Provider: "deezer", ID: "123", Limit: 1000})
	if e != nil || r.Album == nil || !r.Album.TrackList.Complete || len(r.Album.Tracks) != 3 || calls != 3 || waits != 3 {
		t.Fatalf("%+v %v calls=%d", r, e, calls)
	}
	if r.Album.Tracks[2].Position != 3 || r.Album.UPC != "00123" || r.Album.ProviderData["genres"] == nil {
		t.Fatal("lost order or catalog extras")
	}
}
func TestDeezerPageFailureRetainsPartialAlbumAndMarksHealth(t *testing.T) {
	s := New(nil, nil)
	s.Health = providerhealth.New(1, time.Hour)
	s.Health.Set("deezer", "active", "public", "absent", "")
	s.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/album/123" {
			return reply(`{"id":123,"title":"Album","nb_tracks":2}`)
		}
		if r.URL.Query().Get("index") == "0" {
			return reply(`{"data":[{"id":1,"title":"One"}],"next":"next"}`)
		}
		return nil, errors.New("private upstream URL")
	})
	r, e := s.Resolve(context.Background(), Query{Provider: "deezer", ID: "123", Limit: 1000})
	if e != nil || r.Album == nil || len(r.Album.Tracks) != 1 || r.Album.TrackList.Complete || r.Album.TrackList.Reason != "upstream_error" || s.Health.Available("deezer") {
		t.Fatalf("%+v %v", r, e)
	}
	b, _ := json.Marshal(r)
	if strings.Contains(string(b), "private") {
		t.Fatal("upstream error leaked")
	}
}
func TestRepeatedPageCannotClaimComplete(t *testing.T) {
	s := New(nil, nil)
	s.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/album/123" {
			return reply(`{"id":123,"title":"A","nb_tracks":2}`)
		}
		return reply(`{"data":[{"id":1,"title":"One"}],"next":"next"}`)
	})
	r, e := s.Resolve(context.Background(), Query{Provider: "deezer", ID: "123", Limit: 1000})
	if e != nil || r.Album.TrackList.Complete || len(r.Album.Tracks) != 1 {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestMusicBrainzReleasePreservesDiscsAndRepeatedRecording(t *testing.T) {
	const id = "59211ea4-ffd2-4ad9-9a4e-941d3148024a"
	s := stub(`{"id":"` + id + `","title":"Deluxe","date":"2024-01","barcode":"001234","label-info":[{"catalog-number":"TEST"}],"media":[{"position":1,"format":"CD","track-count":1,"tracks":[{"id":"entry-one","position":1,"number":"A1","title":"Album edit","recording":{"id":"same-recording","title":"Recording","length":120000,"isrcs":["USABC1234567"]}}]},{"position":2,"title":"Bonus","format":"CD","track-count":1,"tracks":[{"id":"entry-two","position":1,"number":"1","recording":{"id":"same-recording","title":"Recording"}}]}]}`)
	r, e := s.Resolve(context.Background(), Query{Provider: "musicbrainz", ID: id, Limit: 1000})
	if e != nil || !r.Album.TrackList.Complete || *r.Album.TrackList.Total != 2 || len(r.Album.Discs) != 2 {
		t.Fatalf("%+v %v", r, e)
	}
	a := r.Album
	if a.Tracks[0].Title != "Album edit" || a.Tracks[0].TrackNumber != "A1" || a.Tracks[1].DiscNumber != 2 || a.Tracks[0].RecordingID != a.Tracks[1].RecordingID || a.Tracks[0].ID == a.Tracks[1].ID || a.ProviderData["label-info"] == nil {
		t.Fatal("lost release identity/order/context")
	}
	r, e = s.Resolve(context.Background(), Query{Provider: "musicbrainz", ID: id, Limit: 1})
	if e != nil || r.Album.TrackList.Complete || r.Album.TrackList.Reason != "limit" || *r.Album.TrackList.Total != 2 {
		t.Fatal("limit misreported")
	}
}
func TestMusicBrainzMissingMediumCannotClaimComplete(t *testing.T) {
	const id = "59211ea4-ffd2-4ad9-9a4e-941d3148024a"
	s := stub(`{"id":"` + id + `","title":"A","media":[{"position":1,"track-count":1,"tracks":[{"id":"a","title":"A","position":1}]},{"position":2,"track-count":2}]}`)
	r, e := s.Resolve(context.Background(), Query{Provider: "musicbrainz", ID: id, Limit: 1000})
	if e != nil || r.Album.TrackList.Complete || *r.Album.TrackList.Total != 3 {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestJioBothFormatsAndSanitizedExtras(t *testing.T) {
	for _, body := range []string{
		`{"id":"123","title":"A &amp; B","list_count":"1","more_info":{"artistMap":{"primary_artists":[{"id":"9","name":"Singer"}]}},"list":[{"id":"abcdefgh","title":"Song","more_info":{"album_id":"123","duration":"163","artistMap":{"primary_artists":[{"name":"Singer"}],"artists":[{"name":"Composer","role":"music"}]},"encrypted_media_url":"PRIVATE"}}],"language":"punjabi","cookie":"PRIVATE"}`,
		`{"id":123,"name":"A &amp; B","song_count":1,"songs":[{"id":"abcdefgh","song":"Song","duration":163,"albumid":123,"primary_artists":"Singer","music":"Composer","encrypted_media_url":"PRIVATE"}]}`,
	} {
		s := stub(body)
		r, e := s.Resolve(context.Background(), Query{Provider: "jiosaavn", ID: "123", Limit: 1000})
		if e != nil || !r.Album.TrackList.Complete || r.Album.Title != "A & B" || r.Album.Tracks[0].DurationMS != 163000 || len(r.Album.Tracks[0].Artists) != 1 {
			t.Fatalf("%+v %v", r, e)
		}
		b, _ := json.Marshal(r)
		if strings.Contains(string(b), "PRIVATE") || !strings.Contains(string(b), "Composer") {
			t.Fatalf("extras/privacy: %s", b)
		}
	}
}
func TestUnknownTotalAndWrongIdentity(t *testing.T) {
	s := stub(`{"id":123,"title":"A","songs":[{"id":"abcdefgh","song":"Song"}]}`)
	r, e := s.Resolve(context.Background(), Query{Provider: "jiosaavn", ID: "123", Limit: 1000})
	if e != nil || r.Album.TrackList.Complete || r.Album.TrackList.Total != nil {
		t.Fatal("invented total")
	}
	s = stub(`{"id":456,"title":"Wrong album","nb_tracks":0}`)
	r, e = s.Resolve(context.Background(), Query{Provider: "deezer", ID: "123", Limit: 1000})
	if !errors.Is(e, ErrResponse) || r.Album != nil {
		t.Fatal("wrong album accepted")
	}
}
func appleFixture() string {
	return `{"results":[{"wrapperType":"collection","collectionId":123,"collectionName":"Album","trackCount":2,"copyright":"Label","artistName":"Artist"},{"wrapperType":"track","collectionId":123,"trackId":12,"trackName":"Second","discNumber":2,"trackNumber":1},{"wrapperType":"track","collectionId":123,"trackId":11,"trackName":"First","discNumber":1,"trackNumber":1}]}`
}
func TestAppleOrderLimitAndCacheIsolation(t *testing.T) {
	backend, err := disk.NewDiskCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	s := New(nil, backend)
	var calls atomic.Int32
	s.HTTP.Transport = transport(func(*http.Request) (*http.Response, error) { calls.Add(1); return reply(appleFixture()) })
	q := Query{Provider: "applemusic", ID: "123", Limit: 1000}
	r, e := s.Resolve(context.Background(), q)
	if e != nil || !r.Album.TrackList.Complete || r.Album.Tracks[0].ID != "11" {
		t.Fatalf("%+v %v", r, e)
	}
	observed := r.Album.ObservedAt
	r, e = s.Resolve(context.Background(), q)
	if e != nil || !r.Source.Cached || calls.Load() != 1 || !r.Album.ObservedAt.Equal(observed) {
		t.Fatal("cache lost observation or repeated lookup")
	}
	q.Limit = 1
	r, e = s.Resolve(context.Background(), q)
	if e != nil || len(r.Album.Tracks) != 1 || r.Album.TrackList.Complete || r.Album.TrackList.Reason != "limit" || *r.Album.TrackList.Total != 2 {
		t.Fatal("limit cache poisoned")
	}
	q.BypassCache = true
	_, _ = s.Resolve(context.Background(), q)
	s.Storefront = "gb"
	q.BypassCache = false
	_, _ = s.Resolve(context.Background(), q)
	if calls.Load() != 4 {
		t.Fatalf("cache variants: %d", calls.Load())
	}
}
func TestInputTransportAndBudgetBoundaries(t *testing.T) {
	for _, q := range []Query{{Provider: "deezer", ID: "https://foreign.invalid", Limit: 1000}, {Provider: "musicbrainz", ID: "123", Limit: 1000}, {Provider: "spotify", ID: "123", Limit: 1000}, {Provider: "deezer", ID: "123", Limit: 1001}} {
		if Validate(q) == nil {
			t.Fatal("invalid query accepted")
		}
	}
	for _, body := range []string{"<html>Site Unavailable</html>", `{} {}`, `{"error":{"message":"private"}}`, strings.Repeat("x", (4<<20)+1)} {
		s := stub(body)
		r, e := s.Resolve(context.Background(), Query{Provider: "deezer", ID: "123", Limit: 1000})
		if e == nil || r.Album != nil {
			t.Fatal("invalid response accepted")
		}
	}
	s := New(func(context.Context, string) error { return fmt.Errorf("budget") }, nil)
	s.Health = providerhealth.New(1, time.Hour)
	s.Health.Set("deezer", "active", "public", "absent", "")
	_, e := s.Resolve(context.Background(), Query{Provider: "deezer", ID: "123", Limit: 1000})
	if !errors.Is(e, errBudget) || !s.Health.Available("deezer") {
		t.Fatal("local budget counted as provider failure")
	}
	s.Available = func(string) bool { return false }
	_, e = s.Resolve(context.Background(), Query{Provider: "deezer", ID: "123", Limit: 1000})
	if !errors.Is(e, ErrUnavailable) {
		t.Fatal("disabled provider loaded")
	}
	var calls int
	s = New(nil, nil)
	s.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": {"https://foreign.invalid"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	_, e = s.Resolve(context.Background(), Query{Provider: "deezer", ID: "123", Limit: 1000})
	if e == nil || calls != 1 {
		t.Fatal("redirect followed")
	}
}
