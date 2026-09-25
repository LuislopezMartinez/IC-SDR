package screens

import (
	"testing"

	"go-zero/internal/sdr"
	"go-zero/simpleui"
)

func TestScanFindPeakAndDCExclusion(t *testing.T) {
	screen := &MainScreen{centerFrequencyHz: 100_000_000, spanHz: 1_000_000, stats: sdr.Stats{SampleRate: 1_000_000}}
	panel := &ScanPanel{screen: screen}
	spectrum := make([]float32, 100)
	for i := range spectrum {
		spectrum[i] = -100
	}
	spectrum[50], spectrum[60] = -5, -20
	peak := panel.findPeak(spectrum, 99_500_000, 100_500_000, 1, 0)
	if peak == nil || peak.frequencyHz != 100_100_000 {
		t.Fatalf("peak = %#v, want off-center signal", peak)
	}
}

func TestScannerMemoryRecallLeavesOpenBandSelectorUntouched(t *testing.T) {
	selector := NewBandSelector("HAM", "20 m", nil)
	selector.Open()
	screen := &MainScreen{
		bandCategory: "HAM", bandName: "20 m", centerFrequencyHz: 14_200_000,
		spanHz:       1_000_000,
		mode:         simpleui.NewDropdown("scanTestMode", 0, 0, 100, 30, "", []string{"NFM"}, 12),
		bandSelector: selector,
	}
	panel := &MemoryPanel{screen: screen}
	panel.recallForScanner(MemoryEntry{Name: "PMR", FrequencyHz: 446_100_000, Mode: "NFM"})

	if !selector.OverlayOpen() {
		t.Fatal("scanner memory recall closed the band selector")
	}
	if selector.category != "HAM" || selector.selectedCategory != "HAM" || selector.selectedName != "20 m" {
		t.Fatalf("scanner changed modal selection: category=%q selected=%q/%q", selector.category, selector.selectedCategory, selector.selectedName)
	}
	if screen.frequencyHz != 446_100_000 {
		t.Fatalf("scanner did not tune the detected memory: %d", screen.frequencyHz)
	}
}

func TestMemoryPassbandsRespectSideband(t *testing.T) {
	usbLow, usbHigh := memoryPassband(MemoryEntry{FrequencyHz: 10_000, FilterBandwidthHz: 2400, Mode: "USB"})
	lsbLow, lsbHigh := memoryPassband(MemoryEntry{FrequencyHz: 10_000, FilterBandwidthHz: 2400, Mode: "LSB"})
	if usbLow != 10_000 || usbHigh != 12_400 || lsbLow != 7_600 || lsbHigh != 10_000 {
		t.Fatalf("USB %d..%d LSB %d..%d", usbLow, usbHigh, lsbLow, lsbHigh)
	}
}

func TestCenterToMemoryChoosesNearestOverlappingMemory(t *testing.T) {
	screen := &MainScreen{}
	screen.memoryPanel = &MemoryPanel{memories: []MemoryEntry{
		{Name: "A", FrequencyHz: 100_000, FilterBandwidthHz: 20_000, ScanEnabled: true},
		{Name: "B", FrequencyHz: 106_000, FilterBandwidthHz: 20_000, ScanEnabled: true},
	}}
	panel := &ScanPanel{screen: screen, centerToMemory: true}
	if got := panel.matchingMemory(104_000); got != 1 {
		t.Fatalf("matchingMemory() = %d, want nearest memory 1", got)
	}
}

func TestScannerThresholdAlwaysFollowsSquelch(t *testing.T) {
	screen := &MainScreen{squelchThreshold: -47}
	panel := &ScanPanel{screen: screen}
	if got := panel.thresholdDB(); got != -47 {
		t.Fatalf("thresholdDB() = %v, want -47", got)
	}
	screen.squelchThreshold = -31
	if got := panel.thresholdDB(); got != -31 {
		t.Fatalf("thresholdDB() after SQL change = %v, want -31", got)
	}
}

func TestMemoryViewStateControlsMarkerVisibility(t *testing.T) {
	screen := &MainScreen{}
	panel := &MemoryPanel{screen: screen, markersVisible: true}
	screen.memoryPanel = panel
	screen.setMemoryView(false)
	if panel.markersVisible || screen.memoryViewEnabled {
		t.Fatal("MEM VIEW off did not hide FFT memory markers")
	}
	screen.setMemoryView(true)
	if !panel.markersVisible || !screen.memoryViewEnabled {
		t.Fatal("MEM VIEW on did not restore FFT memory markers")
	}
}

func TestScannerKeepsValidSegmentOutsideVisibleFFT(t *testing.T) {
	screen := &MainScreen{centerFrequencyHz: 434_000_000, spanHz: 100_000}
	panel := &ScanPanel{screen: screen, minimumHz: 433_800_000, maximumHz: 434_200_000}
	panel.Enter()
	if panel.minimumHz != 433_800_000 || panel.maximumHz != 434_200_000 {
		t.Fatalf("Enter changed valid off-screen segment to %d..%d", panel.minimumHz, panel.maximumHz)
	}
}

func TestScanLimitSide(t *testing.T) {
	const low, high = int64(433_950_000), int64(434_050_000)
	for _, test := range []struct {
		hz   int64
		want int
	}{{433_900_000, scanLimitLeft}, {low, scanLimitInside}, {434_000_000, scanLimitInside}, {high, scanLimitInside}, {434_100_000, scanLimitRight}} {
		if got := scanLimitSide(test.hz, low, high); got != test.want {
			t.Errorf("scanLimitSide(%d) = %d, want %d", test.hz, got, test.want)
		}
	}
}

func TestScanFitSpanUsesSmallestSupportedSpanWithMargin(t *testing.T) {
	for _, test := range []struct {
		minimum, maximum int64
		want             int64
	}{{0, 40_000, 50_000}, {0, 80_000, 100_000}, {0, 200_000, 250_000}, {0, 400_000, 500_000}, {0, 800_000, 1_000_000}, {0, 1_500_000, 2_000_000}, {0, 3_000_000, 2_000_000}} {
		if got := scanFitSpan(test.minimum, test.maximum); got != test.want {
			t.Errorf("scanFitSpan(%d, %d) = %d, want %d", test.minimum, test.maximum, got, test.want)
		}
	}
}
