package screens

import (
	"path/filepath"
	"testing"
)

func TestAppSettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	want := persistedAppSettings{
		Version: appSettingsVersion, BandCategory: "ISM", BandName: "PMR446", Mode: "NFM",
		FrequencyHz: 446_093_750, CenterFrequencyHz: 446_100_000,
		SpanHz: 500_000, TuningStepHz: 6_250, CenterMode: false,
		ScanCenterToMemory: boolSetting(true), ScanResume: "HOLD", ScanPolicy: "STRONGER",
		ScanDwellMs: 5000, ScanMinimumHz: 433_050_000, ScanMaximumHz: 434_750_000,
		ActiveTool: "MEMORIES", ViewMode: 2,
		SquelchEnabled: boolSetting(true), SquelchThreshold: -42, SquelchHoldMs: 120, SquelchCloseMs: 240,
		SpectrumMinimumDB: -80, SpectrumMaximumDB: -10,
		FFTAveragingMs: 150, FFTRefreshFPS: 30, FFTPeakHold: boolSetting(false), FFTPeakDecay: 6, FFTWindow: "FLAT TOP",
		WaterfallSpeed: 25, WaterfallContrast: 130, WaterfallOffsetDB: -18,
		WaterfallMinimum: -95, WaterfallMaximum: -15, WaterfallPalette: "VIRIDIS",
		MemoryViewEnabled: boolSetting(false),
		AudioNotchEnabled: boolSetting(true), AudioNotchFrequencyHz: 1250, AudioNotchWidthHz: 180, AudioNotchDepthDB: -40,
	}
	if err := writeAppSettings(path, want); err != nil {
		t.Fatal(err)
	}
	// Saving again must atomically replace the existing Windows file.
	if err := writeAppSettings(path, want); err != nil {
		t.Fatal(err)
	}
	screen := NewMainScreen(nil)
	loadAppSettings(path, screen)
	if screen.bandCategory != want.BandCategory || screen.bandName != want.BandName || screen.savedMode != want.Mode {
		t.Fatalf("restored identity = %s/%s/%s", screen.bandCategory, screen.bandName, screen.savedMode)
	}
	if screen.frequencyHz != want.FrequencyHz || screen.centerFrequencyHz != want.CenterFrequencyHz {
		t.Fatalf("restored tuning = %d/%d", screen.frequencyHz, screen.centerFrequencyHz)
	}
	if screen.spanHz != want.SpanHz || screen.tuningStepHz != want.TuningStepHz || screen.centerMode != want.CenterMode {
		t.Fatalf("restored raster = %d/%d center=%v", screen.spanHz, screen.tuningStepHz, screen.centerMode)
	}
	if screen.activeTool != "MEMORIES" || screen.viewMode != 2 || !screen.scanCenterToMemory || screen.scanResume != "HOLD" || screen.scanPolicy != "STRONGER" || screen.scanDwellMs != 5000 {
		t.Fatalf("restored scanner settings are incomplete")
	}
	if screen.activeTool != want.ActiveTool || screen.viewMode != want.ViewMode {
		t.Fatalf("restored workspace = %s/view %d", screen.activeTool, screen.viewMode)
	}
	if !screen.squelchEnabled || screen.squelchThreshold != -42 || screen.squelchHoldMs != 120 || screen.squelchCloseMs != 240 {
		t.Fatalf("restored squelch = %v/%d/%d/%d", screen.squelchEnabled, screen.squelchThreshold, screen.squelchHoldMs, screen.squelchCloseMs)
	}
	if screen.spectrumMinimumDB != -80 || screen.spectrumMaximumDB != -10 || screen.fftAveragingMs != 150 || screen.fftRefreshFPS != 30 || screen.fftPeakHold || screen.fftWindow != "FLAT TOP" {
		t.Fatalf("restored FFT settings are incomplete")
	}
	if screen.waterfallSettings.Palette != "VIRIDIS" || screen.waterfallSettings.Contrast != 130 || screen.memoryViewEnabled {
		t.Fatalf("restored visual settings are incomplete")
	}
	if !screen.audioNotchEnabled || screen.audioNotchFrequencyHz != 1250 || screen.audioNotchWidthHz != 180 || screen.audioNotchDepthDB != -40 {
		t.Fatalf("restored audio notch = %v/%d/%d/%.0f", screen.audioNotchEnabled, screen.audioNotchFrequencyHz, screen.audioNotchWidthHz, screen.audioNotchDepthDB)
	}
}

func TestInvalidAppSettingsKeepDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	invalid := persistedAppSettings{
		Version: appSettingsVersion, BandCategory: "BAD", BandName: "BAD", Mode: "BAD",
		FrequencyHz: -1, CenterFrequencyHz: -1, SpanHz: 123, TuningStepHz: 3,
	}
	if err := writeAppSettings(path, invalid); err != nil {
		t.Fatal(err)
	}
	screen := NewMainScreen(nil)
	beforeBand, beforeMode, beforeFrequency := screen.bandName, screen.savedMode, screen.frequencyHz
	loadAppSettings(path, screen)
	if screen.bandName != beforeBand || screen.savedMode != beforeMode || screen.frequencyHz != beforeFrequency {
		t.Fatalf("invalid settings replaced defaults: %#v", screen)
	}
}

func TestBandDisplayProfilesRestoreVisualSettingsIndependently(t *testing.T) {
	screen := NewMainScreen(nil)
	// The constructor restores the developer's portable settings; pin the band
	// under test so this unit test is independent of DATA/config/settings.json.
	screen.bandCategory, screen.bandName = "HAM", "20 m"
	screen.spanHz, screen.tuningStepHz = 100_000, 1_000
	screen.spectrumMinimumDB, screen.spectrumMaximumDB = -96, -16
	screen.fftAveragingMs, screen.fftRefreshFPS = 180, 25
	screen.fftPeakHold, screen.fftPeakDecay, screen.fftWindow = false, 7, "FLAT TOP"
	screen.waterfallSettings = WaterfallSettings{LinesPerSecond: 22, Contrast: 144, ColorOffsetDB: -21, MinimumDBm: -105, MaximumDBm: -20, Palette: "VIRIDIS"}
	screen.rememberBandDisplayProfile()

	screen.bandName = "40 m"
	screen.spanHz, screen.tuningStepHz = 50_000, 500
	screen.spectrumMinimumDB, screen.spectrumMaximumDB = -58, -4
	screen.waterfallSettings = WaterfallSettings{LinesPerSecond: 12, Contrast: 90, ColorOffsetDB: -8, MinimumDBm: -72, MaximumDBm: -5, Palette: "FIRE"}
	screen.rememberBandDisplayProfile()

	screen.bandName = "20 m"
	screen.spanHz, screen.tuningStepHz = 2_000_000, 100
	screen.spectrumMinimumDB, screen.spectrumMaximumDB = -37, 0
	screen.restoreBandDisplayProfile()

	if screen.spanHz != 100_000 || screen.tuningStepHz != 1_000 {
		t.Fatalf("restored 20 m raster = %d/%d", screen.spanHz, screen.tuningStepHz)
	}
	if screen.spectrumMinimumDB != -96 || screen.spectrumMaximumDB != -16 || screen.fftAveragingMs != 180 || screen.fftRefreshFPS != 25 || screen.fftPeakHold || screen.fftPeakDecay != 7 || screen.fftWindow != "FLAT TOP" {
		t.Fatalf("restored 20 m spectrum profile is incomplete")
	}
	if got := screen.waterfallSettings; got.LinesPerSecond != 22 || got.Contrast != 144 || got.ColorOffsetDB != -21 || got.MinimumDBm != -105 || got.MaximumDBm != -20 || got.Palette != "VIRIDIS" {
		t.Fatalf("restored waterfall = %#v", got)
	}
}
