package ytmusic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCleanChannelName(t *testing.T) {
	tests := []struct {
		in, out string
	}{
		{"Talwiinder - Topic", "Talwiinder"},
		{"AP Dhillon - Topic", "AP Dhillon"},
		{"Artist - VEVO", "Artist"},
		{"ArtistVEVO", "Artist"},
		{"plain channel", "plain channel"},
	}
	for _, tt := range tests {
		if got := CleanChannelName(tt.in); got != tt.out {
			t.Errorf("CleanChannelName(%q) = %q; want %q", tt.in, got, tt.out)
		}
	}
}

func TestParseLengthToMS(t *testing.T) {
	tests := []struct {
		in  string
		out int64
	}{
		{"3:38", 218000},
		{"1:02:05", 3725000},
		{"42", 42000},
		{"", 0},
	}
	for _, tt := range tests {
		if got := parseLengthToMS(tt.in); got != tt.out {
			t.Errorf("parseLengthToMS(%q) = %d; want %d", tt.in, got, tt.out)
		}
	}
}

const sampleSearchResponse = `{
  "contents": {
    "twoColumnSearchResultsRenderer": {
      "primaryContents": {
        "sectionListRenderer": {
          "contents": [
            {
              "itemSectionRenderer": {
                "contents": [
                  {
                    "videoRenderer": {
                      "videoId": "FVNSACXFAy0",
                      "title": {"runs": [{"text": "Tu"}]},
                      "ownerText": {"runs": [{"text": "Talwiinder - Topic"}]},
                      "lengthText": {"simpleText": "3:38"},
                      "thumbnail": {"thumbnails": [{"url": "https://i.ytimg.com/vi/FVNSACXFAy0/hqdefault.jpg", "width": 480, "height": 360}]}
                    }
                  }
                ]
              }
            }
          ]
        }
      }
    }
  }
}`

func TestInnerTubeSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleSearchResponse))
	}))
	defer srv.Close()

	it := NewInnerTube(srv.Client())

	origURL := innerTubeBaseURL
	innerTubeBaseURL = srv.URL
	defer func() { innerTubeBaseURL = origURL }()

	videos, err := it.SearchYouTube(context.Background(), "Talwiinder Tu", 5)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(videos) != 1 {
		t.Fatalf("expected 1 video, got %d", len(videos))
	}

	track := it.parseVideo(videos[0])
	if track.Title != "Tu" {
		t.Errorf("expected title 'Tu', got %q", track.Title)
	}
	if track.Artists[0].Name != "Talwiinder" {
		t.Errorf("expected artist 'Talwiinder', got %q", track.Artists[0].Name)
	}
	if track.DurationMS != 218000 {
		t.Errorf("expected duration 218000, got %d", track.DurationMS)
	}
	if track.IDs["ytmusic"] != "FVNSACXFAy0" {
		t.Errorf("expected video id FVNSACXFAy0, got %q", track.IDs["ytmusic"])
	}
}
