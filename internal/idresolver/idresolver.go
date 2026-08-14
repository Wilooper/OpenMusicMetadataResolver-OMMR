package idresolver

import (
	"strings"
)

// IDResolver resolves and normalizes platform track and video IDs.
type IDResolver struct{}

func New() *IDResolver {
	return &IDResolver{}
}

// ExtractSpotifyID parses a raw Spotify track ID or full URL (e.g., https://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT).
func (r *IDResolver) ExtractSpotifyID(input string) string {
	input = strings.TrimSpace(input)
	if strings.Contains(input, "spotify.com/track/") {
		parts := strings.Split(input, "spotify.com/track/")
		if len(parts) > 1 {
			id := strings.Split(parts[1], "?")[0]
			return strings.Trim(id, "/")
		}
	}
	return input
}

// ExtractYouTubeID parses a raw YouTube video ID or URL (e.g., https://www.youtube.com/watch?v=dQw4w9WgXcQ or https://youtu.be/dQw4w9WgXcQ).
func (r *IDResolver) ExtractYouTubeID(input string) string {
	input = strings.TrimSpace(input)
	if strings.Contains(input, "youtu.be/") {
		parts := strings.Split(input, "youtu.be/")
		if len(parts) > 1 {
			id := strings.Split(parts[1], "?")[0]
			return strings.Trim(id, "/")
		}
	}
	if strings.Contains(input, "v=") {
		parts := strings.Split(input, "v=")
		if len(parts) > 1 {
			id := strings.Split(parts[1], "&")[0]
			return id
		}
	}
	return input
}

// ExtractSoundCloudID parses a raw SoundCloud permalink or URL
// (e.g., https://soundcloud.com/artist/track or artist/track).
func (r *IDResolver) ExtractSoundCloudID(input string) string {
	input = strings.TrimSpace(input)
	if strings.Contains(input, "soundcloud.com/") {
		parts := strings.Split(input, "soundcloud.com/")
		if len(parts) > 1 {
			id := strings.Split(parts[1], "?")[0]
			return strings.Trim(id, "/")
		}
	}
	return strings.TrimPrefix(strings.TrimSpace(input), "/")
}
