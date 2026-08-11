package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/pkg/query"
)

// IdentityEngine computes deterministic, collision-resistant canonical IDs for resolved tracks.
type IdentityEngine struct{}

func New() *IdentityEngine {
	return &IdentityEngine{}
}

// GenerateID produces a stable, recording-based canonical_id formatted as "ommr_track_<16-char-hex>".
// Hierarchy:
// 1. Stable Recording Metadata (Title + Primary Artist): hash("meta:" + cleanTitle + "|" + cleanPrimaryArtist)
// 2. ISRC fallback if available.
// 3. MusicBrainz ID / Provider ID fallback if title/artist fold empty.
func (e *IdentityEngine) GenerateID(t canonical.Track) string {
	var rawKey string

	cleanTitle := query.CleanTitle(t.Title)
	if cleanTitle == "" && strings.TrimSpace(t.Title) != "" {
		cleanTitle = strings.ToLower(strings.TrimSpace(t.Title))
	}

	cleanPrimaryArtist := ""
	if len(t.Artists) > 0 {
		cleanPrimaryArtist = query.CleanArtist(t.Artists[0].Name)
		if cleanPrimaryArtist == "" && strings.TrimSpace(t.Artists[0].Name) != "" {
			cleanPrimaryArtist = strings.ToLower(strings.TrimSpace(t.Artists[0].Name))
		}
	}

	cleanISRC := strings.TrimSpace(strings.ToUpper(t.ISRC))
	mbID := strings.TrimSpace(t.IDs["musicbrainz"])

	if cleanTitle != "" && cleanPrimaryArtist != "" {
		rawKey = fmt.Sprintf("meta:%s|%s", cleanTitle, cleanPrimaryArtist)
	} else if cleanISRC != "" {
		rawKey = "isrc:" + cleanISRC
	} else if mbID != "" {
		rawKey = "mbid:" + mbID
	} else if len(t.IDs) > 0 {
		idPairs := make([]string, 0, len(t.IDs))
		for k, v := range t.IDs {
			if v != "" {
				idPairs = append(idPairs, k+":"+v)
			}
		}
		sort.Strings(idPairs)
		rawKey = "ids:" + strings.Join(idPairs, "|")
	} else {
		rawKey = fmt.Sprintf("raw:%s", t.Title)
	}

	h := sha256.Sum256([]byte(rawKey))
	hashHex := hex.EncodeToString(h[:])
	if len(hashHex) > 16 {
		hashHex = hashHex[:16]
	}

	return "ommr_track_" + hashHex
}
