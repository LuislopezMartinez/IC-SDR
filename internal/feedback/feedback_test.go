package feedback

import (
	"net/url"
	"strings"
	"testing"
)

func TestIssueURLTargetsOfficialRepositoryAndPrefillsDiagnostics(t *testing.T) {
	address := IssueURL(TechnicalInfo{
		Version: "v1.2.3", Windows: "Windows 11", Architecture: "amd64",
		SDR: "SDRplay RSP1B", SDRStatus: "Conectado",
	})
	parsed, err := url.Parse(address)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scheme != "https" || parsed.Host != "github.com" || parsed.Path != "/LuislopezMartinez/IC-SDR/issues/new" {
		t.Fatalf("unexpected issue destination: %s", address)
	}
	body := parsed.Query().Get("body")
	for _, expected := range []string{"IC-SDR: v1.2.3", "Windows 11 · amd64", "SDRplay RSP1B · Conectado"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("issue body lacks %q: %s", expected, body)
		}
	}
}

func TestTechnicalSummaryUsesFallbacks(t *testing.T) {
	summary := TechnicalSummary(TechnicalInfo{})
	if !strings.Contains(summary, "IC-SDR: desarrollo") || !strings.Contains(summary, "SDR: No detectado · Desconectado") {
		t.Fatalf("summary lacks fallbacks: %s", summary)
	}
}
