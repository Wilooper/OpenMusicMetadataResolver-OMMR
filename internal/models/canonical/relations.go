package canonical

// Relations provides graph cross-references (album IDs, artist IDs, related tracks) across providers.
type Relations struct {
	AlbumIDs        []string `json:"album_ids,omitempty"`
	ArtistIDs       []string `json:"artist_ids,omitempty"`
	RelatedTrackIDs []string `json:"related_track_ids,omitempty"`
}
