package merger

import (
	"sort"
	"strings"

	"github.com/ommr/ommr/internal/identity"
	"github.com/ommr/ommr/internal/models/canonical"
)

// Priorities for metadata fields per provider
var (
	titlePriority   = []string{"spotify", "applemusic", "musicbrainz", "deezer", "soundcloud", "jiosaavn", "ytmusic"}
	creditsPriority = []string{"musicbrainz", "applemusic", "spotify", "deezer", "soundcloud", "jiosaavn", "ytmusic"}
	genresPriority  = []string{"spotify", "applemusic", "musicbrainz", "deezer", "soundcloud", "jiosaavn", "ytmusic"}
	isrcPriority    = []string{"spotify", "musicbrainz", "applemusic", "deezer", "soundcloud", "jiosaavn", "ytmusic"}
	labelPriority   = []string{"spotify", "applemusic", "deezer", "musicbrainz", "jiosaavn"}
	previewPriority = []string{"spotify", "applemusic", "deezer"}
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

	// 9b. Preview URL
	for _, prov := range previewPriority {
		if cand, ok := byProvider[prov]; ok && cand.Track.PreviewURL != "" {
			res.PreviewURL = cand.Track.PreviewURL
			res.FieldSources["preview_url"] = []string{prov}
			break
		}
	}

	// 9c. Track / Disc Number
	for _, prov := range titlePriority {
		if cand, ok := byProvider[prov]; ok && cand.Track.TrackNumber > 0 {
			res.TrackNumber = cand.Track.TrackNumber
			res.FieldSources["track_number"] = []string{prov}
			break
		}
	}
	for _, cand := range candidates {
		if cand.Track.DiscNumber > 0 {
			res.DiscNumber = cand.Track.DiscNumber
			res.FieldSources["disc_number"] = []string{cand.Provider}
			break
		}
	}

	// 9d. Label, Barcode, ISWC, PlayCount
	for _, prov := range labelPriority {
		if cand, ok := byProvider[prov]; ok && cand.Track.Label != "" {
			res.Label = cand.Track.Label
			res.FieldSources["label"] = []string{prov}
			break
		}
	}
	if res.Label == "" {
		for _, cand := range candidates {
			if cand.Track.Label != "" {
				res.Label = cand.Track.Label
				res.FieldSources["label"] = []string{cand.Provider}
				break
			}
		}
	}
	for _, cand := range candidates {
		if cand.Track.Barcode != "" && res.Barcode == "" {
			res.Barcode = cand.Track.Barcode
		}
		if cand.Track.ISWC != "" && res.ISWC == "" {
			res.ISWC = cand.Track.ISWC
		}
		if cand.Track.PlayCount > res.PlayCount {
			res.PlayCount = cand.Track.PlayCount
		}
	}

	// 9e. Copyrights (deduplicated union)
	copyrightMap := make(map[string]canonical.Copyright)
	for _, cand := range candidates {
		for _, c := range cand.Track.Copyrights {
			if c.Text != "" {
				copyrightMap[c.Type+":"+c.Text] = c
			}
		}
	}
	res.Copyrights = make([]canonical.Copyright, 0, len(copyrightMap))
	for _, c := range copyrightMap {
		res.Copyrights = append(res.Copyrights, c)
	}

	// 9f. Album enrichment (label, UPC/barcode, total tracks, copyrights)
	if res.Album.Label == "" {
		for _, cand := range candidates {
			if cand.Track.Album.Label != "" {
				res.Album.Label = cand.Track.Album.Label
				break
			}
		}
	}
	if res.Album.UPC == "" {
		for _, cand := range candidates {
			if cand.Track.Album.UPC != "" {
				res.Album.UPC = cand.Track.Album.UPC
				break
			}
		}
	}
	if res.Album.Barcode == "" {
		for _, cand := range candidates {
			if cand.Track.Album.Barcode != "" {
				res.Album.Barcode = cand.Track.Album.Barcode
				break
			}
		}
	}
	if res.Album.TotalTracks == 0 {
		for _, cand := range candidates {
			if cand.Track.Album.TotalTracks > 0 {
				res.Album.TotalTracks = cand.Track.Album.TotalTracks
				break
			}
		}
	}
	if len(res.Album.Copyrights) == 0 {
		albumCopyrights := make(map[string]canonical.Copyright)
		for _, cand := range candidates {
			for _, c := range cand.Track.Album.Copyrights {
				if c.Text != "" {
					albumCopyrights[c.Type+":"+c.Text] = c
				}
			}
		}
		res.Album.Copyrights = make([]canonical.Copyright, 0, len(albumCopyrights))
		for _, c := range albumCopyrights {
			res.Album.Copyrights = append(res.Album.Copyrights, c)
		}
	}

	// 10. Merge IDs & Sources
	topScore := 0.0
	topBreakdown := canonical.MatchBreakdown{}
	for _, cand := range candidates {
		if !contains(res.Sources, cand.Provider) {
			res.Sources = append(res.Sources, cand.Provider)
		}
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
