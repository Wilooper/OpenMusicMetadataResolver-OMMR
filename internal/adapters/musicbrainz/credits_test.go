package musicbrainz

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/models/provider"
)

func TestWorkLyricistAndComposerAreActualRelations(t *testing.T) {
	var rec provider.MusicBrainzRecording
	data := []byte(`{"id":"recording-id","title":"Song","artist-credit":[{"name":"Performer","artist":{"name":"Performer"}}],"relations":[{"type":"performance","work":{"id":"work-id","iswcs":["T-123.456.789-0"],"language":"eng","relations":[{"type":"lyricist","artist":{"name":"Poet"}},{"type":"composer","artist":{"name":"Composer"}}]}}]}`)
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatal(err)
	}
	track := New().normalizeRecording(rec)
	roles := map[string]string{}
	for _, credit := range track.Credits {
		for _, role := range credit.Roles {
			roles[credit.Name] = role
		}
	}
	if roles["Poet"] != "Lyricist" || roles["Composer"] != "Composer" || roles["Performer"] != "Artist" {
		t.Fatalf("incorrect roles: %v", roles)
	}
	if track.ISWC != "T-123.456.789-0" || track.Language != "eng" || len(track.Extensions["musicbrainz_work_id"]) == 0 {
		t.Fatalf("work fields lost: %+v", track)
	}
}

type recordingTransport func(*http.Request) (*http.Response, error)

func (f recordingTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestISRCRequestsRetainRequestedCode(t *testing.T) {
	for _, mode := range []string{"lookup", "search"} {
		t.Run(mode, func(t *testing.T) {
			p := New()
			p.httpClient.Transport = recordingTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != "musicbrainz.org" || r.Header.Get("User-Agent") == "" {
					t.Fatal("invalid MusicBrainz request")
				}
				if mode == "lookup" {
					if r.URL.Path != "/ws/2/isrc/USABC2300001" || !strings.Contains(r.URL.Query().Get("inc"), "isrcs") {
						t.Fatalf("ISRC lookup missing recording codes: %s", r.URL)
					}
				} else if r.URL.Query().Get("query") != "isrc:USABC2300001" {
					t.Fatalf("invalid ISRC query: %s", r.URL)
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"recordings":[{"id":"recording-id","title":"Song","isrcs":["OTHER","USABC2300001"],"artist-credit":[{"name":"Singer"}]}]}`))}, nil
			})
			if mode == "lookup" {
				got, err := p.FetchByID(context.Background(), "isrc", "USABC2300001")
				if err != nil {
					t.Fatal(err)
				}
				if got.Track.ISRC != "USABC2300001" {
					t.Fatal("lookup chose wrong ISRC")
				}
			} else {
				got, err := p.Search(context.Background(), adapters.Query{ISRC: "USABC2300001"})
				if err != nil {
					t.Fatal(err)
				}
				if len(got) != 1 || got[0].Track.ISRC != "USABC2300001" {
					t.Fatal("search chose wrong ISRC")
				}
			}
		})
	}
}

func TestMultipleWorksDoNotAssertSingleComposition(t *testing.T) {
	var rec provider.MusicBrainzRecording
	if err := json.Unmarshal([]byte(`{"id":"medley","relations":[{"work":{"id":"first","iswcs":["FIRST"],"language":"eng"}},{"work":{"id":"second","iswcs":["SECOND"],"language":"fra"}}]}`), &rec); err != nil {
		t.Fatal(err)
	}
	track := New().normalizeRecording(rec)
	if track.ISWC != "" || track.Language != "" || len(track.Extensions["musicbrainz_work_id"]) > 0 {
		t.Fatal("medley asserted arbitrary first composition")
	}
}

func TestRecordingEnrichmentRequestsWorkCreditsAndVerifiesISRC(t *testing.T) {
	p := New()
	p.httpClient.Transport = recordingTransport(func(r *http.Request) (*http.Response, error) {
		includes := strings.Fields(r.URL.Query().Get("inc"))
		for _, required := range []string{"isrcs", "artist-rels", "work-rels", "work-level-rels"} {
			found := false
			for _, include := range includes {
				if include == required {
					found = true
				}
			}
			if !found {
				t.Fatalf("recording lookup missing %s: %s", required, r.URL)
			}
		}
		if r.URL.Path != "/ws/2/recording/recording-id" {
			t.Fatalf("wrong recording lookup: %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"id":"recording-id","title":"Song","artist-credit":[{"name":"Singer"}],"isrcs":["OTHER","USABC2300001"],"relations":[{"type":"performance","work":{"id":"work-id","relations":[{"type":"lyricist","artist":{"name":"Poet"}}]}}]}`))}, nil
	})
	seed := canonical.Track{ISRC: "USABC2300001", IDs: map[string]string{"musicbrainz": "recording-id"}}
	got, err := p.Enrich(context.Background(), seed)
	if err != nil {
		t.Fatal(err)
	}
	if got.ISRC != seed.ISRC || len(got.Credits) != 2 || got.Credits[1].Roles[0] != "Lyricist" {
		t.Fatalf("incorrect recording enrichment: %+v", got)
	}
	seed.ISRC = "UNREPORTED"
	if _, err := p.Enrich(context.Background(), seed); err == nil {
		t.Fatal("unreported ISRC accepted during enrichment")
	}
}

func TestRequestedISRCIsSelectedFromAllRecordingCodes(t *testing.T) {
	recording := provider.MusicBrainzRecording{ISRCs: []string{"OTHER", "USABC2300001"}}
	if !selectISRC(&recording, "usabc2300001") || recording.ISRCs[0] != "USABC2300001" {
		t.Fatal("query ISRC lost due to provider array order")
	}
	if selectISRC(&recording, "MISMATCH") {
		t.Fatal("unreported ISRC accepted")
	}
}
