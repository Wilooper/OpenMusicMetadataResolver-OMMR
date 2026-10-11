package platformlinks

import (
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/ommr/ommr/internal/models/canonical"
)

type Link struct {
	ID          string  `json:"id"`
	IDType      string  `json:"id_type"`
	LookupID    string  `json:"lookup_id,omitempty"`
	URL         string  `json:"url,omitempty"`
	URLSource   string  `json:"url_source,omitempty"`
	Confidence  float64 `json:"confidence,omitempty"`
	MatchMethod string  `json:"match_method,omitempty"`
}

type Catalog struct {
	OMMRID               string          `json:"ommr_id,omitempty"`
	ISRC                 string          `json:"isrc,omitempty"`
	Platforms            map[string]Link `json:"platforms"`
	UnavailablePlatforms []string        `json:"unavailable_platforms"`
}

var (
	digits    = regexp.MustCompile(`^[0-9]+$`)
	spotifyID = regexp.MustCompile(`^[A-Za-z0-9]{22}$`)
	videoID   = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
	asin      = regexp.MustCompile(`^[A-Z0-9]{10}$`)
	mbid      = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)
	pandoraID = regexp.MustCompile(`^(TR:[0-9]+|TR[A-Za-z0-9]{13})$`)
	permalink = regexp.MustCompile(`^[A-Za-z0-9_-]+/[A-Za-z0-9_-]+$`)
	token     = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

var providers = []string{"amazonmusic", "applemusic", "deezer", "jiosaavn", "musicbrainz", "pandora", "qobuz", "soundcloud", "spotify", "tidal", "ytmusic"}

// Build receives an accepted resolved track, or one provider's extracted track.
// It never searches by title or infers an ID from an unrelated provider's URL.
func Build(track canonical.Track) Catalog {
	result := Catalog{OMMRID: track.CanonicalID, ISRC: track.ISRC, Platforms: map[string]Link{}, UnavailablePlatforms: []string{}}
	for _, provider := range providers {
		id := strings.TrimSpace(track.IDs[provider])
		link, ok := makeLink(provider, id)
		if !ok {
			result.UnavailablePlatforms = append(result.UnavailablePlatforms, provider)
			continue
		}
		if raw := providerURL(provider, id, track.IDs[provider+"_url"]); raw != "" {
			link.URL = raw
			link.URLSource = "provider"
		}
		if provider == "soundcloud" || provider == "jiosaavn" {
			native := track.IDs[provider+"_track_id"]
			if native != "" && token.MatchString(native) && (provider != "soundcloud" || digits.MatchString(native)) {
				link.LookupID, link.ID = id, native
				link.IDType = "track_id"
			}
		}
		for _, match := range track.IdentityMatches {
			if match.Provider == provider && match.ID == id {
				link.Confidence, link.MatchMethod = match.Confidence, match.Method
				break
			}
		}
		result.Platforms[provider] = link
		if provider == "ytmusic" {
			youtube := link
			youtube.URL, youtube.URLSource = "https://www.youtube.com/watch?v="+id, "id_template"
			result.Platforms["youtube"] = youtube
		}
	}
	if _, ok := result.Platforms["youtube"]; !ok {
		result.UnavailablePlatforms = append(result.UnavailablePlatforms, "youtube")
	}
	sort.Strings(result.UnavailablePlatforms)
	return result
}

func Attach(track *canonical.Track) {
	if track == nil {
		return
	}
	if track.Extensions == nil {
		track.Extensions = map[string]json.RawMessage{}
	}
	track.Extensions["platform_links"], _ = json.Marshal(Build(*track))
}

func makeLink(provider, id string) (Link, bool) {
	link := Link{ID: id, IDType: "track_id"}
	if id == "" {
		return link, false
	}
	switch provider {
	case "spotify":
		if !spotifyID.MatchString(id) {
			return link, false
		}
		link.URL = "https://open.spotify.com/track/" + id
	case "ytmusic":
		if !videoID.MatchString(id) {
			return link, false
		}
		link.IDType = "video_id"
		link.URL = "https://music.youtube.com/watch?v=" + id
	case "applemusic":
		if !digits.MatchString(id) {
			return link, false
		}
		link.URL = "https://music.apple.com/us/song/" + id
	case "deezer":
		if !digits.MatchString(id) {
			return link, false
		}
		link.URL = "https://www.deezer.com/track/" + id
	case "tidal":
		if !digits.MatchString(id) {
			return link, false
		}
		link.URL = "https://tidal.com/track/" + id
	case "qobuz":
		if !digits.MatchString(id) {
			return link, false
		}
		link.URL = "https://open.qobuz.com/track/" + id
	case "amazonmusic":
		if !token.MatchString(id) {
			return link, false
		}
		if asin.MatchString(id) {
			link.IDType = "asin"
			link.URL = "https://music.amazon.com/?trackAsin=" + id
		}
	case "pandora":
		if !pandoraID.MatchString(id) && !digits.MatchString(id) {
			return link, false
		}
		if !digits.MatchString(id) {
			link.IDType = "music_token"
		}
		// Pandora IDs use multiple namespaces; only return a provider URL.
	case "soundcloud":
		if !permalink.MatchString(id) {
			return link, false
		}
		link.IDType = "permalink"
		link.URL = "https://soundcloud.com/" + id
	case "jiosaavn":
		if !token.MatchString(id) {
			return link, false
		}
		link.IDType = "song_token"
		// A share URL contains a title slug as well as a token. Do not invent it.
	case "musicbrainz":
		if !mbid.MatchString(id) {
			return link, false
		}
		link.IDType = "recording_mbid"
		link.URL = "https://musicbrainz.org/recording/" + id
	default:
		return link, false
	}
	if link.URL != "" {
		link.URLSource = "id_template"
	}
	return link, true
}

func validURL(provider, id, raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" {
		return false
	}
	path := strings.Trim(u.Path, "/")
	parts := strings.Split(path, "/")
	last := parts[len(parts)-1]
	switch provider {
	case "spotify":
		return u.Host == "open.spotify.com" && path == "track/"+id && u.RawQuery == ""
	case "ytmusic":
		return u.Host == "music.youtube.com" && path == "watch" && u.Query().Get("v") == id && onlyQuery(u, "v")
	case "applemusic":
		if u.Host != "music.apple.com" || !onlyQuery(u, "i") {
			return false
		}
		if strings.Contains("/"+path+"/", "/album/") {
			return u.Query().Get("i") == id
		}
		return strings.Contains("/"+path+"/", "/song/") && last == id && (u.Query().Get("i") == "" || u.Query().Get("i") == id)
	case "deezer":
		return (u.Host == "www.deezer.com" || u.Host == "deezer.com") && len(parts) >= 2 && parts[len(parts)-2] == "track" && last == id && u.RawQuery == ""
	case "qobuz":
		return (u.Host == "open.qobuz.com" || u.Host == "play.qobuz.com") && path == "track/"+id && u.RawQuery == ""
	case "tidal":
		return (u.Host == "tidal.com" || u.Host == "listen.tidal.com") && (path == "track/"+id || path == "browse/track/"+id) && u.RawQuery == ""
	case "amazonmusic":
		return u.Host == "music.amazon.com" && ((path == "tracks/"+id && u.RawQuery == "") || ((path == "" || strings.HasPrefix(path, "albums/")) && u.Query().Get("trackAsin") == id && onlyQuery(u, "trackAsin")))
	case "pandora":
		return u.Host == "www.pandora.com" && last == id && u.RawQuery == ""
	case "soundcloud":
		return u.Host == "soundcloud.com" && path == id && u.RawQuery == ""
	case "jiosaavn":
		return (u.Host == "www.jiosaavn.com" || u.Host == "jiosaavn.com") && len(parts) == 3 && parts[0] == "song" && last == id && u.RawQuery == ""
	case "musicbrainz":
		return u.Host == "musicbrainz.org" && path == "recording/"+id && u.RawQuery == ""
	}
	return false
}

func onlyQuery(u *url.URL, allowed string) bool {
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return false
	}
	for key, values := range q {
		if key != allowed || len(values) != 1 {
			return false
		}
	}
	return true
}

// SetProviderURL retains only a song URL tied to this provider's exact ID.
func SetProviderURL(track *canonical.Track, provider, raw string) {
	if track.IDs != nil {
		if clean := providerURL(provider, track.IDs[provider], raw); clean != "" {
			track.IDs[provider+"_url"] = clean
		}
	}
}

func providerURL(provider, id, raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return ""
	}
	for key := range q {
		if key == "uo" || key == "at" || key == "ct" || key == "app" || key == "si" || strings.HasPrefix(key, "utm_") {
			q.Del(key)
		}
	}
	u.RawQuery = q.Encode()
	clean := u.String()
	if !validURL(provider, id, clean) {
		return ""
	}
	return clean
}

func ProviderIDs(provider string, ids map[string]string) map[string]string {
	result := map[string]string{}
	for key, value := range ids {
		if key == provider || key == provider+"_url" || key == provider+"_track_id" {
			result[key] = value
		}
	}
	return result
}
