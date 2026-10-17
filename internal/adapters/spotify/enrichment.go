package spotify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/models/provider"
)

func (p *Provider) EnrichCandidate(ctx context.Context, candidate canonical.TrackCandidate) (*canonical.TrackCandidate, error) {
	id := candidate.Track.Album.IDs[ProviderName]
	if id == "" {
		return nil, nil
	}
	token, err := p.tokens.Get(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBaseURL+"/albums/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Spotify album details status %d", resp.StatusCode)
	}
	data, err := adapters.ReadJSONBody(resp)
	if err != nil {
		return nil, err
	}
	var album provider.SpotifyWebAlbum
	if err := json.Unmarshal(data, &album); err != nil {
		return nil, err
	}
	if album.ID != id || !strings.EqualFold(strings.TrimSpace(album.Name), strings.TrimSpace(candidate.Track.Album.Title)) {
		return nil, fmt.Errorf("Spotify album detail identity mismatch")
	}
	// Clone before adding album facts: cached search candidates may be shared.
	encoded, _ := json.Marshal(candidate)
	var result canonical.TrackCandidate
	if err := json.Unmarshal(encoded, &result); err != nil {
		return nil, err
	}
	track := &result.Track
	if album.Label != "" {
		track.Label = album.Label
		track.Album.Label = album.Label
	}
	upc := album.ExternalIDs.UPC
	if upc == "" {
		upc = album.UPC
	}
	if upc != "" {
		track.Barcode = upc
		track.Album.UPC = upc
		track.Album.Barcode = upc
	}
	if album.TotalTracks > 0 {
		track.Album.TotalTracks = album.TotalTracks
	}
	if album.AlbumType != "" {
		track.Album.Type = album.AlbumType
	}
	if len(album.Copyrights) > 0 {
		track.Copyrights = nil
		for _, copyright := range album.Copyrights {
			track.Copyrights = append(track.Copyrights, canonical.Copyright{Type: copyright.Type, Text: copyright.Text})
		}
		track.Album.Copyrights = append([]canonical.Copyright(nil), track.Copyrights...)
	}
	if track.Extensions == nil {
		track.Extensions = map[string]json.RawMessage{}
	}
	track.Extensions["spotify_album"] = append(json.RawMessage(nil), data...)
	return &result, nil
}
