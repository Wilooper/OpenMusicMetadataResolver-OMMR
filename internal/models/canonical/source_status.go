package canonical

// SourceStatus provides diagnostic metadata regarding provider execution status.
type SourceStatus struct {
	Name            string  `json:"name"`
	Success         bool    `json:"success"`
	Matched         bool    `json:"matched"`
	Contributed     bool    `json:"contributed"`
	Cached          bool    `json:"cached"`
	LatencyMS       int64   `json:"latency_ms"`
	Error           string  `json:"error,omitempty"`
	RejectionReason string  `json:"rejection_reason,omitempty"` // "no_results", "title_mismatch", "artist_mismatch", "duration_mismatch", "below_threshold", "provider_error"
	CandidateScore  float64 `json:"candidate_score,omitempty"`
	Version         string  `json:"version"` // e.g. "deezer-v1", "musicbrainz-v2"
}
