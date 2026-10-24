package contestlog

import (
	"testing"
	"time"
)

func TestSubmissionValidation(t *testing.T) {
	s, _ := NewOceaniaDXSSB("EA1ABC", "", "LOW", time.Now())
	if got := len(s.ValidateSubmission()); got != 5 {
		t.Fatalf("empty metadata issues = %d, want 5", got)
	}
	s.Name, s.Email, s.Address, s.Country = "Luis", "luis@example.com", "Calle Radio 1", "Spain"
	_, _ = s.Add(Entry{Time: time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC), FrequencyHz: 14_200_000, Mode: "USB", WorkedCall: "VK2AAA", SerialReceived: 1})
	if issues := s.ValidateSubmission(); len(issues) != 0 {
		t.Fatalf("valid submission issues: %+v", issues)
	}
	s.QSOs[0].Band = "40M"
	issues := s.ValidateSubmission()
	if len(issues) != 1 || issues[0].Code != "qso_band" || issues[0].QSOIndex != 0 {
		t.Fatalf("band issue does not identify its QSO: %+v", issues)
	}
}

func TestCQWWValidationRejectsInvalidZones(t *testing.T) {
	when := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	s, _ := NewSession(CQWWSSB, "EA1ABC", "", Config{Power: "LOW", SentValue: "41"}, when)
	s.Name, s.Email, s.Address, s.Country = "Luis", "luis@example.com", "Calle Radio 1", "Spain"
	_, _ = s.Add(Entry{Time: when, FrequencyHz: 14_200_000, Mode: "USB", WorkedCall: "K1ABC", ExchangeReceived: "41"})
	issues := s.ValidateSubmission()
	want := map[string]bool{"cq_zone_sent": false, "cq_zone_received": false}
	for _, issue := range issues {
		if _, ok := want[issue.Code]; ok {
			want[issue.Code] = true
		}
	}
	for code, found := range want {
		if !found {
			t.Fatalf("missing %s in %+v", code, issues)
		}
	}
}
