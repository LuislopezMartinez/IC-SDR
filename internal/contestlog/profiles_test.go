package contestlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCustomProfileRoundTripAndSessionSnapshot(t *testing.T) {
	dir := t.TempDir()
	profile := Profile{ID: "LOCAL-TEST", Name: "Local Test", CabrilloName: "LOCAL-TEST", Mode: "CW",
		Start: time.Date(2027, 1, 2, 8, 0, 0, 0, time.UTC), End: time.Date(2027, 1, 2, 12, 0, 0, 0, time.UTC),
		DuplicateRule: "CONTEST", PointsPerQSO: 3}
	if err := SaveProfile(dir, profile); err != nil {
		t.Fatal(err)
	}
	profiles, err := LoadProfiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	loaded := profiles[len(profiles)-1]
	if loaded.ID != profile.ID || loaded.BuiltIn || loaded.PointsPerQSO != 3 {
		t.Fatalf("custom profile round trip: %+v", loaded)
	}
	session, err := NewSessionFromProfile(loaded, "EA1ABC", "", Config{Power: "LOW"}, profile.Start)
	if err != nil {
		t.Fatal(err)
	}
	if session.ContestMode != "CW" || session.DuplicateRule != "CONTEST" || session.PointsPerQSO != 3 {
		t.Fatalf("profile rules were not copied to session: %+v", session)
	}
	path := filepath.Join(dir, "session.json")
	if err := Save(path, session); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("custom session cannot be loaded independently: %v", err)
	}
}

func TestCustomTextExchangeIsPreservedInCabrillo(t *testing.T) {
	profile := Profile{ID: "LOCAL-SECTION", Name: "Local Section", CabrilloName: "LOCAL-SECTION", Mode: "SSB",
		DuplicateRule: "CONTEST", PointsPerQSO: 2, SentExchange: "FIXED", SentValue: "M",
		ReceivedExchange: "SECTION"}
	session, err := NewSessionFromProfile(profile, "EA1ABC", "", Config{Power: "LOW"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	qso, err := session.Add(Entry{Time: time.Now(), FrequencyHz: 14_200_000, Mode: "USB", WorkedCall: "EA2XYZ",
		RSTSent: "59", RSTReceived: "59", ExchangeReceived: "BI"})
	if err != nil {
		t.Fatal(err)
	}
	if qso.SentExchangeText() != "M" || qso.ReceivedExchangeText() != "BI" {
		t.Fatalf("exchange not preserved: %+v", qso)
	}
	path := filepath.Join(t.TempDir(), "section.log")
	if err := ExportCabrillo(path, session); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), " 59      M") || !strings.Contains(string(data), " 59     BI") {
		t.Fatalf("Cabrillo lacks custom exchanges:\n%s", data)
	}
}

func TestTemplateAllowedBandsProduceOperationalWarning(t *testing.T) {
	profile := Profile{ID: "ONLY-20M", Name: "Only 20m", CabrilloName: "ONLY-20M", Mode: "SSB",
		DuplicateRule: "BAND", PointsPerQSO: 1, SentExchange: "SERIAL", ReceivedExchange: "SERIAL",
		AllowedBands: []string{"20M"}}
	session, err := NewSessionFromProfile(profile, "EA1ABC", "", Config{Power: "LOW"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	warnings := session.Warnings(Entry{Time: time.Now(), FrequencyHz: 7_100_000, Mode: "LSB", WorkedCall: "EA2XYZ", SerialReceived: 1})
	found := false
	for _, warning := range warnings {
		found = found || warning == WarningBand
	}
	if !found {
		t.Fatalf("disallowed band warning missing: %v", warnings)
	}
}

func TestJIDXSSBProfileIsReadyForDXSingleOperator(t *testing.T) {
	profile, ok := ProfileByID(JIDXSSB)
	if !ok || profile.CabrilloName != "JIDX-SSB" || profile.Mode != "SSB" || profile.ReceivedExchange != "NUMBER" || !profile.OfficialScoring {
		t.Fatalf("unexpected JIDX profile: %+v, %v", profile, ok)
	}
	session, err := NewSession(JIDXSSB, "EA3ABC", "EA3ABC", Config{SentValue: "14", OperatorCategory: "MULTI-OP", Transmitter: "TWO"}, profile.Start)
	if err != nil {
		t.Fatal(err)
	}
	if session.SentValue != "14" || session.CategoryOperator != "SINGLE-OP" || session.CategoryTransmitter != "ONE" {
		t.Fatalf("JIDX setup was not normalized: %+v", session)
	}
	if _, err := NewSession(JIDXSSB, "K1ABC", "", Config{SentValue: "41"}, profile.Start); err == nil {
		t.Fatal("invalid CQ zone was accepted")
	}
}
