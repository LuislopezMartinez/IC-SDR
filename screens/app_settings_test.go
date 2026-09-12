package screens

import (
	"path/filepath"
	"testing"
)

func TestAppSettingsRoundTrip(t *testing.T) {
	restoreDefaultLocaleForTests()
	path := filepath.Join(t.TempDir(), "settings.json")
	want := persistedAppSettings{
		Version: appSettingsVersion, Language: "en", ITURegion: "itu-r1", Country: "auto",
		BandCategory: "ISM", BandName: "PMR446", Mode: "NFM",
		FrequencyHz: 446_093_750, CenterFrequencyHz: 446_100_000,
		SpanHz: 500_000, TuningStepHz: 6_250, CenterMode: false,
		ScanCenterToMemory: boolSetting(true), ScanResume: "HOLD", ScanPolicy: "STRONGER",
		ScanDwellMs: 5000, ScanMinimumHz: 433_050_000, ScanMaximumHz: 434_750_000,
		ActiveTool: "MEMORIES", ViewMode: 3,
		SquelchEnabled: boolSetting(true), SquelchThreshold: -42, SquelchHoldMs: 120, SquelchCloseMs: 240,
		SpectrumMinimumDB: -80, SpectrumMaximumDB: -10,
		FFTAveragingMs: 150, FFTRefreshFPS: 30, FFTPeakHold: boolSetting(false), FFTPeakDecay: 6, FFTWindow: "FLAT TOP",
		WaterfallSpeed: 25, WaterfallContrast: 130, WaterfallOffsetDB: -18,
		WaterfallMinimum: -95, WaterfallMaximum: -15, WaterfallPalette: "VIRIDIS",
		MemoryViewEnabled: boolSetting(false),
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
	screen.applyLocaleAndRegion()
	if screen.language != "en" || screen.ituRegion != "itu-r1" || screen.country != "auto" {
		t.Fatalf("restored locale = %s/%s/%s", screen.language, screen.ituRegion, screen.country)
	}
	if screen.bandCategory != want.BandCategory || screen.bandName != want.BandName || screen.savedMode != want.Mode {
		t.Fatalf("restored identity = %s/%s/%s", screen.bandCategory, screen.bandName, screen.savedMode)
	}
	if screen.frequencyHz != want.FrequencyHz || screen.centerFrequencyHz != want.CenterFrequencyHz {
		t.Fatalf("restored tuning = %d/%d", screen.frequencyHz, screen.centerFrequencyHz)
	}
	if screen.spanHz != want.SpanHz || screen.tuningStepHz != want.TuningStepHz || screen.centerMode != want.CenterMode {
		t.Fatalf("restored raster = %d/%d center=%v", screen.spanHz, screen.tuningStepHz, screen.centerMode)
	}
	if screen.activeTool != "MEMORIES" || screen.viewMode != 3 || !screen.scanCenterToMemory || screen.scanResume != "HOLD" || screen.scanPolicy != "STRONGER" || screen.scanDwellMs != 5000 {
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
