package musicbrainz

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/models/provider"
	"github.com/ommr/ommr/pkg/query"
)

const (
	ProviderName    = "musicbrainz"
	ProviderVersion = "musicbrainz-v2"
	BaseURL         = "https://musicbrainz.org/ws/2"
	UserAgent       = "OMMR/1.0.0 (https://github.com/ommr/ommr)"
)

type Provider struct {
	httpClient *http.Client
}

var _ adapters.ProviderAdapter = (*Provider)(nil)

func New() *Provider {
	return &Provider{
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func (p *Provider) Name() string {
	return ProviderName
}

func (p *Provider) Version() string {
	return ProviderVersion
}

func (p *Provider) FetchByID(ctx context.Context, idType string, id string) (*canonical.TrackCandidate, error) {
	if idType != "mbid" && idType != "musicbrainz" && idType != "isrc" {
		return nil, fmt.Errorf("unsupported ID type %q for MusicBrainz", idType)
	}

	var reqURL string
	if idType == "isrc" {
		reqURL = fmt.Sprintf("%s/isrc/%s?fmt=json", BaseURL, url.PathEscape(id))
	} else {
		reqURL = fmt.Sprintf("%s/recording/%s?inc=artists+releases+isrcs+tags&fmt=json", BaseURL, url.PathEscape(id))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("musicbrainz API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rec provider.MusicBrainzRecording
	if idType == "isrc" {
		var isrcResp struct {
			Recordings []provider.MusicBrainzRecording `json:"recordings"`
		}
		if err := json.Unmarshal(body, &isrcResp); err != nil || len(isrcResp.Recordings) == 0 {
			return nil, fmt.Errorf("no MusicBrainz recording found for ISRC %s", id)
		}
		rec = isrcResp.Recordings[0]
	} else {
		if err := json.Unmarshal(body, &rec); err != nil {
			return nil, err
		}
	}

	track := p.normalizeRecording(rec)
	return &canonical.TrackCandidate{
		Provider:    ProviderName,
		Track:       track,
		MatchScore:  1.0,
		RawResponse: body,
	}, nil
}

func (p *Provider) Search(ctx context.Context, q adapters.Query) ([]canonical.TrackCandidate, error) {
	var luceneQuery string
	if q.ISRC != "" {
		luceneQuery = fmt.Sprintf("isrc:%s", escapeLucene(q.ISRC))
	} else if q.Artist != "" && q.Title != "" {
		luceneQuery = fmt.Sprintf("artist:\"%s\" AND recording:\"%s\"", escapeLucene(query.CleanArtist(q.Artist)), escapeLucene(query.CleanTitle(q.Title)))
	} else if q.Title != "" {
		luceneQuery = fmt.Sprintf("recording:\"%s\"", escapeLucene(query.CleanTitle(q.Title)))
	} else {
		return nil, nil
	}

	reqURL := fmt.Sprintf("%s/recording/?query=%s&fmt=json&limit=5", BaseURL, url.QueryEscape(luceneQuery))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("musicbrainz search returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var searchResp provider.MusicBrainzSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, err
	}

	candidates := make([]canonical.TrackCandidate, 0, len(searchResp.Recordings))
	for _, rec := range searchResp.Recordings {
		cand := canonical.TrackCandidate{
			Provider:    ProviderName,
			Track:       p.normalizeRecording(rec),
			MatchScore:  float64(rec.Score) / 100.0,
			RawResponse: body,
		}
		candidates = append(candidates, cand)
	}

	return candidates, nil
}

func (p *Provider) normalizeRecording(rec provider.MusicBrainzRecording) canonical.Track {
	artists := make([]canonical.Artist, 0, len(rec.ArtistCredit))
	for _, ac := range rec.ArtistCredit {
		name := ac.Name
		if name == "" {
			name = ac.Artist.Name
		}
		artists = append(artists, canonical.Artist{
			Name: name,
			Role: "main",
			IDs:  map[string]string{"musicbrainz": ac.Artist.ID},
		})
	}

	var album canonical.Album
	if len(rec.Releases) > 0 {
		rel := rec.Releases[0]
		album = canonical.Album{
			Title:       rel.Title,
			ReleaseDate: rel.Date,
			IDs:         map[string]string{"musicbrainz": rel.ID},
		}
	}

	var isrc string
	if len(rec.ISRCs) > 0 {
		isrc = rec.ISRCs[0]
	}

	genres := make([]string, 0, len(rec.Tags))
	for _, tag := range rec.Tags {
		if tag.Count > 0 {
			genres = append(genres, tag.Name)
		}
	}

	credits := make([]canonical.Credit, 0, len(artists))
	for _, a := range artists {
		credits = append(credits, canonical.Credit{Name: a.Name, Roles: []string{"Artist"}})
	}

	return canonical.Track{
		Title:       rec.Title,
		Artists:     artists,
		Album:       album,
		DurationMS:  rec.Length,
		ReleaseDate: album.ReleaseDate,
		ISRC:        isrc,
		Genres:      genres,
		Credits:     credits,
		IDs:         map[string]string{"musicbrainz": rec.ID},
		Sources:     []string{ProviderName},
	}
}

func escapeLucene(input string) string {
	special := []string{`\`, "+", "-", "!", "(", ")", "{", "}", "[", "]", "^", `"`, "~", "*", "?", ":", "/", "$", "&", "|"}
	res := input
	for _, char := range special {
		res = strings.ReplaceAll(res, char, `\`+char)
	}
	return res
}
