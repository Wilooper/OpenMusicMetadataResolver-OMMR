package ytmusic

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ommr/ommr/internal/models/canonical"
	"github.com/ommr/ommr/internal/models/provider"
)

// innerTubeAPIKey is the public API key used by the YouTube web client. It is
// required to invoke the internal /youtubei endpoints used for zero-key
// search and player metadata retrieval.
const innerTubeAPIKey = "AIzaSyAO_FJ2SlqU8Q4STEHLGCilw_Y9_11qcW8"

var innerTubeBaseURL = "https://www.youtube.com/youtubei/v1"

// InnerTube is a thin client for YouTube's internal web API endpoints
// (search + player) used to enrich and search video metadata without keys.
type InnerTube struct {
	httpClient *http.Client
	cookie     string
}

func NewInnerTube(client *http.Client) *InnerTube {
	return &InnerTube{httpClient: client}
}

func (i *InnerTube) setAuth(req *http.Request) {
	if i.cookie == "" || req.URL.Scheme != "https" || req.URL.Hostname() != "www.youtube.com" {
		return
	}
	if strings.ContainsAny(i.cookie, "\r\n") {
		return
	}
	var sapisid string
	for _, part := range strings.Split(i.cookie, ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok && (key == "SAPISID" || key == "__Secure-3PAPISID") {
			sapisid = value
			break
		}
	}
	if sapisid == "" {
		return
	}
	origin := "https://www.youtube.com"
	timestamp := fmt.Sprint(time.Now().Unix())
	digest := sha1.Sum([]byte(timestamp + " " + sapisid + " " + origin))
	req.Header.Set("Cookie", i.cookie)
	req.Header.Set("Origin", origin)
	req.Header.Set("Authorization", fmt.Sprintf("SAPISIDHASH %s_%x", timestamp, digest))
}

// SearchYouTube searches YouTube for videos matching the given query and
// returns raw videoRenderer items.
func (i *InnerTube) SearchYouTube(ctx context.Context, query string, limit int) ([]innerTubeVideo, error) {
	bodyPayload := map[string]interface{}{
		"context": map[string]interface{}{
			"client": map[string]interface{}{
				"clientName":    "WEB",
				"clientVersion": "2.20250101.00.00",
			},
		},
		"query": query,
	}

	bodyBytes, err := json.Marshal(bodyPayload)
	if err != nil {
		return nil, err
	}

	reqURL := fmt.Sprintf("%s/search?key=%s", innerTubeBaseURL, innerTubeAPIKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	i.setAuth(req)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("youtube innertube search status %d", resp.StatusCode)
	}

	var parsed innerTubeSearchResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}

	videos := make([]innerTubeVideo, 0, limit)
	for _, content := range parsed.Contents.TwoColumnSearchResults.PrimaryContents.SectionList.Contents {
		if content.ItemSectionRenderer == nil {
			continue
		}
		for _, item := range content.ItemSectionRenderer.Contents {
			if item.VideoRenderer == nil {
				continue
			}
			v := item.VideoRenderer
			videos = append(videos, *v)
			if len(videos) >= limit {
				return videos, nil
			}
		}
	}
	return videos, nil
}

// FetchPlayer fetches the player metadata (videoDetails) for a video ID.
func (i *InnerTube) FetchPlayer(ctx context.Context, videoID string) (*provider.YTPlayerResponse, error) {
	bodyPayload := map[string]interface{}{
		"context": map[string]interface{}{
			"client": map[string]interface{}{
				"clientName":    "WEB",
				"clientVersion": "2.20250101.00.00",
			},
		},
		"videoId": videoID,
	}

	bodyBytes, err := json.Marshal(bodyPayload)
	if err != nil {
		return nil, err
	}

	reqURL := fmt.Sprintf("%s/player?key=%s", innerTubeBaseURL, innerTubeAPIKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	i.setAuth(req)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("youtube innertube player status %d", resp.StatusCode)
	}

	var player provider.YTPlayerResponse
	if err := json.Unmarshal(body, &player); err != nil {
		return nil, err
	}
	if player.VideoDetails.VideoID == "" && player.VideoDetails.Title == "" {
		return nil, fmt.Errorf("youtube player returned empty videoDetails")
	}
	return &player, nil
}

func (i *InnerTube) parseVideo(v innerTubeVideo) canonical.Track {
	title := flattenRuns(v.Title.Runs)
	if title == "" {
		title = v.Title.SimpleText
	}

	artist := flattenRuns(v.OwnerText.Runs)
	if artist == "" {
		artist = v.OwnerText.SimpleText
	}
	artist = CleanChannelName(artist)

	durationMS := parseLengthToMS(v.LengthText.SimpleText)

	images := make([]canonical.Image, 0, len(v.Thumbnail.Thumbnails))
	for _, th := range v.Thumbnail.Thumbnails {
		images = append(images, canonical.Image{URL: th.URL, Width: th.Width, Height: th.Height, Type: "thumbnail"})
	}

	return canonical.Track{
		Title:      title,
		Artists:    []canonical.Artist{{Name: artist, Role: "main"}},
		DurationMS: durationMS,
		Images:     images,
		IDs:        map[string]string{"ytmusic": v.VideoID},
		Sources:    []string{ProviderName},
	}
}

// CleanChannelName strips common auto-generated channel suffixes from the
// channel name so that "Artist - Topic" becomes "Artist".
func CleanChannelName(name string) string {
	cleaned := name
	if strings.Contains(cleaned, " - Topic") {
		cleaned = strings.SplitN(cleaned, " - Topic", 2)[0]
	}
	cleaned = strings.TrimSuffix(cleaned, " - VEVO")
	cleaned = strings.TrimSuffix(cleaned, "VEVO")
	cleaned = strings.TrimSuffix(cleaned, " - Topic")
	return strings.TrimSpace(cleaned)
}

var lengthPattern = regexp.MustCompile(`^(?:(\d+):)?(\d+):(\d+)$`)

func parseLengthToMS(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// Formats: "3:45", "1:02:05", "12345" (seconds)
	m := lengthPattern.FindStringSubmatch(s)
	if m != nil {
		h, _ := strconv.Atoi(m[1])
		min, _ := strconv.Atoi(m[2])
		sec, _ := strconv.Atoi(m[3])
		total := h*3600 + min*60 + sec
		return int64(total) * 1000
	}
	secs, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return int64(secs) * 1000
}

func flattenRuns(runs []innerTubeRun) string {
	if len(runs) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, r := range runs {
		sb.WriteString(r.Text)
	}
	return sb.String()
}

// --- InnerTube response model ---

type innerTubeSearchResponse struct {
	Contents innerTubeSearchContents `json:"contents"`
}

type innerTubeSearchContents struct {
	TwoColumnSearchResults struct {
		PrimaryContents struct {
			SectionList struct {
				Contents []struct {
					ItemSectionRenderer *struct {
						Contents []struct {
							VideoRenderer *innerTubeVideo `json:"videoRenderer"`
						} `json:"contents"`
					} `json:"itemSectionRenderer"`
				} `json:"contents"`
			} `json:"sectionListRenderer"`
		} `json:"primaryContents"`
	} `json:"twoColumnSearchResultsRenderer"`
}

type innerTubeVideo struct {
	VideoID string `json:"videoId"`
	Title   struct {
		Runs       []innerTubeRun `json:"runs"`
		SimpleText string         `json:"simpleText"`
	} `json:"title"`
	OwnerText struct {
		Runs       []innerTubeRun `json:"runs"`
		SimpleText string         `json:"simpleText"`
	} `json:"ownerText"`
	LengthText struct {
		SimpleText string `json:"simpleText"`
	} `json:"lengthText"`
	Thumbnail struct {
		Thumbnails []struct {
			URL    string `json:"url"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
		} `json:"thumbnails"`
	} `json:"thumbnail"`
	PublishedTimeText struct {
		SimpleText string `json:"simpleText"`
	} `json:"publishedTimeText"`
}

type innerTubeRun struct {
	Text string `json:"text"`
}
