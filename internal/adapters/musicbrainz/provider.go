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
	"github.com/ommr/ommr/internal/enrichment/wikipedia"
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
		reqURL = fmt.Sprintf("%s/isrc/%s?inc=artists+releases+isrcs+tags&fmt=json", BaseURL, url.PathEscape(id))
	} else {
		reqURL = fmt.Sprintf("%s/recording/%s?inc=artists+releases+isrcs+tags+artist-rels+work-rels+work-level-rels+url-rels&fmt=json", BaseURL, url.PathEscape(id))
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
		if len(isrcResp.Recordings) > 1 {
			return nil, fmt.Errorf("ISRC maps to multiple MusicBrainz recordings; use a recording MBID or title and artist")
		}
		rec = isrcResp.Recordings[0]
		if !selectISRC(&rec, id) {
			return nil, fmt.Errorf("MusicBrainz recording does not report requested ISRC")
		}
	} else {
		if err := json.Unmarshal(body, &rec); err != nil {
			return nil, err
		}
		if rec.ID != id {
			return nil, fmt.Errorf("MusicBrainz recording ID does not match requested ID")
		}
	}

	track := p.normalizeRecording(rec)
	workIDs := make(map[string]bool)
	for _, relation := range rec.Relations {
		if relation.Work.ID != "" {
			workIDs[relation.Work.ID] = true
		}
	}
	if idType != "isrc" && len(workIDs) == 1 {
		for _, rel := range rec.Relations {
			if rel.Work.ID == "" {
				continue
			}
			for _, workRel := range rel.Work.Relations {
				var context *wikipedia.Context
				var err error
				switch workRel.Type {
				case "wikipedia":
					context, err = wikipedia.Fetch(ctx, workRel.URL.Resource)
				case "wikidata":
					context, err = wikipedia.FetchFromWikidata(ctx, workRel.URL.Resource)
				default:
					continue
				}
				if err == nil {
					encoded, _ := json.Marshal(context)
					if track.Extensions == nil {
						track.Extensions = make(map[string]json.RawMessage)
					}
					track.Extensions["wikipedia"] = encoded
					break
				}
			}
			if len(track.Extensions["wikipedia"]) > 0 {
				break
			}
		}
	}
	return &canonical.TrackCandidate{
		Provider:    ProviderName,
		Track:       track,
		MatchScore:  1.0,
		RawResponse: body,
	}, nil
}

func (p *Provider) Enrich(ctx context.Context, track canonical.Track) (*canonical.Track, error) {
	id := track.IDs[ProviderName]
	if id == "" {
		return nil, fmt.Errorf("MusicBrainz enrichment requires a recording ID")
	}
	candidate, err := p.FetchByID(ctx, ProviderName, id)
	if err != nil {
		return nil, err
	}
	if track.ISRC != "" {
		var recording provider.MusicBrainzRecording
		if err := json.Unmarshal(candidate.RawResponse, &recording); err != nil {
			return nil, err
		}
		if !selectISRC(&recording, track.ISRC) {
			return nil, fmt.Errorf("MusicBrainz enriched recording has conflicting ISRC")
		}
		candidate.Track.ISRC = recording.ISRCs[0]
	}
	return &candidate.Track, nil
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
		if q.ISRC != "" && !selectISRC(&rec, q.ISRC) {
			continue
		}
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

// Preserve the ISRC that established the match when a recording has multiple
// codes; provider array order is not identity evidence.
func selectISRC(recording *provider.MusicBrainzRecording, expected string) bool {
	for index, isrc := range recording.ISRCs {
		if strings.EqualFold(strings.TrimSpace(isrc), strings.TrimSpace(expected)) {
			copy(recording.ISRCs[1:index+1], recording.ISRCs[:index])
			recording.ISRCs[0] = isrc
			return true
		}
	}
	return false
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
	var iswc, language string
	var extensions map[string]json.RawMessage
	works := make(map[string]bool)
	for _, relation := range rec.Relations {
		if relation.Work.ID != "" {
			works[relation.Work.ID] = true
		}
	}
	for _, a := range artists {
		credits = append(credits, canonical.Credit{Name: a.Name, Roles: []string{"Artist"}})
	}
	for _, rel := range rec.Relations {
		if rel.Work.ID != "" {
			if len(works) == 1 && extensions == nil {
				workID, _ := json.Marshal(rel.Work.ID)
				extensions = map[string]json.RawMessage{"musicbrainz_work_id": workID}
			}
			if len(works) == 1 && iswc == "" && len(rel.Work.ISWCs) > 0 {
				iswc = rel.Work.ISWCs[0]
			}
			if len(works) == 1 && language == "" {
				language = rel.Work.Language
			}
			for _, workRel := range rel.Work.Relations {
				appendRelationCredit(&credits, workRel)
			}
		} else {
			appendRelationCredit(&credits, rel)
		}
	}

	return canonical.Track{
		Title:       rec.Title,
		Artists:     artists,
		Album:       album,
		DurationMS:  rec.Length,
		ReleaseDate: album.ReleaseDate,
		ISRC:        isrc,
		ISWC:        iswc,
		Language:    language,
		Genres:      genres,
		Credits:     credits,
		IDs:         map[string]string{"musicbrainz": rec.ID},
		Sources:     []string{ProviderName},
		Extensions:  extensions,
	}
}

func appendRelationCredit(credits *[]canonical.Credit, relation provider.MusicBrainzRelation) {
	role := strings.ToLower(relation.Type)
	switch role {
	case "lyricist", "composer", "writer", "producer", "engineer", "performer", "arranger":
	default:
		return
	}
	if relation.Artist.Name == "" {
		return
	}
	for n := range *credits {
		if (*credits)[n].Name == relation.Artist.Name {
			for _, existing := range (*credits)[n].Roles {
				if strings.EqualFold(existing, role) {
					return
				}
			}
			(*credits)[n].Roles = append((*credits)[n].Roles, strings.ToUpper(role[:1])+role[1:])
			return
		}
	}
	*credits = append(*credits, canonical.Credit{Name: relation.Artist.Name, Roles: []string{strings.ToUpper(role[:1]) + role[1:]}})
}

func escapeLucene(input string) string {
	special := []string{`\`, "+", "-", "!", "(", ")", "{", "}", "[", "]", "^", `"`, "~", "*", "?", ":", "/", "$", "&", "|"}
	res := input
	for _, char := range special {
		res = strings.ReplaceAll(res, char, `\`+char)
	}
	return res
}
