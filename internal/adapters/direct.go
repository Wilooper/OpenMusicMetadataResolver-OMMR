package adapters

import (
	"net/url"
	"strings"

	"github.com/ommr/ommr/internal/models/canonical"
)

type DirectInput struct{ Provider, Kind, ID, IdentityID, Field string }

func (q Query) DirectInput() DirectInput {
	for _, entry := range []DirectInput{
		{"spotify", "spotify", q.SpotifyID, q.SpotifyID, "spotify_id"},
		{"ytmusic", "youtube", q.YouTubeID, q.YouTubeID, "youtube_id"},
		{"deezer", "deezer", q.DeezerID, q.DeezerID, "deezer_id"},
		{"applemusic", "applemusic", q.AppleID, q.AppleID, "apple_id"},
		{"soundcloud", "soundcloud", q.SoundCloudID, q.SoundCloudID, "soundcloud_id"},
		{"qobuz", "qobuz", q.QobuzID, q.QobuzID, "qobuz_id"},
		{"tidal", "tidal", q.TidalID, q.TidalID, "tidal_id"},
		{"amazonmusic", "amazonmusic", q.AmazonMusicID, q.AmazonMusicID, "amazonmusic_id"},
		{"bugs", "bugs", q.BugsID, q.BugsID, "bugs_id"},
		{"genie", "genie", q.GenieID, q.GenieID, "genie_id"},
		{"melon", "melon", q.MelonID, q.MelonID, "melon_id"},
		{"pandora", "pandora", q.PandoraID, q.PandoraID, "pandora_id"},
		{"musicbrainz", "musicbrainz", q.MusicBrainzID, q.MusicBrainzID, "musicbrainz_id"},
	} {
		if entry.ID != "" {
			return entry
		}
	}
	if q.JioSaavnURL != "" {
		token := ""
		if u, err := url.Parse(q.JioSaavnURL); err == nil {
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(parts) == 3 {
				token = parts[2]
			}
		}
		return DirectInput{"jiosaavn", "jiosaavn_url", q.JioSaavnURL, token, "jiosaavn_url"}
	}
	if q.JioSaavnID != "" {
		kind := "jiosaavn_share"
		if len(q.JioSaavnID) == 8 {
			kind = "jiosaavn_track_id"
		}
		return DirectInput{"jiosaavn", kind, q.JioSaavnID, q.JioSaavnID, "jiosaavn_id"}
	}
	return DirectInput{}
}

func DirectIDMatch(q Query, c canonical.TrackCandidate) bool {
	direct := q.DirectInput()
	if direct.Provider == "" || direct.IdentityID == "" || !strings.EqualFold(direct.Provider, c.Provider) {
		return false
	}
	if direct.Kind == "jiosaavn_track_id" && c.Track.IDs["jiosaavn_track_id"] != "" {
		return c.Track.IDs["jiosaavn_track_id"] == direct.IdentityID
	}
	return c.Track.IDs[direct.Provider] == direct.IdentityID
}
