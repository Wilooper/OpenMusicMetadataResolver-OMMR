package albums

import (
	"context"
	"encoding/json"
	"testing"
)

func TestAppleCoverageRejectsCountOnlyCompleteness(t *testing.T) {
	for _, tc := range []struct {
		name       string
		positions  []int
		ids        []int
		discs      []int
		counts     []any
		discCounts []any
		complete   bool
	}{
		{name: "valid", positions: []int{1, 2}, complete: true},
		{name: "missing middle", positions: []int{1, 3}},
		{name: "missing first", positions: []int{2, 3}},
		{name: "duplicate position", positions: []int{1, 1}},
		{name: "duplicate native ID", positions: []int{1, 2}, ids: []int{11, 11}},
		{name: "missing disc", positions: []int{1, 2}, discCounts: []any{2, 2}},
		{name: "missing first disc", positions: []int{1, 2}, discs: []int{2, 2}},
		{name: "conflicting disc counts", positions: []int{1, 2}, discCounts: []any{1, 2}},
		{name: "disc total exceeds rows", positions: []int{1, 2}, counts: []any{3, 3}},
		{name: "conflicting track counts", positions: []int{1, 2}, counts: []any{2, 3}},
		{name: "later missing count preserves declaration", positions: []int{1, 2}, counts: []any{2, nil}, complete: true},
		{name: "two discs", positions: []int{1, 1}, discs: []int{1, 2}, counts: []any{1, 1}, discCounts: []any{2, 2}, complete: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := []any{map[string]any{"wrapperType": "collection", "collectionId": 123, "collectionName": "Album", "trackCount": 2}}
			for i, pos := range tc.positions {
				id, disc := 11+i, 1
				var count, discCount any = 2, 1
				if tc.ids != nil {
					id = tc.ids[i]
				}
				if tc.discs != nil {
					disc = tc.discs[i]
				}
				if tc.counts != nil {
					count = tc.counts[i]
				}
				if tc.discCounts != nil {
					discCount = tc.discCounts[i]
				}
				rows = append(rows, map[string]any{"wrapperType": "track", "collectionId": 123, "trackId": id, "trackName": "Song", "discNumber": disc, "trackNumber": pos, "trackCount": count, "discCount": discCount})
			}
			body, _ := json.Marshal(map[string]any{"results": rows})
			r, err := stub(string(body)).Resolve(context.Background(), Query{Provider: "applemusic", ID: "123", Limit: 1000})
			if err != nil || r.Album == nil {
				t.Fatalf("resolve: %+v %v", r, err)
			}
			if r.Album.TrackList.Complete != tc.complete {
				t.Fatalf("coverage: %+v", r.Album.TrackList)
			}
			if !tc.complete && r.Album.TrackList.Reason != "incomplete_response" {
				t.Fatalf("reason: %+v", r.Album.TrackList)
			}
			seen := map[string]bool{}
			for _, track := range r.Album.Tracks {
				if seen[track.ID] {
					t.Fatal("duplicate ID returned")
				}
				seen[track.ID] = true
			}
		})
	}
}

func TestAppleDiscCoverageRemainsSeparateFromCollection(t *testing.T) {
	const body = `{"results":[{"wrapperType":"collection","collectionId":123,"collectionName":"Album","trackCount":3},{"wrapperType":"track","collectionId":123,"trackId":11,"trackName":"First","discNumber":1,"trackNumber":1,"trackCount":2,"discCount":1},{"wrapperType":"track","collectionId":123,"trackId":12,"trackName":"Second","discNumber":1,"trackNumber":2,"trackCount":2,"discCount":1}]}`
	for _, limit := range []int{1000, 1} {
		r, err := stub(body).Resolve(context.Background(), Query{Provider: "applemusic", ID: "123", Limit: limit})
		if err != nil {
			t.Fatal(err)
		}
		a := r.Album
		if a.TrackList.Complete || *a.TrackList.Total != 3 {
			t.Fatalf("collection total changed: %+v", a.TrackList)
		}
		d := a.Discs[0].TrackList
		if d == nil || *d.Total != 2 || d.Returned != len(a.Tracks) {
			t.Fatalf("disc coverage: %+v", d)
		}
		if limit == 1000 && (!d.Complete || d.Reason != "") {
			t.Fatalf("complete disc: %+v", d)
		}
		if limit == 1 && (d.Complete || d.Reason != "limit") {
			t.Fatalf("limited disc: %+v", d)
		}
		for _, n := range a.MusicGraph.Nodes {
			if limit == 1 && n.NativeID == "123/1/2/12" {
				t.Fatal("trimmed entry leaked into graph")
			}
		}
	}
}

func TestAppleUnknownDiscTotalAndLimitedLaterDisc(t *testing.T) {
	const body = `{"results":[{"wrapperType":"collection","collectionId":123,"collectionName":"Album","trackCount":2},{"wrapperType":"track","collectionId":123,"trackId":11,"trackName":"First","discNumber":1,"trackNumber":1},{"wrapperType":"track","collectionId":123,"trackId":12,"trackName":"Second","discNumber":2,"trackNumber":1,"trackCount":1}]}`
	r, err := stub(body).Resolve(context.Background(), Query{Provider: "applemusic", ID: "123", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	first, second := r.Album.Discs[0].TrackList, r.Album.Discs[1].TrackList
	if first.Total != nil || first.Complete || first.Reason != "unknown_total" {
		t.Fatalf("invented disc total: %+v", first)
	}
	if second.Returned != 0 || second.Complete || second.Reason != "limit" {
		t.Fatalf("unreturned disc: %+v", second)
	}
}

func TestAppleNumericSlotNormalization(t *testing.T) {
	s := stub(`{"results":[{"wrapperType":"collection","collectionId":123,"collectionName":"Album","trackCount":2},{"wrapperType":"track","collectionId":123,"trackId":11,"trackName":"First","discNumber":1,"trackNumber":"01"},{"wrapperType":"track","collectionId":123,"trackId":12,"trackName":"Duplicate slot","discNumber":1,"trackNumber":1}]}`)
	r, err := s.Resolve(context.Background(), Query{Provider: "applemusic", ID: "123", Limit: 1000})
	if err != nil || len(r.Album.Tracks) != 1 || r.Album.TrackList.Complete {
		t.Fatalf("duplicate normalized slot accepted: %+v %v", r, err)
	}
	if r.Album.Tracks[0].TrackNumber != "01" {
		t.Fatal("printed source number lost")
	}
}
