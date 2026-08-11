package canonical

// ResolutionStrategy provides transparency into how a query was resolved.
type ResolutionStrategy struct {
	InputType     string `json:"input_type"`     // e.g. "youtube_id", "spotify_id", "artist_title"
	PrimarySource string `json:"primary_source"` // e.g. "ytmusic", "musicbrainz"
	CrossResolved bool   `json:"cross_resolved"`
}
