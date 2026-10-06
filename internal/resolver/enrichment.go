package resolver

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/canonical"
)

func (res *Resolver) enrichAccepted(ctx context.Context, accepted []canonical.TrackCandidate, statuses []canonical.SourceStatus) {
	for index := range accepted {
		candidate := &accepted[index]
		adapter, ok := res.registry.Get(candidate.Provider)
		if !ok {
			continue
		}
		enricher, ok := adapter.(adapters.MetadataEnricher)
		if !ok {
			continue
		}
		status := "unavailable"
		start := time.Now()
		enrichCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if err := res.WaitProvider(enrichCtx, candidate.Provider); err == nil {
			if rich, err := enricher.Enrich(enrichCtx, candidate.Track); err == nil && rich != nil {
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
		cancel()
		for index := range statuses {
			if strings.EqualFold(statuses[index].Name, candidate.Provider) {
				statuses[index].EnrichmentStatus = status
				statuses[index].LatencyMS += time.Since(start).Milliseconds()
			}
		}
	}
}

func sameEnrichmentIdentity(provider string, original, rich canonical.Track) bool {
	id := original.IDs[provider]
	if id == "" || rich.IDs[provider] != id {
		return false
	}
	if original.ISRC != "" && !strings.EqualFold(original.ISRC, rich.ISRC) {
		return false
	}
	// Even an endpoint returning the requested ID must agree with the accepted
	// identity before its credits or external context are attached.
	return strings.EqualFold(strings.TrimSpace(original.Title), strings.TrimSpace(rich.Title)) &&
		getPrimaryArtist(original.Artists) != "" && strings.EqualFold(strings.TrimSpace(getPrimaryArtist(original.Artists)), strings.TrimSpace(getPrimaryArtist(rich.Artists)))
}
