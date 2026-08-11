package canonical

// TrackCandidate encapsulates normalized metadata returned from a single provider adapter before matching & merging.
type TrackCandidate struct {
	Provider    string  `json:"provider"`
	Track       Track   `json:"track"`
	MatchScore  float64 `json:"match_score"`
	RawResponse []byte  `json:"-"`
}
