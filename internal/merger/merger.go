package merger

import (
	"sort"
	"strings"

	"github.com/ommr/ommr/internal/identity"
	"github.com/ommr/ommr/internal/models/canonical"
)

// Priorities for metadata fields per provider
var (
	titlePriority   = []string{"spotify", "applemusic", "musicbrainz", "deezer", "ytmusic"}
	creditsPriority = []string{"musicbrainz", "applemusic", "spotify", "deezer", "ytmusic"}
	genresPriority  = []string{"spotify", "applemusic", "musicbrainz", "deezer", "ytmusic"}
	isrcPriority    = []string{"spotify", "musicbrainz", "applemusic", "deezer", "ytmusic"}
)

type Merger struct {
	identityEngine *identity.IdentityEngine
}

func New() *Merger {
	return &Merger{
		identityEngine: identity.New(),
	}
}

// MergeCandidates merges a slice of scored TrackCandidates into a single unified canonical Track.
func (m *Merger) MergeCandidates(candidates []canonical.TrackCandidate) *canonical.Track {
	if len(candidates) == 0 {
		return nil
	}

	byProvider := make(map[string]canonical.TrackCandidate)
	for _, c := range candidates {
		byProvider[strings.ToLower(c.Provider)] = c
	}

	res := &canonical.Track{
		IDs:          make(map[string]string),
		Sources:      make([]string, 0, len(candidates)),
		FieldSources: make(map[string][]string),
		Relations:    canonical.Relations{},
	}

	// 1. Resolve Title
	for _, prov := range titlePriority {
		if cand, ok := byProvider[prov]; ok && cand.Track.Title != "" {
			res.Title = cand.Track.Title
			res.FieldSources["title"] = []string{prov}
			break
		}
	}
	if res.Title == "" {
		res.Title = candidates[0].Track.Title
		res.FieldSources["title"] = []string{candidates[0].Provider}
	}

	// 2. Resolve Artists
	for _, prov := range titlePriority {
		if cand, ok := byProvider[prov]; ok && len(cand.Track.Artists) > 0 {
			res.Artists = cand.Track.Artists
			res.FieldSources["artists"] = []string{prov}
			break
		}
	}
	if len(res.Artists) == 0 {
		res.Artists = candidates[0].Track.Artists
		res.FieldSources["artists"] = []string{candidates[0].Provider}
	}

	// 3. Resolve Album
	for _, prov := range titlePriority {
		if cand, ok := byProvider[prov]; ok && cand.Track.Album.Title != "" {
			res.Album = cand.Track.Album
			res.FieldSources["album"] = []string{prov}
			break
		}
	}

	// 4. Resolve DurationMS
	for _, cand := range candidates {
		if cand.Track.DurationMS > 0 {
			res.DurationMS = cand.Track.DurationMS
			res.FieldSources["duration"] = []string{cand.Provider}
			break
		}
	}

	// 5. Resolve ReleaseDate
	for _, cand := range candidates {
		if cand.Track.ReleaseDate != "" {
			res.ReleaseDate = cand.Track.ReleaseDate
			res.FieldSources["release_date"] = []string{cand.Provider}
			break
		}
	}

	// 6. Resolve ISRC & count ISRC matches across all candidates
	isrcSources := make([]string, 0)
	for _, cand := range candidates {
		if cand.Track.ISRC != "" {
			if res.ISRC == "" {
				res.ISRC = cand.Track.ISRC
			}
			if strings.EqualFold(res.ISRC, cand.Track.ISRC) && !contains(isrcSources, cand.Provider) {
				isrcSources = append(isrcSources, cand.Provider)
			}
		}
	}
	if len(isrcSources) > 0 {
		res.FieldSources["isrc"] = isrcSources
	}

	// 7. Resolve Credits (Multi-Role aggregation by contributor name)
	creditMap := make(map[string][]string) // Contributor Name -> slice of Roles
	creditSources := make([]string, 0)

	for _, prov := range creditsPriority {
		if cand, ok := byProvider[prov]; ok && len(cand.Track.Credits) > 0 {
			creditSources = append(creditSources, prov)
			for _, cr := range cand.Track.Credits {
				if cr.Name == "" {
					continue
				}
				existingRoles := creditMap[cr.Name]
				for _, r := range cr.Roles {
					if !contains(existingRoles, r) {
						existingRoles = append(existingRoles, r)
					}
				}
				creditMap[cr.Name] = existingRoles
			}
		}
	}

	res.Credits = make([]canonical.Credit, 0, len(creditMap))
	for name, roles := range creditMap {
		res.Credits = append(res.Credits, canonical.Credit{
			Name:  name,
			Roles: roles,
		})
	}
	if len(creditSources) > 0 {
		res.FieldSources["credits"] = creditSources
	}

	// 8. Resolve Genres
	genreSet := make(map[string]bool)
	genreSources := make([]string, 0)
	for _, prov := range genresPriority {
		if cand, ok := byProvider[prov]; ok && len(cand.Track.Genres) > 0 {
			genreSources = append(genreSources, prov)
			for _, g := range cand.Track.Genres {
				if g != "" {
					genreSet[g] = true
				}
			}
		}
	}
	res.Genres = make([]string, 0, len(genreSet))
	for g := range genreSet {
		res.Genres = append(res.Genres, g)
	}
	if len(genreSources) > 0 {
		res.FieldSources["genres"] = genreSources
	}

	// 9. Merge Images (Deduplicated union, sorted from lowest resolution to highest resolution)
	imageMap := make(map[string]canonical.Image)
	imageSources := make([]string, 0)
	for _, cand := range candidates {
		if len(cand.Track.Images) > 0 {
			imageSources = append(imageSources, cand.Provider)
			for _, img := range cand.Track.Images {
				if img.URL != "" {
					imageMap[img.URL] = img
				}
			}
		}
	}
	images := make([]canonical.Image, 0, len(imageMap))
	for _, img := range imageMap {
		images = append(images, img)
	}
	// Sort ascending: lowest resolution (width*height) to highest resolution
	sort.Slice(images, func(i, j int) bool {
		return (images[i].Width * images[i].Height) < (images[j].Width * images[j].Height)
	})
	res.Images = images
	if len(imageSources) > 0 {
		res.FieldSources["images"] = imageSources
	}

	// 10. Merge IDs & Sources
	topScore := 0.0
	topBreakdown := canonical.MatchBreakdown{}
	for _, cand := range candidates {
		res.Sources = append(res.Sources, cand.Provider)
		for k, v := range cand.Track.IDs {
			res.IDs[k] = v
		}
		if cand.MatchScore > topScore {
			topScore = cand.MatchScore
			topBreakdown = cand.Track.MatchBreakdown
		}
	}

	res.Explicit = candidates[0].Track.Explicit
	res.MatchScore = topScore
	res.MatchBreakdown = topBreakdown

	// Verified confidence rule: 2+ provider ISRC match
	confLevel := canonical.GetConfidenceLevel(topScore, len(isrcSources))
	res.Confidence = canonical.Confidence{
		Level: confLevel,
		Score: topScore,
	}

	// Canonical ID & Completeness calculation
	res.CanonicalID = m.identityEngine.GenerateID(*res)
	res.Completeness = canonical.CalculateCompleteness(*res)

	return res
}

func contains(slice []string, val string) bool {
	for _, item := range slice {
		if strings.EqualFold(item, val) {
			return true
		}
	}
	return false
}
