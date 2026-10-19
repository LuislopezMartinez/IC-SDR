package feedback

import (
	"strings"
	"testing"
)

func TestConfigOnlyAcceptsGoogleForms(t *testing.T) {
	config := Config{FormAction: "https://example.com/formResponse", Fields: FieldConfig{Type: "entry.1", Name: "entry.2", Email: "entry.3", Comment: "entry.4", Technical: "entry.5"}}
	if config.Validate() == nil {
		t.Fatal("accepted a non-Google submission endpoint")
	}
	config.FormAction = "https://docs.google.com/forms/d/e/test/formResponse"
	if err := config.Validate(); err != nil {
		t.Fatalf("valid Google Forms configuration rejected: %v", err)
	}
}

func TestTechnicalSummaryOnlyIncludesReceiverContextWhenRequested(t *testing.T) {
	info := TechnicalInfo{Version: "v1.2.3", SDR: "RTL-SDR", Mode: "FM", Band: "2 m", Frequency: "145.500 MHz"}
	minimal := TechnicalSummary(info, false)
	if strings.Contains(minimal, "145.500") || strings.Contains(minimal, "Modo:") {
		t.Fatalf("minimal summary leaked receiver context: %s", minimal)
	}
	complete := TechnicalSummary(info, true)
	if !strings.Contains(complete, "145.500 MHz") || !strings.Contains(complete, "Modo: FM") {
		t.Fatalf("complete summary lacks receiver context: %s", complete)
	}
}
