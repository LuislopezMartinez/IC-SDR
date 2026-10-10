package contestlog

import (
	"testing"
	"time"
)

func TestWPXPrefixPortableRules(t *testing.T) {
	for call, want := range map[string]string{"N8BJQ": "N8", "HG19AA": "HG19", "LY1000A": "LY1000", "N8BJQ/KH9": "KH9", "PA/N8BJQ": "PA0", "EA3ABC/P": "EA3"} {
		if got := WPXPrefix(call); got != want {
			t.Errorf("WPXPrefix(%q) = %q, want %q", call, got, want)
		}
	}
}

func TestOceaniaScoreByBandAndPrefix(t *testing.T) {
	s, _ := NewOceaniaDXSSB("EA3ABC", "", "LOW", time.Now())
	when := time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC)
	entries := []Entry{
		{Time: when, FrequencyHz: 14_200_000, Mode: "USB", WorkedCall: "VK2AAA", SerialReceived: 1},
		{Time: when, FrequencyHz: 14_210_000, Mode: "USB", WorkedCall: "VK2BBB", SerialReceived: 2},
		{Time: when, FrequencyHz: 7_100_000, Mode: "LSB", WorkedCall: "VK2CCC", SerialReceived: 3},
		{Time: when, FrequencyHz: 14_220_000, Mode: "USB", WorkedCall: "EA4ZZZ", SerialReceived: 4},
	}
	for _, entry := range entries {
		if _, err := s.Add(entry); err != nil {
			t.Fatal(err)
		}
	}
	s.QSOs = append(s.QSOs, s.QSOs[0])
	s.QSOs[len(s.QSOs)-1].Duplicate = true
	score := s.Score()
	if score.ValidQSOs != 3 || score.Points != 7 || score.Multipliers != 2 || score.Total != 14 || score.NonScoring != 1 || score.Duplicates != 1 {
		t.Fatalf("unexpected score: %+v", score)
	}
}
