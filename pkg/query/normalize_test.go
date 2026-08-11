package query

import (
	"testing"
)

func TestCleanTitle(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Believer (Official Video)", "believer"},
		{"Get Lucky [Official Audio]", "get lucky"},
		{"Bohemian Rhapsody (2011 Remaster)", "bohemian rhapsody"},
		{"Café de Flore (HD)", "cafe de flore"},
		{"   Multiple   Spaces   ", "multiple spaces"},
		{"", ""},
	}

	for _, tt := range tests {
		got := CleanTitle(tt.input)
		if got != tt.expected {
			t.Errorf("CleanTitle(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

func TestCleanArtist(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Beyoncé", "beyonce"},
		{"Daft Punk", "daft punk"},
		{"Mötley Crüe", "motley crue"},
		{"AP Dhillon - Topic", "ap dhillon"},
		{"KatyPerryVEVO", "katyperry"},
		{"", ""},
	}

	for _, tt := range tests {
		got := CleanArtist(tt.input)
		if got != tt.expected {
			t.Errorf("CleanArtist(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}
