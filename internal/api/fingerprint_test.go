package api

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFingerprintValidatesConfigurationAndPayload(t *testing.T) {
	h, _ := responseFixture(t)
	for _, tt := range []struct {
		method, body string
		key          string
		status       int
	}{
		{"GET", "", "", 405}, {"POST", `{}`, "", 503}, {"POST", `{}`, "configured", 400},
		{"POST", `{"fingerprint":"fp","duration":0}`, "configured", 400},
		{"POST", `{"fingerprint":"fp","duration":180} {}`, "configured", 400},
	} {
		h.cfg.AcoustIDKey = tt.key
		rec := httptest.NewRecorder()
		h.HandleFingerprint(rec, httptest.NewRequest(tt.method, "/v1/fingerprint", strings.NewReader(tt.body)))
		if rec.Code != tt.status {
			t.Fatalf("status %d != %d", rec.Code, tt.status)
		}
	}
}
func TestProductionIncludesExtraProviderMetadataWithoutDiagnostics(t *testing.T) {
	h, _ := responseFixture(t)
	got, _ := readResponse(t, h.HandleResolve, "GET", "/v1/resolve?artist=Singer&title=Song", "")
	track := got["track"].(map[string]any)
	data := track["provider_data"].(map[string]any)
	if data["deezer"] == nil || track["explicit_status"] != "unknown" {
		t.Fatal("extra data or unknown flag lost")
	}
	if _, ok := got["enrichment_status"]; ok {
		t.Fatal("enrichment diagnostics leaked")
	}
}

func TestLargeSearchPageDoesNotOverflow(t *testing.T) {
	h, _ := responseFixture(t)
	got, _ := readResponse(t, h.HandleSearch, "GET", "/v1/search?q=Song&page=9223372036854775807&limit=50", "")
	if len(got["results"].([]any)) != 0 {
		t.Fatal("huge page should be empty")
	}
}
