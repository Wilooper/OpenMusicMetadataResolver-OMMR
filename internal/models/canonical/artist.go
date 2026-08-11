package canonical

// Artist represents a contributing artist for a track or album.
type Artist struct {
	Name string            `json:"name"`
	Role string            `json:"role,omitempty"` // e.g. "main", "featured", "remixer"
	IDs  map[string]string `json:"ids,omitempty"`  // Provider -> Artist ID mapping
}
