package metadata

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ommr/ommr/internal/models/canonical"
)

func TestRecordSelectsOnlyTheAcceptedEntityAndPreservesExtraFields(t *testing.T) {
	c := canonical.TrackCandidate{Provider: "spotify", Track: canonical.Track{Title: "Song", IDs: map[string]string{"spotify": "accepted"}}, RawResponse: []byte(`{"tracks":{"items":[{"id":"wrong","name":"Wrong"},{"id":"accepted","name":"Song","popularity":73,"new_catalog_field":{"mood":"calm"},"available_markets":["IN"],"access_token":"private","stream_url":"private-stream","external_urls":{"spotify":"https://open.spotify.com/track/accepted"}}]},"request":{"api_key":"private"}}`)}
	data, _ := json.Marshal(Record(c))
	text := string(data)
	for _, wanted := range []string{"new_catalog_field", "popularity", "available_markets", "external_urls"} {
		if !strings.Contains(text, wanted) {
			t.Errorf("lost %s", wanted)
		}
	}
	for _, forbidden := range []string{"Wrong", "private", "access_token", "stream_url", "match_score", "field_sources"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("leaked %s", forbidden)
		}
	}
}
func TestExplicitDistinguishesUnknownFromKnownClean(t *testing.T) {
	c := canonical.TrackCandidate{Provider: "deezer", Track: canonical.Track{IDs: map[string]string{"deezer": "123"}}}
	if _, known := Explicit(c); known {
		t.Fatal("missing flag treated as clean")
	}
	c.RawResponse = []byte(`{"id":123,"explicit_lyrics":false}`)
	if explicit, known := Explicit(c); explicit || !known {
		t.Fatal("known clean flag lost")
	}
	c.Track.Explicit = true
	if explicit, known := Explicit(c); !explicit || !known {
		t.Fatal("explicit flag lost")
	}
}
func TestProviderDataDoesNotMutateInputOrLoseExtensionMetadata(t *testing.T) {
	track := canonical.Track{Extensions: map[string]json.RawMessage{"new_details": json.RawMessage(`{"quality":"hi-res"}`)}}
	c := canonical.TrackCandidate{Provider: "provider", Track: track}
	Attach(&track, []canonical.TrackCandidate{c})
	if !strings.Contains(string(track.Extensions["provider_data"]), "hi-res") {
		t.Fatal("extension data lost")
	}
	if len(c.Track.Extensions["provider_data"]) > 0 {
		t.Fatal("input map was mutated")
	}
}

func TestGraphUsesVerifiedDetailsAndKeepsWorkCreditsScoped(t *testing.T) {
	candidate := canonical.TrackCandidate{Provider: "musicbrainz", Track: canonical.Track{Title: "Song", IDs: map[string]string{"musicbrainz": "accepted"}, Credits: []canonical.Credit{{Name: "Writer", Roles: []string{"Lyricist"}}}, Extensions: map[string]json.RawMessage{"recording_details": json.RawMessage(`{"id":"accepted","title":"Song","relations":[{"type":"performance","direction":"forward","work":{"id":"work","title":"Composition","relations":[{"type":"lyricist","direction":"backward","artist":{"id":"writer","name":"Writer"}}]}}]}`)}}}
	track := candidate.Track
	track.CanonicalID = "canonical"
	Attach(&track, []canonical.TrackCandidate{candidate})
	AttachGraph(&track, []canonical.TrackCandidate{candidate})
	var graph struct {
		Edges []struct{ From, To, Type, Role string }
	}
	if json.Unmarshal(track.Extensions["music_graph"], &graph) != nil {
		t.Fatal("missing graph")
	}
	found := false
	for _, e := range graph.Edges {
		if e.Role == "lyricist" {
			if e.To != "musicbrainz:work:work" {
				t.Fatal("work credit flattened onto recording")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("verified legacy detail extension not consumed")
	}
	candidate.Track.Extensions["recording_details"] = json.RawMessage(`{"id":"foreign","title":"Wrong","relations":[{"type":"performance","direction":"forward","work":{"id":"foreign-work"}}]}`)
	AttachGraph(&track, []canonical.TrackCandidate{candidate})
	if strings.Contains(string(track.Extensions["music_graph"]), "foreign-work") {
		t.Fatal("unverified detail graph leaked")
	}
}
