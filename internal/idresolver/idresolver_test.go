package idresolver

import (
	"testing"
)

func TestExtractSpotifyID(t *testing.T) {
	r := New()
	tests := []struct {
		input    string
		expected string
	}{
		{"4cOdK2wGLETKBW3PvgPWqT", "4cOdK2wGLETKBW3PvgPWqT"},
		{"https://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT", "4cOdK2wGLETKBW3PvgPWqT"},
		{"https://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT?si=abc123", "4cOdK2wGLETKBW3PvgPWqT"},
	}

	for _, tt := range tests {
		got := r.ExtractSpotifyID(tt.input)
		if got != tt.expected {
			t.Errorf("ExtractSpotifyID(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

func TestExtractYouTubeID(t *testing.T) {
	r := New()
	tests := []struct {
		input    string
		expected string
	}{
		{"dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"https://youtu.be/dQw4w9WgXcQ?t=10", "dQw4w9WgXcQ"},
	}

	for _, tt := range tests {
		got := r.ExtractYouTubeID(tt.input)
		if got != tt.expected {
			t.Errorf("ExtractYouTubeID(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}
