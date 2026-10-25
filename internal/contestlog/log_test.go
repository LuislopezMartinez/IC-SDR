package contestlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOceaniaSessionPersistsDuplicatesAndExportsCabrillo(t *testing.T) {
	now := time.Date(2026, 10, 3, 6, 7, 0, 0, time.FixedZone("CEST", 2*60*60))
	s, err := NewOceaniaDXSSB(" ea1abc ", "", "LOW", now)
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.Add(Entry{Time: now, FrequencyHz: 14_225_000, Mode: "USB", WorkedCall: "vk2xyz", RSTSent: "59", RSTReceived: "59", SerialReceived: 17})
	if err != nil {
		t.Fatal(err)
	}
	if first.SerialSent != 1 || first.Band != "20M" || first.Duplicate {
		t.Fatalf("unexpected first QSO: %+v", first)
	}
	duplicate, err := s.Add(Entry{Time: now.Add(time.Minute), FrequencyHz: 14_230_000, WorkedCall: "VK2XYZ", SerialReceived: 18})
	if err != nil {
		t.Fatal(err)
	}
	if !duplicate.Duplicate || duplicate.SerialSent != 2 {
		t.Fatalf("duplicate not retained/marked: %+v", duplicate)
	}

	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "session.json")
	if err := Save(jsonPath, s); err != nil {
		t.Fatal(err)
	}
	if err := Save(jsonPath, s); err != nil {
		t.Fatalf("replacing an existing session: %v", err)
	}
	loaded, err := Load(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.QSOs) != 2 || loaded.NextSerial != 3 {
		t.Fatalf("bad round trip: %+v", loaded)
	}
	loaded.Name, loaded.Email = "Luis Radio", "luis@example.com"
	loaded.Address, loaded.City, loaded.Country, loaded.Club = "Calle Radio 1", "Madrid", "Spain", "EA Contest Club"

	logPath := filepath.Join(dir, "ea1abc.log")
	if err := ExportCabrillo(logPath, loaded); err != nil {
		t.Fatal(err)
	}
	if err := ExportCabrillo(logPath, loaded); err != nil {
		t.Fatalf("replacing an existing Cabrillo file: %v", err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"CONTEST: OCEANIA-DX-SSB", "NAME: Luis Radio", "EMAIL: luis@example.com",
		"ADDRESS: Calle Radio 1", "ADDRESS-CITY: Madrid", "ADDRESS-COUNTRY: Spain", "CLUB: EA Contest Club",
		"CLAIMED-SCORE:", "QSO: 14225 PH 2026-10-03 0407 EA1ABC", "VK2XYZ", "END-OF-LOG:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("Cabrillo lacks %q:\n%s", want, text)
		}
	}
	if !strings.Contains(text, " 59    017") {
		t.Fatalf("received serial is not zero padded to Cabrillo width:\n%s", text)
	}
}

func TestBandForFrequencyRejectsWARC(t *testing.T) {
	if got := BandForFrequency(18_100_000); got != "" {
		t.Fatalf("WARC band accepted as %s", got)
	}
	if got := BandForFrequency(7_100_000); got != "40M" {
		t.Fatalf("got %s", got)
	}
}

func TestUpdateAndDeleteRecomputeDuplicatesWithoutReusingSentSerial(t *testing.T) {
	s, _ := NewOceaniaDXSSB("EA1ABC", "", "LOW", time.Now())
	base := Entry{Time: time.Now(), FrequencyHz: 14_200_000, WorkedCall: "VK2AAA", SerialReceived: 1}
	_, _ = s.Add(base)
	base.SerialReceived = 2
	_, _ = s.Add(base)
	if !s.QSOs[1].Duplicate {
		t.Fatal("second QSO should start as duplicate")
	}

	base.WorkedCall = "ZL1BBB"
	base.SerialReceived = 3
	updated, err := s.Update(0, base)
	if err != nil {
		t.Fatal(err)
	}
	if updated.SerialSent != 1 || s.QSOs[1].Duplicate {
		t.Fatalf("bad update: %+v / %+v", updated, s.QSOs[1])
	}

	removed, err := s.Delete(1)
	if err != nil {
		t.Fatal(err)
	}
	if removed.SerialSent != 2 || len(s.QSOs) != 1 || s.NextSerial != 3 {
		t.Fatalf("bad delete state: %+v / %+v", removed, s)
	}
}

func TestConfigAndOperationalWarnings(t *testing.T) {
	during := time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC)
	s, err := NewOceaniaDXSSBWithConfig("EA1ABC", "EA1XYZ", Config{Power: "HIGH", OperatorCategory: "MULTI-OP", Assisted: "ASSISTED", Band: "20M", Transmitter: "TWO"}, during)
	if err != nil {
		t.Fatal(err)
	}
	entry := Entry{Time: during, FrequencyHz: 14_200_000, Mode: "USB", WorkedCall: "VK2AAA", SerialReceived: 1}
	if warnings := s.Warnings(entry); len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if _, err := s.Add(entry); err != nil {
		t.Fatal(err)
	}
	entry.Mode = "FM"
	entry.Time = during.Add(-48 * time.Hour)
	warnings := s.Warnings(entry)
	for _, want := range []Warning{WarningDuplicate, WarningMode, WarningTime} {
		found := false
		for _, got := range warnings {
			if got == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing warning %q in %v", want, warnings)
		}
	}

	path := filepath.Join(t.TempDir(), "configured.log")
	if err := ExportCabrillo(path, s); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	text := string(data)
	for _, want := range []string{"CATEGORY-OPERATOR: MULTI-OP", "CATEGORY-ASSISTED: ASSISTED", "CATEGORY-BAND: 20M", "CATEGORY-POWER: HIGH", "CATEGORY-TRANSMITTER: TWO"} {
		if !strings.Contains(text, want) {
			t.Fatalf("configured Cabrillo lacks %q:\n%s", want, text)
		}
	}
}

func TestGenericProfileStoresLoadsAndExportsWithoutOceaniaRules(t *testing.T) {
	outsideOceaniaPeriod := time.Date(2027, 1, 15, 12, 0, 0, 0, time.UTC)
	s, err := NewSession(GenericSSB, "EA1ABC", "", Config{Power: "LOW"}, outsideOceaniaPeriod)
	if err != nil {
		t.Fatal(err)
	}
	qso, err := s.Add(Entry{Time: outsideOceaniaPeriod, FrequencyHz: 14_200_000, Mode: "USB", WorkedCall: "F1ABC", SerialReceived: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(qso.Warnings) != 0 {
		t.Fatalf("generic profile inherited Oceania warnings: %v", qso.Warnings)
	}
	if score := s.Score(); score.ValidQSOs != 1 || score.Total != 1 {
		t.Fatalf("generic score = %+v", score)
	}
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "generic.json")
	if err := Save(jsonPath, s); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(jsonPath)
	if err != nil || loaded.Contest != GenericSSB {
		t.Fatalf("generic round trip: %+v, %v", loaded, err)
	}
	logPath := filepath.Join(dir, "generic.log")
	if err := ExportCabrillo(logPath, loaded); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(logPath)
	if !strings.Contains(string(data), "CONTEST: "+GenericSSB) {
		t.Fatalf("generic Cabrillo uses wrong contest:\n%s", data)
	}
}

func TestCQWWSingleOperatorExportsOfficialCabrilloFields(t *testing.T) {
	when := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	s, err := NewSession(CQWWSSB, "EA1ABC", "EA1ABC", Config{Power: "LOW", OperatorCategory: "MULTI-OP", Transmitter: "TWO", SentValue: "14", Location: "DX"}, when)
	if err != nil {
		t.Fatal(err)
	}
	if s.CategoryOperator != "SINGLE-OP" || s.CategoryTransmitter != "ONE" {
		t.Fatalf("CQ WW profile did not enforce single operator: %+v", s)
	}
	if _, err := s.Add(Entry{Time: when, FrequencyHz: 14_225_000, Mode: "USB", WorkedCall: "K1ABC", RSTSent: "59", RSTReceived: "59", ExchangeReceived: "05"}); err != nil {
		t.Fatal(err)
	}
	s.Name, s.Email, s.Address, s.Country = "Luis", "luis@example.com", "Calle Radio 1", "Spain"
	if issues := s.ValidateSubmission(); len(issues) != 0 {
		t.Fatalf("valid CQ WW log has issues: %+v", issues)
	}
	path := filepath.Join(t.TempDir(), "cqww.log")
	if err := ExportCabrillo(path, s); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	text := string(data)
	for _, want := range []string{"CONTEST: CQ-WW-SSB", "LOCATION: DX", "CATEGORY-OPERATOR: SINGLE-OP", "CATEGORY-TRANSMITTER: ONE", " 59     14", " 59     05"} {
		if !strings.Contains(text, want) {
			t.Fatalf("CQ WW Cabrillo lacks %q:\n%s", want, text)
		}
	}
}

func TestCQWWExportsADIFWithContestExchangeAndZones(t *testing.T) {
	when := time.Date(2026, 10, 24, 12, 34, 56, 0, time.UTC)
	s, err := NewSession(CQWWSSB, "EA3IWW", "EA3IWW", Config{Power: "LOW", SentValue: "14", Location: "DX"}, when)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add(Entry{Time: when, FrequencyHz: 28_435_000, Mode: "USB", WorkedCall: "7Z7A",
		RSTSent: "59", RSTReceived: "59", ExchangeReceived: "21"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "ea3iww.adi")
	if err := ExportADIF(path, s); err != nil {
		t.Fatal(err)
	}
	if err := ExportADIF(path, s); err != nil {
		t.Fatalf("replacing an existing ADIF file: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"<ADIF_VER:5>3.1.7", "<PROGRAMID:6>IC-SDR", "<EOH>", "<CALL:4>7Z7A",
		"<STATION_CALLSIGN:6>EA3IWW", "<QSO_DATE:8>20261024", "<TIME_ON:6>123456",
		"<BAND:3>10m", "<FREQ:9>28.435000", "<MODE:3>SSB", "<CONTEST_ID:9>CQ-WW-SSB",
		"<STX_STRING:2>14", "<SRX_STRING:2>21", "<CQZ:2>21", "<MY_CQ_ZONE:2>14", "<EOR>",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("ADIF lacks %q:\n%s", want, text)
		}
	}
}
