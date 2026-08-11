package identity

import (
	"testing"

	"github.com/ommr/ommr/internal/models/canonical"
)

func TestGenerateIDHierarchy(t *testing.T) {
	e := New()

	// 1. Path-independent metadata matching
	trackMeta1 := canonical.Track{
		Title:       "Tu",
		Artists:     []canonical.Artist{{Name: "Talwiinder"}},
		ReleaseDate: "2024-06-21",
	}

	trackMeta2 := canonical.Track{
		Title:       "Tu (Official Video)",
		Artists:     []canonical.Artist{{Name: "Talwiinder - Topic"}},
		ReleaseDate: "2024-06-21",
	}

	idMeta1 := e.GenerateID(trackMeta1)
	idMeta2 := e.GenerateID(trackMeta2)

	if idMeta1 != idMeta2 {
		t.Errorf("expected identical canonical_id for trackMeta1 (%s) and trackMeta2 (%s)", idMeta1, idMeta2)
	}

	// 2. Different track produces different canonical_id
	trackWishes := canonical.Track{
		Title:   "Wishes",
		Artists: []canonical.Artist{{Name: "Talwiinder"}},
	}
	idWishes := e.GenerateID(trackWishes)

	if idMeta1 == idWishes {
		t.Errorf("expected different canonical_id for Tu vs Wishes")
	}
}
