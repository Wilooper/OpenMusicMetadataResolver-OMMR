package merger

import "github.com/ommr/ommr/internal/models/canonical"

func (m *Merger) MergeCandidatesAnchored(candidates []canonical.TrackCandidate, provider string) *canonical.Track {
	for _, c := range candidates {
		if provider == "" || c.Provider != provider || c.Track.Album.IDs[provider] == "" || c.Track.Album.Title == "" {
			continue
		}
		copy := *m
		copy.policy.Priorities = map[string][]string{}
		for key, value := range m.policy.Priorities {
			copy.policy.Priorities[key] = value
		}
		copy.policy.Priorities["album"] = append([]string{provider}, titlePriority...)
		return copy.MergeCandidates(candidates)
	}
	return m.MergeCandidates(candidates)
}
