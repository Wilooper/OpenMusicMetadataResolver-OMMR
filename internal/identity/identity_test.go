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

func TestLinkHelpersDoNotChangeSparseCanonicalID(t *testing.T) {
	e := New()
	base := canonical.Track{IDs: map[string]string{"soundcloud": "m83/midnight-city"}}
	withLinks := canonical.Track{IDs: map[string]string{"soundcloud": "m83/midnight-city", "soundcloud_track_id": "123456", "soundcloud_url": "https://soundcloud.com/m83/midnight-city"}}
	if e.GenerateID(base) != e.GenerateID(withLinks) {
		t.Fatal("adding native ID/share URL changed OMMR identity")
	}
	if e.GenerateID(canonical.Track{IDs: map[string]string{"soundcloud_url": "https://soundcloud.com/m83/midnight-city"}}) != e.GenerateID(canonical.Track{}) {
		t.Fatal("helper-only map asserted recording identity")
	}
}
