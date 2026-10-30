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

func TestCQWWScoreUsesContinentsCountriesAndZones(t *testing.T) {
	when := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	s, err := NewSession(CQWWSSB, "EA1ABC", "", Config{Power: "LOW", SentValue: "14"}, when)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []Entry{
		{Time: when, FrequencyHz: 14_200_000, Mode: "USB", WorkedCall: "K1ABC", ExchangeReceived: "05"},
		{Time: when.Add(time.Minute), FrequencyHz: 14_210_000, Mode: "USB", WorkedCall: "DL1ABC", ExchangeReceived: "14"},
		{Time: when.Add(2 * time.Minute), FrequencyHz: 14_220_000, Mode: "USB", WorkedCall: "EA2ABC", ExchangeReceived: "14"},
	} {
		if _, err := s.Add(entry); err != nil {
			t.Fatal(err)
		}
	}
	score := s.Score()
	// 3 intercontinental + 1 same-continent + 0 same-country points.
	// Two zones and three countries on 20m produce five multipliers.
	if score.ValidQSOs != 3 || score.Points != 4 || score.Multipliers != 5 || score.Total != 20 {
		t.Fatalf("unexpected CQ WW score: %+v", score)
	}
}

func TestCQZoneForCallUsesCTYPrefixesAndPortableLocations(t *testing.T) {
	for call, want := range map[string]int{"EA1ABC": 14, "EA8ABC": 33, "DL1ABC/EA8": 33, "K1ABC": 5} {
		if got, ok := CQZoneForCall(call); !ok || got != want {
			t.Errorf("CQZoneForCall(%q) = %d, %v; want %d", call, got, ok, want)
		}
	}
}

func TestLookupCallIdentityReturnsEntityMetadata(t *testing.T) {
	identity, ok := LookupCallIdentity("EA8ABC")
	if !ok || identity.Entity != "Canary Islands" || identity.Continent != "AF" || identity.CQZone != 33 || identity.Latitude == 0 || identity.Longitude == 0 {
		t.Fatalf("unexpected identity: %+v, %v", identity, ok)
	}
}

func TestJIDXScoreUsesBandPointsAndPrefecturesPerBand(t *testing.T) {
	when := time.Date(2026, 11, 14, 8, 0, 0, 0, time.UTC)
	s, err := NewSession(JIDXSSB, "EA3ABC", "", Config{Power: "LOW", SentValue: "14"}, when)
	if err != nil {
		t.Fatal(err)
	}
	entries := []Entry{
		{Time: when, FrequencyHz: 14_200_000, Mode: "USB", WorkedCall: "JA1AAA", ExchangeReceived: "10"},
		{Time: when.Add(time.Minute), FrequencyHz: 14_210_000, Mode: "USB", WorkedCall: "JA2BBB", ExchangeReceived: "10"},
		{Time: when.Add(2 * time.Minute), FrequencyHz: 3_700_000, Mode: "LSB", WorkedCall: "JA1AAA", ExchangeReceived: "10"},
		{Time: when.Add(3 * time.Minute), FrequencyHz: 28_400_000, Mode: "USB", WorkedCall: "DL1AAA", ExchangeReceived: "14"},
	}
	for _, entry := range entries {
		if _, err := s.Add(entry); err != nil {
			t.Fatal(err)
		}
	}
	score := s.Score()
	// JA QSOs: 1 + 1 points on 20m and 2 on 80m. Prefecture 10 is a
	// separate multiplier on each band. The non-JA QSO is non-scoring.
	if score.ValidQSOs != 3 || score.Points != 4 || score.Multipliers != 2 || score.Total != 8 || score.NonScoring != 1 {
		t.Fatalf("unexpected JIDX score: %+v", score)
	}
}

func TestJIDXRejectsInvalidPrefecture(t *testing.T) {
	when := time.Date(2026, 11, 14, 8, 0, 0, 0, time.UTC)
	s, _ := NewSession(JIDXSSB, "EA3ABC", "", Config{SentValue: "14"}, when)
	if _, err := s.Add(Entry{Time: when, FrequencyHz: 14_200_000, Mode: "USB", WorkedCall: "JA1AAA", ExchangeReceived: "51"}); err == nil {
		t.Fatal("prefecture 51 was accepted")
	}
}
