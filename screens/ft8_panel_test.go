package screens

import (
	"math"
	"testing"
	"time"

	"go-zero/internal/ft8"

	"github.com/biter777/countries"
)

func TestFT8ToolIsAvailableAndUsesReadableWorkspace(t *testing.T) {
	found := false
	for _, item := range toolMenuItems {
		if item.id == ft8ToolID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("FT8 is missing from the tool menu")
	}
	panel := NewFT8Panel(NewMainScreen(nil))
	if len(panel.controls) != 4 {
		t.Fatalf("FT8 controls = %d, want decoder switch, clear button, depth selector and activity button", len(panel.controls))
	}
	for _, control := range panel.controls {
		bounds := control.Bounds()
		if bounds.X < toolContentX || bounds.Y < toolY || bounds.Y+bounds.Height > toolY+toolH {
			t.Fatalf("FT8 control %q is outside tool workspace: %+v", control.ID(), bounds)
		}
	}
}

func TestFT8ClockPresentationRequiresEnoughSamples(t *testing.T) {
	text, _ := ft8ClockPresentation(ft8.Status{ClockOffset: 1.4, ClockSamples: 2})
	if text != "RELOJ  CALIBRANDO" {
		t.Fatalf("premature clock result = %q", text)
	}
	text, _ = ft8ClockPresentation(ft8.Status{ClockOffset: .2, ClockSamples: 4})
	if text != "RELOJ  CORRECTO  ·  DT +0.2 s" {
		t.Fatalf("clock result = %q", text)
	}
}

func TestFT8AudibleProgressCompensatesPlaybackDelayAndWrapsSlot(t *testing.T) {
	slot := time.Unix(1_700_000_010, 0).UTC()
	gotSlot, progress := ft8AudibleProgress(slot, .5, 150*time.Millisecond)
	if !gotSlot.Equal(slot) || math.Abs(float64(progress-.49)) > .0001 {
		t.Fatalf("audible progress = %v %.4f; want same slot .4900", gotSlot, progress)
	}
	gotSlot, progress = ft8AudibleProgress(slot, .005, 150*time.Millisecond)
	if !gotSlot.Equal(slot.Add(-15*time.Second)) || math.Abs(float64(progress-.995)) > .0001 {
		t.Fatalf("wrapped audible progress = %v %.4f", gotSlot, progress)
	}
}

func TestFT8UTCCycleProgressRecalculatesAtEverySlot(t *testing.T) {
	base := time.Unix(1_700_000_000, 0).UTC().Truncate(15 * time.Second)
	now := base.Add(14*time.Second + 900*time.Millisecond)
	slot, progress := ft8UTCCycleProgress(now, 150*time.Millisecond)
	wantSlot := base
	if !slot.Equal(wantSlot) || math.Abs(float64(progress-.9833333)) > .0001 {
		t.Fatalf("UTC progress = %v %.4f; want %v .9833", slot, progress, wantSlot)
	}

	// A delayed frame is derived afresh from UTC and therefore enters the next
	// cycle instead of retaining or incrementing the previous percentage.
	slot, progress = ft8UTCCycleProgress(now.Add(700*time.Millisecond), 150*time.Millisecond)
	if !slot.Equal(wantSlot.Add(15*time.Second)) || math.Abs(float64(progress-.03)) > .0001 {
		t.Fatalf("progress after slow frame = %v %.4f; want next slot .0300", slot, progress)
	}
}

func TestFT8MapSessionMemoryEnrichesLaterMessages(t *testing.T) {
	panel := NewFT8Panel(NewMainScreen(nil))
	panel.mapWindow = 0
	panel.mapDecodes = []ft8.Decode{
		{Slot: time.Unix(30, 0), SNR: -9, Message: "EA8ABC DL1XYZ RR73"},
		{Slot: time.Unix(15, 0), SNR: -13, Message: "EA8ABC DL1XYZ JO31"},
		{Slot: time.Unix(15, 0), SNR: -18, Message: "CQ K1ABC FN42"},
	}
	panel.mapSelected = 0
	stations := panel.ft8MapStations()
	if len(stations) != 2 {
		t.Fatalf("stations = %d, want two unique callsigns", len(stations))
	}
	selected := panel.selectedFT8MapStation()
	if selected.call != "DL1XYZ" || selected.grid != "JO31" || !selected.hasPosition || selected.messageCount != 2 {
		t.Fatalf("enriched selected station = %+v", selected)
	}
}

func TestMaidenheadCenterAndFT8StationExtraction(t *testing.T) {
	call, grid := ft8MessageStation("CQ EA8ABC IL18")
	if call != "EA8ABC" || grid != "IL18" {
		t.Fatalf("station = %q %q", call, grid)
	}
	lat, lon, ok := maidenheadCenter("IN80")
	if !ok || math.Abs(lat-40.5) > .001 || math.Abs(lon+3) > .001 {
		t.Fatalf("IN80 center = %.4f, %.4f, %v", lat, lon, ok)
	}
	call, grid = ft8MessageStation("EA8ABC DL1XYZ JO31")
	if call != "DL1XYZ" || grid != "JO31" {
		t.Fatalf("directed station = %q %q", call, grid)
	}
}

func TestInvalidLocatorNeverCreatesMapPosition(t *testing.T) {
	for _, value := range []string{"", "ZZ99", "IL1", "IL18!"} {
		if _, _, ok := maidenheadCenter(value); ok {
			t.Fatalf("invalid locator %q was accepted", value)
		}
	}
	_, grid := ft8MessageStation("EA1ABC DL1XYZ RR73")
	if grid != "" {
		t.Fatalf("RR73 was interpreted as locator %q", grid)
	}
}

func TestFT8ActivityClassifiesDirectionAndQSOPhase(t *testing.T) {
	for _, test := range []struct {
		message, from, to, kind string
	}{
		{"CQ DL1XYZ JO31", "DL1XYZ", "", "cq"},
		{"EA4ABC DL1XYZ -12", "DL1XYZ", "EA4ABC", ft8QSOReport},
		{"DL1XYZ EA4ABC R-09", "EA4ABC", "DL1XYZ", ft8QSORogerReport},
		{"EA4ABC DL1XYZ RRR", "DL1XYZ", "EA4ABC", ft8QSOConfirmed},
		{"EA4ABC DL1XYZ RR73", "DL1XYZ", "EA4ABC", ft8QSOComplete},
	} {
		event, ok := classifyFT8Activity(ft8.Decode{Message: test.message})
		if !ok || event.from != test.from || event.to != test.to || event.kind != test.kind {
			t.Fatalf("classify %q = %+v, %v", test.message, event, ok)
		}
	}
}

func TestFT8QSOReconstructionTracksFullExchangeChronologically(t *testing.T) {
	now := time.Now().UTC()
	// Runtime history is newest first; reconstruction must still advance in
	// chronological protocol order and never regress on a repeated report.
	decodes := []ft8.Decode{
		{Message: "EA4ABC DL1XYZ 73", ReceivedAt: now},
		{Message: "DL1XYZ EA4ABC RRR", ReceivedAt: now.Add(-15 * time.Second)},
		{Message: "EA4ABC DL1XYZ R-09", ReceivedAt: now.Add(-30 * time.Second)},
		{Message: "DL1XYZ EA4ABC -12", ReceivedAt: now.Add(-45 * time.Second)},
		{Message: "EA4ABC DL1XYZ JO31", ReceivedAt: now.Add(-60 * time.Second)},
	}
	qsos := reconstructFT8QSOs(decodes)
	qso := qsos["DL1XYZ|EA4ABC"]
	if qso.phase != ft8QSOComplete || qso.messages != 5 || !qso.first.Equal(now.Add(-60*time.Second)) || !qso.last.Equal(now) {
		t.Fatalf("reconstructed QSO = %+v", qso)
	}
}

func TestLatestFT8QSOForCallSelectsMostRecentConversation(t *testing.T) {
	now := time.Now().UTC()
	decodes := []ft8.Decode{
		{Message: "EA4ABC DL1XYZ RR73", ReceivedAt: now.Add(-time.Minute)},
		{Message: "EA4ABC F4AAA -12", ReceivedAt: now},
	}
	qso, ok := latestFT8QSOForCall(decodes, "EA4ABC")
	if !ok || qso.callA != "EA4ABC" || qso.callB != "F4AAA" || qso.phase != ft8QSOReport {
		t.Fatalf("latest QSO = %+v, %v", qso, ok)
	}
}

func TestFT8QSOStatsGroupsBothDirections(t *testing.T) {
	now := time.Now()
	decodes := []ft8.Decode{
		{Message: "EA4ABC DL1XYZ -12", ReceivedAt: now.Add(-30 * time.Second)},
		{Message: "DL1XYZ EA4ABC R-09", ReceivedAt: now.Add(-20 * time.Second)},
		{Message: "EA4ABC DL1XYZ RR73", ReceivedAt: now.Add(-10 * time.Second)},
		{Message: "F4AAA G1BBB -18", ReceivedAt: now.Add(-15 * time.Second)},
	}
	stats := calculateFT8QSOStats(decodes, now)
	if stats.total != 2 || stats.completed != 1 || stats.active != 1 {
		t.Fatalf("QSO stats = %+v", stats)
	}
}

func TestFT8CountryResolutionUsesBundledCTY(t *testing.T) {
	for call, want := range map[string]string{
		"EA4ABC":     "España",
		"EA8ABC":     "Canary Islands",
		"DL1XYZ/P":   "Alemania",
		"PY2BRU":     "Brasil",
		"LU6ETB":     "Argentina",
		"JA1ABC":     "Japón",
		"EA8/DL1XYZ": "Canary Islands",
	} {
		if got := ft8CallRegion(call); got != want {
			t.Errorf("ft8CallRegion(%q) = %q; want %q", call, got, want)
		}
	}
}

func TestFT8CQZoneUsesBundledCTY(t *testing.T) {
	for call, want := range map[string]int{
		"EA4ABC": 14,
		"EA8ABC": 33,
		"DL1XYZ": 14,
		"EX6JRO": 17,
	} {
		got, ok := ft8CallCQZone(call)
		if !ok || got != want {
			t.Errorf("ft8CallCQZone(%q) = %d, %v; want %d, true", call, got, ok, want)
		}
	}
	if zone, ok := ft8CallCQZone(""); ok || zone != 0 {
		t.Fatalf("unknown call returned CQ zone %d, %v", zone, ok)
	}
}

func TestFT8RegionFilterKeepsStationsAndRelatedMessages(t *testing.T) {
	panel := NewFT8Panel(NewMainScreen(nil))
	panel.mapWindow = 0
	panel.mapRegionFilter = "España"
	panel.mapDecodes = []ft8.Decode{
		{Message: "CQ EA4ABC IN80", SNR: -10},
		{Message: "EA4ABC DL1XYZ -12", SNR: -12},
		{Message: "CQ PY2BRU GG66", SNR: -5},
	}
	stations := panel.visibleFT8MapStations()
	if len(stations) != 1 || stations[0].call != "EA4ABC" {
		t.Fatalf("filtered stations = %+v; want EA4ABC", stations)
	}
	decodes := panel.filterFT8DecodesByRegion(panel.activityDecodes())
	if len(decodes) != 2 {
		t.Fatalf("filtered decodes = %d; want 2", len(decodes))
	}
}

func TestFT8RegionFlagCountryAliases(t *testing.T) {
	for region, want := range map[string]countries.CountryCode{
		"Lithuania":        countries.LT,
		"Rusia europea":    countries.RU,
		"Northern Ireland": countries.GB,
		"Canary Islands":   countries.ES,
		"Brasil":           countries.BR,
		"The Gambia":       countries.GM,
		"European Russia":  countries.RU,
		"Kosovo":           countries.XK,
	} {
		if got := ft8RegionCountryCode(region); got != want {
			t.Errorf("ft8RegionCountryCode(%q) = %s; want %s", region, got.Alpha2(), want.Alpha2())
		}
	}
	if got := ft8RegionCountryCode("Otras regiones"); got != countries.Unknown {
		t.Fatalf("generic region resolved to %s", got.Alpha2())
	}
	for region, alpha2 := range map[string]string{
		"Agalega & St. Brandon": "MU", "Bouvet": "BV", "Peter 1 Island": "NO",
		"Dem. Rep. of the Congo": "CD", "Trinidad & Tobago": "TT",
		"San Felix & San Ambrosio": "CL", "Juan Fernandez Islands": "CL",
		"St. Barthelemy": "BL", "Reunion Island": "RE", "St. Martin": "MF",
		"Juan de Nova, Europa": "FR", "Galapagos Islands": "EC",
		"San Andres & Providencia": "CO", "Republic of Korea": "KR",
		"African Italy": "IT", "Sicily": "IT", "Minami Torishima": "JP", "Ogasawara": "JP",
		"Bear Island": "NO", "Saba & St. Eustatius": "BQ", "St. Peter & St. Paul": "BR",
		"Trindade & Martim Vaz": "BR", "Asiatic Turkey": "TR", "European Turkey": "TR",
		"Heard Island": "HM", "Revillagigedo": "MX",
	} {
		if got := ft8RegionCountryCode(region).Alpha2(); got != alpha2 {
			t.Errorf("ft8RegionCountryCode(%q) = %q; want %q", region, got, alpha2)
		}
	}
}
