package merger

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/ommr/ommr/internal/models/canonical"
)

func artistNamesKey(artists []canonical.Artist) string {
	names := make([]string, 0, len(artists))
	for _, artist := range artists {
		names = append(names, strings.ToLower(strings.Join(strings.Fields(artist.Name), " ")))
	}
	sort.Strings(names)
	encoded, _ := json.Marshal(names)
	return string(encoded)
}

func artistVoteKey(artists []canonical.Artist, candidates []canonical.TrackCandidate) string {
	key := artistNamesKey(artists)
	if len(artists) != 1 {
		return key
	}
	display := strings.ToLower(strings.Join(strings.Fields(artists[0].Name), " "))
	keys := map[string]bool{}
	for _, candidate := range candidates {
		separate := candidate.Track.Artists
		if len(separate) < 2 {
			continue
		}
		names := make([]string, len(separate))
		for index, artist := range separate {
			names[index] = strings.ToLower(strings.Join(strings.Fields(artist.Name), " "))
		}
		if len(names) > 2 && (display == strings.Join(names[:len(names)-1], ", ")+" & "+names[len(names)-1] || display == strings.Join(names[:len(names)-1], ", ")+" and "+names[len(names)-1]) {
			keys[artistNamesKey(separate)] = true
		}
		for _, separator := range []string{" & ", ", ", " and "} {
			if display == strings.Join(names, separator) {
				keys[artistNamesKey(separate)] = true
			}
		}
	}
	// A display string cannot establish a unique person split if catalogs disagree.
	if len(keys) == 1 {
		for equivalent := range keys {
			return equivalent
		}
	}
	return key
}

func mergeArtistIDs(selected []canonical.Artist, candidates []canonical.TrackCandidate, key string) []canonical.Artist {
	result := make([]canonical.Artist, len(selected))
	for index, artist := range selected {
		result[index] = artist
		result[index].IDs = map[string]string{}
		for provider, id := range artist.IDs {
			result[index].IDs[provider] = id
		}
		for _, candidate := range candidates {
			if artistVoteKey(candidate.Track.Artists, candidates) != key {
				continue
			}
			for _, other := range candidate.Track.Artists {
				if !strings.EqualFold(strings.TrimSpace(artist.Name), strings.TrimSpace(other.Name)) {
					continue
				}
				id := other.IDs[candidate.Provider]
				if id != "" && result[index].IDs[candidate.Provider] == "" {
					result[index].IDs[candidate.Provider] = id
				}
			}
		}
	}
	return result
}
