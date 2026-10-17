package adapters

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestReadJSONBody(t *testing.T) {
	for _, tc := range []struct {
		body  string
		valid bool
	}{
		{`{"results":[]}`, true},
		{"\n{\"results\":[]}", true},
		{`<html>Site Unavailable</html>`, false},
		{`{"access_token":"secret"`, false},
		{strings.Repeat(" ", 8<<20) + "{}", false},
	} {
		resp := &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(tc.body))}
		_, err := ReadJSONBody(resp)
		if (err == nil) != tc.valid {
			t.Fatalf("valid=%v, error=%v", tc.valid, err)
		}
		if err != nil && strings.Contains(err.Error(), "secret") {
			t.Fatal("response body leaked in error")
		}
	}
}
