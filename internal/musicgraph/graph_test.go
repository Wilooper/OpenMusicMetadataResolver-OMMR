package musicgraph

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/ommr/ommr/internal/models/canonical"
)

func record(t *testing.T, raw string) map[string]any {
	t.Helper()
	var m map[string]any
	if e := json.Unmarshal([]byte(raw), &m); e != nil {
		t.Fatal(e)
	}
	return m
}
func find(g Graph, kind, from, to string) *Edge {
	for _, e := range g.Edges {
		if e.Type == kind && (from == "" || e.From == from) && (to == "" || e.To == to) {
			return &e
		}
	}
	return nil
}
func TestRecordingRelationsCreditScopeAndDirection(t *testing.T) {
	// Constructed protocol case: a medley has two works; writers must stay on their respective works.
	m := record(t, `{"id":"rec","title":"Medley","relations":[{"type":"remix","type-id":"reltype","direction":"backward","recording":{"id":"remix","title":"Remix"}},{"type":"producer","direction":"backward","target-credit":"P. Name","attributes":["additional"],"begin":"2020","end":"2021","ended":true,"artist":{"id":"person","name":"Producer"}},{"type":"performance","direction":"forward","attributes":["live","medley"],"work":{"id":"work-a","title":"First work","relations":[{"type":"composer","direction":"backward","artist":{"id":"composer-a","name":"Same Name"}}]}},{"type":"performance","direction":"forward","work":{"id":"work-b","relations":[{"type":"lyricist","direction":"backward","artist":{"id":"composer-b","name":"Same Name"}}]}}]}`)
	b := New("musicbrainz:recording:rec")
	b.MusicBrainz("recording", m)
	g := b.Result()
	if find(g, "remix_of", "musicbrainz:recording:remix", "musicbrainz:recording:rec") == nil {
		t.Fatal("inverse direction lost")
	}
	for _, pair := range [][2]string{{"composer-a", "work-a"}, {"composer-b", "work-b"}} {
		e := find(g, "credit", "musicbrainz:artist:"+pair[0], "musicbrainz:work:"+pair[1])
		if e == nil {
			t.Fatal("work credit missing")
		}
		if find(g, "credit", "musicbrainz:artist:"+pair[0], "musicbrainz:recording:rec") != nil {
			t.Fatal("work author incorrectly attached to recording")
		}
	}
	producer := find(g, "credit", "musicbrainz:artist:person", "musicbrainz:recording:rec")
	if producer == nil || producer.Role != "producer" || producer.CreditedAs != "P. Name" || producer.Begin != "2020" || producer.Ended == nil || !*producer.Ended || len(producer.Attributes) != 1 {
		t.Fatalf("credit context lost: %+v", producer)
	}
	if len(g.Nodes) != 7 {
		t.Fatalf("same-name artists merged: %d", len(g.Nodes))
	}
	// Input data and output are deterministic; nested relationship attributes are not mutated.
	before, _ := json.Marshal(m)
	b2 := New(g.Root)
	b2.MusicBrainz("recording", m)
	after, _ := json.Marshal(m)
	if string(before) != string(after) || !reflect.DeepEqual(g, b2.Result()) {
		t.Fatal("mutation or unstable graph")
	}
}
func TestReleaseRemasterNeverBecomesRecordingRemaster(t *testing.T) {
	b := New("musicbrainz:release:edition")
	b.MusicBrainz("release", record(t, `{"id":"edition","title":"Remastered Edition","release-group":{"id":"group","title":"Album"},"label-info":[{"label":{"id":"label","name":"Label"},"catalog-number":"CAT-1"}],"relations":[{"type":"remaster","direction":"forward","release":{"id":"original","title":"Original Edition"}}]}`))
	g := b.Result()
	if find(g, "remaster_of", "musicbrainz:release:edition", "musicbrainz:release:original") == nil || find(g, "edition_of", "musicbrainz:release:edition", "musicbrainz:release_group:group") == nil {
		t.Fatal("edition relationships missing")
	}
	if e := find(g, "released_by", "", "musicbrainz:label:label"); e == nil || e.CatalogNumber != "CAT-1" {
		t.Fatal("label context lost")
	}
	for _, n := range g.Nodes {
		if n.Type == "recording" {
			t.Fatal("invented recording remaster")
		}
	}
}
func TestNoTitleInferenceNoCrossSourceNameIdentityAndNoForeignRoot(t *testing.T) {
	track := canonical.Track{CanonicalID: "canonical", Title: "Song (Live Remix)", IDs: map[string]string{"deezer": "d"}}
	records := []Record{{Provider: "deezer", Track: canonical.Track{Title: track.Title, IDs: map[string]string{"deezer": "d"}, Credits: []canonical.Credit{{Name: "Alex", Roles: []string{"Producer"}}}}}, {Provider: "jiosaavn", Track: canonical.Track{Title: "Song", IDs: map[string]string{"jiosaavn": "j"}, Credits: []canonical.Credit{{Name: "Alex", Roles: []string{"Producer"}}}}}}
	g := TrackGraph(track, records, nil)
	artists := 0
	for _, n := range g.Nodes {
		if n.Type == "artist" {
			artists++
			if !n.Local {
				t.Fatal("name-only credit invented native ID")
			}
		}
	}
	if artists != 2 {
		t.Fatal("names joined across providers")
	}
	for _, e := range g.Edges {
		if e.Type == "remix_of" || e.Type == "recording_of" {
			t.Fatal("relationship inferred from title")
		}
	}
	wrong := []Record{{Provider: "musicbrainz", Track: canonical.Track{IDs: map[string]string{"musicbrainz": "accepted"}}, Catalog: record(t, `{"id":"foreign","relations":[{"type":"remix","direction":"forward","recording":{"id":"other"}}]}`)}}
	g = TrackGraph(track, wrong, nil)
	for _, n := range g.Nodes {
		if n.NativeID == "foreign" || n.NativeID == "other" {
			t.Fatal("foreign raw record trusted")
		}
	}
}
func TestUnknownDirectionAndRolesPreservedWithoutGuessing(t *testing.T) {
	b := New("musicbrainz:recording:a")
	b.MusicBrainz("recording", record(t, `{"id":"a","relations":[{"type":"remix","recording":{"id":"b"}},{"type":"dedication","direction":"backward","artist":{"id":"person","name":"Person"}},{"type":"instrument","direction":"backward","attributes":["guitar","electric"],"artist":{"id":"player","name":"Player"}}]}`))
	g := b.Result()
	if find(g, "remix_of", "", "") != nil {
		t.Fatal("unknown direction guessed")
	}
	if find(g, "credit", "musicbrainz:artist:person", "") != nil {
		t.Fatal("dedication treated as musical contribution")
	}
	if e := find(g, "credit", "musicbrainz:artist:player", ""); e == nil || e.Role != "instrumentalist" || len(e.Attributes) != 2 {
		t.Fatal("instrument credit lost")
	}
}
func TestGraphBoundsAndNoDanglingEdges(t *testing.T) {
	b := New("x:recording:root")
	root := b.Entity("x", "recording", "root", "Root", nil)
	for i := 0; i < MaxNodes+10; i++ {
		b.Credit("x", root, fmt.Sprint(i), "Artist", "producer", "", "", nil)
	}
	g := b.Result()
	if !g.Truncated || len(g.Nodes) > MaxNodes || len(g.Edges) > MaxEdges {
		t.Fatal("unbounded graph")
	}
	nodes := map[string]bool{}
	for _, n := range g.Nodes {
		nodes[n.ID] = true
	}
	for _, e := range g.Edges {
		if !nodes[e.From] || !nodes[e.To] {
			t.Fatal("dangling edge")
		}
	}
}

func TestForeignStructuredCreditsCannotBorrowAcceptedIdentity(t *testing.T) {
	track := canonical.Track{CanonicalID: "accepted", Title: "Song"}
	records := []Record{{Provider: "deezer", Track: canonical.Track{IDs: map[string]string{"deezer": "123", "spotify": "456"}}, Catalog: record(t, `{"id":"456","contributors":[{"id":"foreign","name":"Wrong Producer","role":"producer"}]}`)}}
	g := TrackGraph(track, records, nil)
	for _, n := range g.Nodes {
		if n.Name == "Wrong Producer" {
			t.Fatal("foreign provider record injected a credit")
		}
	}
}
