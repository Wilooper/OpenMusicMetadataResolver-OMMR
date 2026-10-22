package musicgraph

import (
	"reflect"
	"testing"
)

func TestDetailedCreditIndexPreservesIdentityScopeAndAttributes(t *testing.T) {
	b := New("musicbrainz:recording:rec")
	b.MusicBrainz("recording", record(t, `{"id":"rec","relations":[{"type":"instrument","direction":"backward","target-credit":"A. Player","attributes":["violin","additional"],"attribute-credits":{"violin":"baroque violin"},"begin":"2001","end":"2002","ended":false,"artist":{"id":"a","name":"Alex"}},{"type":"producer","direction":"backward","artist":{"id":"a","name":"Alex"}},{"type":"performance","direction":"forward","work":{"id":"w","relations":[{"type":"composer","artist":{"id":"b","name":"Alex"}}]}}]}`))
	g := b.Result()
	d := DetailedCredits(g)
	if d == nil || len(d.Contributors) != 2 || len(d.Credits) != 3 {
		t.Fatalf("lost contributors/roles: %+v", d)
	}
	for _, edge := range d.Credits {
		if edge.Role == "instrumentalist" && (edge.CreditedAs != "A. Player" || edge.Begin != "2001" || edge.End != "2002" || edge.Ended == nil || *edge.Ended || len(edge.Attributes) != 2 || edge.AttributeCredits["violin"] != "baroque violin") {
			t.Fatalf("lost performance detail: %+v", edge)
		}
		if edge.Role == "composer" && edge.To != "musicbrainz:work:w" {
			t.Fatal("work credit moved to recording")
		}
		if find(g, "credit", edge.From, edge.To) == nil {
			t.Fatal("credit invented")
		}
	}
	if !reflect.DeepEqual(d, DetailedCredits(g)) {
		t.Fatal("unstable credit index")
	}
	g.Truncated = true
	if !DetailedCredits(g).Truncated {
		t.Fatal("truncation hidden")
	}
}
