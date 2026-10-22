package jiosaavn

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ommr/ommr/internal/models/canonical"
)

func TestShareLookupMethodAndIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       bool
	}{
		{"matching", `{"songs":[` + v4Song + `]}`, true},
		{"wrong_link", `{"songs":[` + strings.ReplaceAll(v4Song, "share42", "wrong42") + `]}`, false},
		{"foreign_link", `{"songs":[` + strings.ReplaceAll(v4Song, "www.jiosaavn.com", "foreign.example") + `]}`, false},
		{"ambiguous", `{"songs":[` + v4Song + `,` + v4Song + `]}`, false},
		{"embedded", `<script type="application/json">{"songs":[` + v4Song + `]}</script>`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				q := r.URL.Query()
				if r.Method != "GET" || q.Get("__call") != "webapi.get" || q.Get("type") != "song" || q.Get("token") != "share42" || q.Get("pids") != "" {
					t.Errorf("wrong link protocol: %s", r.URL)
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			old := SearchURL
			SearchURL = srv.URL
			defer func() { SearchURL = old }()
			got, err := New().FetchByID(context.Background(), "jiosaavn_share", "share42")
			if (err == nil) != tc.want {
				t.Fatalf("success=%v, want %v: %v", err == nil, tc.want, err)
			}
			if tc.want && (got.Track.IDs["jiosaavn_track_id"] != "native42" || got.Track.IDs[ProviderName] != "share42") {
				t.Fatalf("identity changed: %+v", got)
			}
		})
	}
}

func TestAcceptedDetailUsesNativeIDAndSongsEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("__call") != "song.getDetails" || r.URL.Query().Get("pids") != "native42" {
			t.Errorf("share token sent to native lookup: %s", r.URL)
		}
		_, _ = w.Write([]byte(`{"songs":[` + v4Song + `]}`))
	}))
	defer srv.Close()
	old := SearchURL
	SearchURL = srv.URL
	defer func() { SearchURL = old }()
	got, err := New().EnrichCandidate(context.Background(), canonical.TrackCandidate{Provider: ProviderName, Track: canonical.Track{IDs: map[string]string{ProviderName: "share42", "jiosaavn_track_id": "native42", "jiosaavn_url": "https://www.jiosaavn.com/song/rock-roll/share42"}}})
	if err != nil || got.Track.Title != "Rock & Roll" {
		t.Fatalf("native hydration: %v %+v", err, got)
	}
}

func TestNativeLookupRejectsContradictoryID(t *testing.T) {
	_, _, err := selectDetail([]byte(`{"native42":`+strings.ReplaceAll(v4Song, "native42", "otherID8")+`}`), "native42", false)
	if err == nil {
		t.Fatal("keyed result overrode contradictory native id")
	}
}

func TestLegacyHydrationPreservesStructuredArtistIdentity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"native42":{"id":"native42","song":"Song","primary_artists":"Artist, With Comma","perma_url":"https://www.jiosaavn.com/song/song/share42"}}`))
	}))
	defer srv.Close()
	old := SearchURL
	SearchURL = srv.URL
	defer func() { SearchURL = old }()
	original := canonical.TrackCandidate{Provider: ProviderName, Track: canonical.Track{IDs: map[string]string{ProviderName: "share42", "jiosaavn_track_id": "native42"}, Artists: []canonical.Artist{{Name: "Artist, With Comma", Role: "main", IDs: map[string]string{ProviderName: "456"}}}}}
	detail, err := New().EnrichCandidate(context.Background(), original)
	if err != nil || len(detail.Track.Artists) != 1 || detail.Track.Artists[0].IDs[ProviderName] != "456" {
		t.Fatalf("structured artist lost: %v %+v", err, detail)
	}
}
