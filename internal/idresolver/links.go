package idresolver

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/ommr/ommr/internal/adapters"
)

var ErrInput = errors.New("invalid music input")
var spotifyTrack = regexp.MustCompile(`^[A-Za-z0-9]{22}$`)
var youtubeTrack = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
var number = regexp.MustCompile(`^[0-9]{1,18}$`)
var shareToken = regexp.MustCompile(`^[A-Za-z0-9_,+-]{1,128}$`)
var recordingID = regexp.MustCompile(`(?i)^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)
var spotifyLocale = regexp.MustCompile(`^intl-[a-zA-Z]{2}(?:-[a-zA-Z]{2})?$`)
var locale = regexp.MustCompile(`^[a-zA-Z]{2}(?:-[a-zA-Z]{2})?$`)
var soundcloudPart = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Link describes an identity parsed locally, never an arbitrary fetch target.
type Link struct{ Provider, Field, ID string }

func invalid(reason string) error { return fmt.Errorf("%w: %s", ErrInput, reason) }

func ParseTrackURL(raw string) (Link, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) > 4096 || strings.IndexFunc(raw, unicode.IsControl) >= 0 {
		return Link{}, invalid("invalid link length or control character")
	}
	if strings.HasPrefix(raw, "spotify:track:") {
		id := strings.TrimPrefix(raw, "spotify:track:")
		if spotifyTrack.MatchString(id) {
			return Link{"spotify", "spotify_id", id}, nil
		}
		return Link{}, invalid("invalid Spotify track URI")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Port() != "" || u.Opaque != "" || strings.Contains(u.Host, ":") {
		return Link{}, invalid("use a supported HTTPS music link")
	}
	host := strings.ToLower(u.Host)
	// Encoded separators and dot segments must not change the route's meaning.
	if escaped := strings.ToLower(u.EscapedPath()); strings.Contains(escaped, "%2f") || strings.Contains(escaped, "%5c") || strings.Contains(u.Path, `\`) {
		return Link{}, invalid("encoded path separators are unsupported")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for _, p := range parts {
		if p == "" || p == "." || p == ".." {
			return Link{}, invalid("invalid music link path")
		}
	}
	params, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return Link{}, invalid("malformed link query")
	}
	for key, values := range params {
		if (key == "v" || key == "i") && len(values) != 1 {
			return Link{}, invalid("ambiguous track identifier")
		}
	}
	link := Link{}
	switch host {
	case "open.spotify.com":
		p := parts
		if len(p) > 0 && (p[0] == "embed" || spotifyLocale.MatchString(p[0])) {
			p = p[1:]
		}
		if len(p) == 2 && p[0] == "track" && spotifyTrack.MatchString(p[1]) {
			link = Link{"spotify", "spotify_id", p[1]}
		}
	case "youtube.com", "www.youtube.com", "m.youtube.com", "music.youtube.com", "youtu.be", "www.youtu.be":
		id := ""
		if host == "youtu.be" || host == "www.youtu.be" {
			if len(parts) == 1 {
				id = parts[0]
			}
		} else if len(parts) == 1 && parts[0] == "watch" {
			id = params.Get("v")
		} else if len(parts) == 2 && (parts[0] == "shorts" || parts[0] == "embed" || parts[0] == "live") {
			id = parts[1]
		}
		if youtubeTrack.MatchString(id) {
			link = Link{"ytmusic", "youtube_id", id}
		}
	case "music.apple.com", "itunes.apple.com":
		if len(parts) >= 3 && locale.MatchString(parts[0]) {
			id := ""
			if parts[1] == "song" && (len(parts) == 3 || len(parts) == 4) {
				id = parts[len(parts)-1]
			} else if parts[1] == "album" && (len(parts) == 3 || len(parts) == 4) {
				id = params.Get("i")
			}
			if number.MatchString(id) {
				link = Link{"applemusic", "apple_id", id}
			}
		}
	case "deezer.com", "www.deezer.com":
		p := parts
		if len(p) == 3 && locale.MatchString(p[0]) {
			p = p[1:]
		}
		if len(p) == 2 && p[0] == "track" && number.MatchString(p[1]) {
			link = Link{"deezer", "deezer_id", p[1]}
		}
	case "jiosaavn.com", "www.jiosaavn.com":
		if len(parts) == 3 && parts[0] == "song" && shareToken.MatchString(parts[2]) {
			canonical := url.URL{Scheme: "https", Host: "www.jiosaavn.com", Path: "/" + strings.Join(parts, "/")}
			link = Link{"jiosaavn", "jiosaavn_url", canonical.String()}
		}
	case "soundcloud.com", "www.soundcloud.com", "m.soundcloud.com":
		if len(parts) == 2 && soundcloudPart.MatchString(parts[0]) && soundcloudPart.MatchString(parts[1]) && parts[1] != "sets" && parts[0] != "discover" && parts[0] != "you" {
			link = Link{"soundcloud", "soundcloud_id", strings.Join(parts, "/")}
		}
	case "musicbrainz.org":
		if len(parts) == 2 && parts[0] == "recording" && recordingID.MatchString(parts[1]) {
			link = Link{"musicbrainz", "musicbrainz_id", strings.ToLower(parts[1])}
		}
	case "tidal.com", "www.tidal.com", "listen.tidal.com":
		p := parts
		if len(p) == 3 && p[0] == "browse" {
			p = p[1:]
		}
		if len(p) == 2 && p[0] == "track" && number.MatchString(p[1]) {
			link = Link{"tidal", "tidal_id", p[1]}
		}
	case "open.qobuz.com", "play.qobuz.com":
		if len(parts) == 2 && parts[0] == "track" && number.MatchString(parts[1]) {
			link = Link{"qobuz", "qobuz_id", parts[1]}
		}
	}
	if link.ID == "" {
		return Link{}, invalid("unsupported track link; use a direct song or recording URL")
	}
	return link, nil
}

func fields(q *adapters.Query) map[string]*string {
	return map[string]*string{
		"spotify_id": &q.SpotifyID, "youtube_id": &q.YouTubeID, "deezer_id": &q.DeezerID, "apple_id": &q.AppleID, "soundcloud_id": &q.SoundCloudID, "qobuz_id": &q.QobuzID, "tidal_id": &q.TidalID, "amazonmusic_id": &q.AmazonMusicID, "pandora_id": &q.PandoraID, "bugs_id": &q.BugsID, "genie_id": &q.GenieID, "melon_id": &q.MelonID, "jiosaavn_id": &q.JioSaavnID, "jiosaavn_url": &q.JioSaavnURL, "musicbrainz_id": &q.MusicBrainzID,
	}
}

func NormalizeQuery(q adapters.Query) (adapters.Query, error) {
	inputs := fields(&q)
	if q.URL != "" {
		for _, value := range inputs {
			if *value != "" {
				return q, invalid("url cannot be combined with provider identifiers")
			}
		}
		if q.ISRC != "" {
			return q, invalid("url cannot be combined with isrc")
		}
		link, err := ParseTrackURL(q.URL)
		if err != nil {
			return q, err
		}
		*inputs[link.Field] = link.ID
		q.URL = ""
	}
	for field, value := range inputs {
		*value = strings.TrimSpace(*value)
		if *value == "" {
			continue
		}
		if field == "jiosaavn_url" || strings.Contains(*value, ":") || strings.Contains(*value, "://") {
			link, err := ParseTrackURL(*value)
			if err != nil {
				return q, err
			}
			if link.Field != field {
				return q, invalid("link does not match its provider input field")
			}
			*value = link.ID
		}
	}
	if q.JioSaavnURL != "" {
		for field, value := range inputs {
			if field != "jiosaavn_url" && *value != "" {
				return q, invalid("jiosaavn_url cannot be combined with another provider identifier")
			}
		}
	}
	q.MusicBrainzID = strings.ToLower(q.MusicBrainzID)
	if q.MusicBrainzID != "" && !recordingID.MatchString(q.MusicBrainzID) {
		return q, invalid("invalid MusicBrainz recording ID")
	}
	return q, nil
}
