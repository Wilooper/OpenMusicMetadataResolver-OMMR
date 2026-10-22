package resolver

import (
	"sort"
	"strings"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/canonical"
)

func durationTolerance(duration int64) int64 {
	// Allow rounding, fades and modest catalog timing differences; reject only large contradictions.
	return max(int64(10000), duration/20)
}
func durationOutliers(candidates []canonical.TrackCandidate) map[string]bool {
	timed := []canonical.TrackCandidate{}
	seen := map[string]bool{}
	for _, c := range candidates {
		p := strings.ToLower(c.Provider)
		if c.Track.DurationMS > 0 && !seen[p] {
			timed = append(timed, c)
			seen[p] = true
		}
	}
	rejected := map[string]bool{}
	if len(timed) < 3 {
		return rejected
	}
	sort.Slice(timed, func(i, j int) bool { return timed[i].Track.DurationMS < timed[j].Track.DurationMS })
	bestStart, bestEnd := 0, 0
	for start := range timed {
		end := start
		for end < len(timed) && timed[end].Track.DurationMS-timed[start].Track.DurationMS <= durationTolerance(timed[start].Track.DurationMS) {
			end++
		}
		if end-start > bestEnd-bestStart {
			bestStart, bestEnd = start, end
		}
	}
	// One candidate or a tied split is not independent agreement on an intended recording.
	if bestEnd-bestStart < 2 || (bestEnd-bestStart)*2 <= len(timed) {
		return rejected
	}
	anchor := timed[(bestStart+bestEnd-1)/2].Track.DurationMS
	for _, c := range timed {
		delta := c.Track.DurationMS - anchor
		if delta < 0 {
			delta = -delta
		}
		if delta > durationTolerance(anchor) {
			rejected[strings.ToLower(c.Provider)] = true
		}
	}
	return rejected
}

func durationRejections(candidates []canonical.TrackCandidate, q adapters.Query) map[string]bool {
	if q.DirectInput().Provider == "" {
		return durationOutliers(candidates)
	}
	rejected := map[string]bool{}
	for _, seed := range candidates {
		if !candidateMatchesDirectID(q, seed) || seed.Track.DurationMS <= 0 {
			continue
		}
		for _, candidate := range candidates {
			if candidateMatchesDirectID(q, candidate) || candidate.Track.DurationMS <= 0 {
				continue
			}
			delta := candidate.Track.DurationMS - seed.Track.DurationMS
			if delta < 0 {
				delta = -delta
			}
			if delta > durationTolerance(seed.Track.DurationMS) {
				rejected[strings.ToLower(candidate.Provider)] = true
			}
		}
		break
	}
	return rejected
}
