package resolver

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/metadata"
	"github.com/ommr/ommr/internal/models/canonical"
)

func (res *Resolver) enrichAccepted(ctx context.Context, accepted []canonical.TrackCandidate, statuses []canonical.SourceStatus) {
	var wg sync.WaitGroup
	for index := range accepted {
		candidate := &accepted[index]
		adapter, ok := res.registry.Get(candidate.Provider)
		if !ok {
			continue
		}
		details, hasDetails := adapter.(adapters.CandidateEnricher)
		legacy, hasLegacy := adapter.(adapters.MetadataEnricher)
		if !hasDetails && !hasLegacy {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			status := "unavailable"
			start := time.Now()
			enrichCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			if err := res.WaitProvider(enrichCtx, candidate.Provider); err == nil {
				if hasDetails {
					rich, err := details.EnrichCandidate(enrichCtx, *candidate)
					if err == nil && rich == nil {
						status = "not_needed"
					}
					if err == nil && rich != nil {
						if rich.Provider == candidate.Provider && sameEnrichmentIdentity(candidate.Provider, candidate.Track, rich.Track) {
							hydrated := metadata.Hydrate(*candidate, *rich)
							rich = &hydrated
							rich.MatchScore = candidate.MatchScore
							rich.Track.MatchScore = candidate.Track.MatchScore
							rich.Track.MatchBreakdown = candidate.Track.MatchBreakdown
							rich.Track.Confidence = candidate.Track.Confidence
							// Detail hydration may add metadata, never another provider's recording ID.
							ids := make(map[string]string)
							for key, id := range candidate.Track.IDs {
								ids[key] = id
							}
							for key, id := range rich.Track.IDs {
								if key == candidate.Provider || key == candidate.Provider+"_url" || key == candidate.Provider+"_track_id" {
									ids[key] = id
								}
							}
							rich.Track.IDs = ids
							*candidate = *rich
							status = "ok"
						} else {
							status = "identity_mismatch"
						}
					}
				} else if rich, err := legacy.Enrich(enrichCtx, candidate.Track); err == nil && rich != nil {
					if sameEnrichmentIdentity(candidate.Provider, candidate.Track, *rich) {
						if len(rich.Credits) > 0 {
							candidate.Track.Credits = rich.Credits
						}
						if rich.ISWC != "" {
							candidate.Track.ISWC = rich.ISWC
						}
						if rich.Language != "" {
							candidate.Track.Language = rich.Language
						}
						if candidate.Track.Extensions == nil {
							candidate.Track.Extensions = make(map[string]json.RawMessage)
						}
						for key, value := range rich.Extensions {
							candidate.Track.Extensions[key] = append(json.RawMessage(nil), value...)
						}
						status = "ok"
					} else {
						status = "identity_mismatch"
					}
				}
			}
			for index := range statuses {
				if strings.EqualFold(statuses[index].Name, candidate.Provider) {
					statuses[index].EnrichmentStatus = status
					statuses[index].LatencyMS += time.Since(start).Milliseconds()
				}
			}
		}()
	}
	wg.Wait()
}

func sameEnrichmentIdentity(provider string, original, rich canonical.Track) bool {
	id := original.IDs[provider]
	if id == "" || rich.IDs[provider] != id {
		return false
	}
	if original.ISRC != "" && !strings.EqualFold(original.ISRC, rich.ISRC) {
		return false
	}
	if original.DurationMS > 0 && rich.DurationMS > 0 {
		delta := original.DurationMS - rich.DurationMS
		if delta < -2000 || delta > 2000 {
			return false
		}
	}
	if albumID := original.Album.IDs[provider]; albumID != "" && rich.Album.IDs[provider] != "" && rich.Album.IDs[provider] != albumID {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(original.Title), strings.TrimSpace(rich.Title)) &&
		getPrimaryArtist(original.Artists) != "" && strings.EqualFold(strings.TrimSpace(getPrimaryArtist(original.Artists)), strings.TrimSpace(getPrimaryArtist(rich.Artists)))
}
