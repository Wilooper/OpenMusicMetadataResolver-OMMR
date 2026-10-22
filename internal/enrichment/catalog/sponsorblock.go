package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"time"

	"github.com/ommr/ommr/internal/models/canonical"
)

var youtubeVideoID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

type SponsorBlock struct {
	Client         *Client
	base           string
	RequestTimeout time.Duration
}

func NewSponsorBlock(c *Client) *SponsorBlock {
	return &SponsorBlock{Client: c, base: "https://sponsor.ajay.app"}
}
func (*SponsorBlock) Name() string { return "sponsorblock" }
func (s *SponsorBlock) Timeout() time.Duration {
	if s.RequestTimeout > 0 {
		return s.RequestTimeout
	}
	return 2 * time.Second
}

type PointOfInterest struct {
	ID                        string `json:"id"`
	TimeMS                    int64  `json:"time_ms"`
	Category                  string `json:"category"`
	ActionType                string `json:"action_type"`
	Votes                     int    `json:"votes"`
	Locked                    int    `json:"locked"`
	SubmissionVideoDurationMS int64  `json:"submission_video_duration_ms,omitempty"`
	DurationChecked           bool   `json:"duration_checked"`
	Description               string `json:"description,omitempty"`
}

type VideoAnnotations struct {
	Source          string            `json:"source"`
	SourceURL       string            `json:"source_url"`
	License         string            `json:"license"`
	LicenseURL      string            `json:"license_url"`
	Attribution     string            `json:"attribution"`
	Service         string            `json:"service"`
	VideoID         string            `json:"video_id"`
	VideoDurationMS int64             `json:"video_duration_ms,omitempty"`
	Points          []PointOfInterest `json:"points_of_interest"`
	Truncated       bool              `json:"truncated,omitempty"`
}

type sponsorBlockSegment struct {
	Segment       []float64 `json:"segment"`
	ID            string    `json:"UUID"`
	Category      string    `json:"category"`
	ActionType    string    `json:"actionType"`
	VideoDuration float64   `json:"videoDuration"`
	Votes         int       `json:"votes"`
	Locked        int       `json:"locked"`
	Description   string    `json:"description"`
}

func (s *SponsorBlock) Enrich(ctx context.Context, track canonical.Track) (*Result, error) {
	id := track.IDs["ytmusic"]
	if !youtubeVideoID.MatchString(id) {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, s.Timeout())
	defer cancel()
	sum := sha256.Sum256([]byte(id))
	params := url.Values{"categories": {`["poi_highlight"]`}, "actionTypes": {`["poi"]`}, "service": {"YouTube"}}
	endpoint := s.base + "/api/skipSegments/" + hex.EncodeToString(sum[:2]) + "?" + params.Encode()
	var bucket []struct {
		VideoID  string                `json:"videoID"`
		Segments []sponsorBlockSegment `json:"segments"`
	}
	client := *s.Client
	httpClient := *s.Client.HTTP
	httpClient.Timeout = s.Timeout()
	client.HTTP = &httpClient
	if err := client.request(ctx, s.Name(), http.MethodGet, endpoint, "", nil, &bucket, requestCachePolicy{ttl: time.Hour, notFoundTTL: 5 * time.Minute, notFoundJSON: []byte("[]")}); err != nil {
		return nil, err
	}
	var segments []sponsorBlockSegment
	found := false
	for _, entry := range bucket {
		if entry.VideoID != id {
			continue
		}
		if found {
			return nil, fmt.Errorf("duplicate SponsorBlock video entry")
		}
		found = true
		segments = entry.Segments
	}
	videoDuration := acceptedVideoDuration(track, id)
	data := VideoAnnotations{Source: "SponsorBlock", SourceURL: "https://sponsor.ajay.app", License: "CC-BY-NC-SA-4.0", LicenseURL: "https://creativecommons.org/licenses/by-nc-sa/4.0/", Attribution: "Uses SponsorBlock data from https://sponsor.ajay.app", Service: "YouTube", VideoID: id, VideoDurationMS: videoDuration}
	seen := map[string]PointOfInterest{}
	for _, segment := range segments {
		if segment.Category != "poi_highlight" || segment.ActionType != "poi" || len(segment.Segment) != 2 || segment.ID == "" || len(segment.ID) > 128 {
			continue
		}
		at, end, duration := segment.Segment[0], segment.Segment[1], segment.VideoDuration
		if !finiteSeconds(at) || !finiteSeconds(end) || !finiteSeconds(duration) || at != end {
			continue
		}
		atMS, durationMS := int64(math.Round(at*1000)), int64(math.Round(duration*1000))
		if (videoDuration > 0 && atMS > videoDuration) || (durationMS > 0 && atMS > durationMS) {
			continue
		}
		checked := videoDuration > 0 && durationMS > 0
		if checked && math.Abs(float64(videoDuration-durationMS)) > 1000 {
			continue
		}
		point := PointOfInterest{ID: segment.ID, TimeMS: atMS, Category: segment.Category, ActionType: segment.ActionType, Votes: segment.Votes, Locked: segment.Locked, SubmissionVideoDurationMS: durationMS, DurationChecked: checked, Description: segment.Description}
		if previous, ok := seen[point.ID]; ok {
			if previous != point {
				return nil, fmt.Errorf("conflicting SponsorBlock point")
			}
			continue
		}
		seen[point.ID] = point
		data.Points = append(data.Points, point)
	}
	if len(data.Points) == 0 {
		return nil, nil
	}
	sort.Slice(data.Points, func(i, j int) bool {
		if data.Points[i].TimeMS == data.Points[j].TimeMS {
			return data.Points[i].ID < data.Points[j].ID
		}
		return data.Points[i].TimeMS < data.Points[j].TimeMS
	})
	if len(data.Points) > 100 {
		data.Points = data.Points[:100]
		data.Truncated = true
	}
	return &Result{Data: data}, nil
}

func finiteSeconds(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1e9
}

// A song's merged duration is not necessarily the duration of its video upload.
func acceptedVideoDuration(track canonical.Track, id string) int64 {
	var records map[string]struct {
		Metadata struct {
			IDs        map[string]string `json:"ids"`
			DurationMS int64             `json:"duration_ms"`
		} `json:"metadata"`
	}
	if json.Unmarshal(track.Extensions["provider_data"], &records) != nil {
		return 0
	}
	if record := records["ytmusic"].Metadata; record.IDs["ytmusic"] == id && record.DurationMS > 0 {
		return record.DurationMS
	}
	return 0
}
