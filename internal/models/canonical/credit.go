package canonical

// Credit represents contributor credit details for a track (e.g. Composer, Lyricist, Producer).
type Credit struct {
	Name  string   `json:"name"`
	Roles []string `json:"roles"` // e.g. ["Composer", "Producer", "Lyricist"]
}
