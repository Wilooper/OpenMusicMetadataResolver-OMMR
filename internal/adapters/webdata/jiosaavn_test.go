package webdata

import (
	"os"
	"strings"
	"testing"
)

func TestJioSaavnSerializedStateBoundaries(t *testing.T) {
	body, err := os.ReadFile("../jiosaavn/testdata/mera-page-captured.html")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		change func(string) string
		valid  bool
	}{
		{"captured", func(s string) string { return s }, true},
		{"pure JSON", func(s string) string {
			return strings.ReplaceAll(strings.ReplaceAll(s, `new Date("2026-10-17T01:04:44.749Z")`, `null`), `undefined`, `null`)
		}, true},
		{"commented property", func(s string) string {
			return strings.Replace(s, `{"mediaElement"`, `{/* "song": {} */ "mediaElement"`, 1)
		}, false},
		{"expression after song", func(s string) string { return strings.Replace(s, `,"login":`, ` || other,"login":`, 1) }, false},
		{"duplicate song", func(s string) string { return strings.Replace(s, `,"login":`, `,"song":{},"login":`, 1) }, false},
		{"unterminated state", func(s string) string { return strings.Replace(s, `}};</script>`, `};</script>`, 1) }, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeHTML([]byte(test.change(string(body))), Request{Provider: "jiosaavn", Mode: "track", ID: "ERIkRAwJf38"})
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v err=%v", test.valid, err)
			}
		})
	}
}
