package deezer

import (
	"context"
	"encoding/json"

	"github.com/ommr/ommr/internal/models/canonical"
)

func (p *Provider) EnrichCandidate(ctx context.Context, candidate canonical.TrackCandidate) (*canonical.TrackCandidate, error) {
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(candidate.RawResponse, &raw)
	if len(raw["data"]) == 0 {
		return nil, nil
	}
	return p.FetchByID(ctx, ProviderName, candidate.Track.IDs[ProviderName])
}
