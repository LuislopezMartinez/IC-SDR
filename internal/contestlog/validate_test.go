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
