package screens

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-zero/internal/bandplan"
	"go-zero/internal/radiosonde"
	"go-zero/internal/resources"
	"go-zero/internal/sdr"
)

const appSettingsVersion = 1

type persistedAppSettings struct {
	RadiosondeFamily      string                `json:"radiosondeFamily,omitempty"`
	RadiosondeFrequencyHz int64                 `json:"radiosondeFrequencyHz,omitempty"`
	Version               int                   `json:"version"`
	Theme                 string                `json:"theme,omitempty"`
	Language              string                `json:"language,omitempty"`
	ITURegion             string                `json:"ituRegion,omitempty"`
	Country               string                `json:"country,omitempty"`
	BandCategory          string                `json:"bandCategory"`
	BandName              string                `json:"bandName"`
	Mode                  string                `json:"mode"`
	FrequencyHz           int64                 `json:"frequencyHz"`
	CenterFrequencyHz     int64                 `json:"centerFrequencyHz"`
	SpanHz                int64                 `json:"spanHz"`
	TuningStepHz          int64                 `json:"tuningStepHz"`
	CenterMode            bool                  `json:"centerMode"`
	ActiveTool            string                `json:"activeTool,omitempty"`
	ViewMode              int                   `json:"viewMode,omitempty"`
	ScanCenterToMemory    *bool                 `json:"scanCenterToMemory,omitempty"`
	ScanResume            string                `json:"scanResumeMode,omitempty"`
	ScanPolicy            string                `json:"scanSignalPolicy,omitempty"`
	ScanDwellMs           int                   `json:"scanDwellMs,omitempty"`
	ScanMinimumHz         int64                 `json:"scanMinimumHz,omitempty"`
	ScanMaximumHz         int64                 `json:"scanMaximumHz,omitempty"`
	SquelchEnabled        *bool                 `json:"squelchEnabled,omitempty"`
	SquelchThreshold      int                   `json:"squelchThreshold,omitempty"`
	SquelchHoldMs         int                   `json:"squelchHoldMs,omitempty"`
	SquelchCloseMs        int                   `json:"squelchCloseMs,omitempty"`
	SpectrumMinimumDB     float32               `json:"spectrumMinimumDb,omitempty"`
	SpectrumMaximumDB     float32               `json:"spectrumMaximumDb,omitempty"`
	FFTAveragingMs        int                   `json:"fftAveragingMs,omitempty"`
	FFTRefreshFPS         int                   `json:"fftRefreshFps,omitempty"`
	FFTPeakHold           *bool                 `json:"fftPeakHold,omitempty"`
	RemoveDCSpike         *bool                 `json:"removeDCSpike,omitempty"`
	FFTPeakDecay          float32               `json:"fftPeakDecay,omitempty"`
	FFTWindow             string                `json:"fftWindow,omitempty"`
	WaterfallSpeed        int                   `json:"waterfallSpeed,omitempty"`
	WaterfallContrast     int                   `json:"waterfallContrast,omitempty"`
	WaterfallOffsetDB     int                   `json:"waterfallOffsetDb,omitempty"`
	WaterfallMinimum      float32               `json:"waterfallMinimumDb,omitempty"`
	WaterfallMaximum      float32               `json:"waterfallMaximumDb,omitempty"`
	WaterfallPalette      string                `json:"waterfallPalette,omitempty"`
	MemoryViewEnabled     *bool                 `json:"memoryViewEnabled,omitempty"`
	RecorderSkipSilence   *bool                 `json:"recorderSkipSilence,omitempty"`
	RecorderFormat        string                `json:"recorderFormat,omitempty"`
	Volume                *float32              `json:"volume,omitempty"`
	Muted                 *bool                 `json:"muted,omitempty"`
	DMRAutoCenter         *bool                 `json:"dmrAutoCenter,omitempty"`
	DMRAudioSlot          string                `json:"dmrAudioSlot,omitempty"`
	RTL433FrequencyHz     int64                 `json:"rtl433FrequencyHz,omitempty"`
	RTL433BandwidthHz     int                   `json:"rtl433BandwidthHz,omitempty"`
	APRSView              string                `json:"aprsView,omitempty"`
	SubtoneMode           string                `json:"subtoneMode,omitempty"`
	SSTVAutomatic         *bool                 `json:"sstvAutomatic,omitempty"`
	SSTVMode              string                `json:"sstvMode,omitempty"`
	SSTVCandidateModes    [4]string             `json:"sstvCandidateModes,omitempty"`
	Hardware              *sdr.HardwareSettings `json:"hardware,omitempty"`
}

func defaultAppSettingsPath() string {
	return resources.WritablePath("config", "settings.json")
}

func loadAppSettings(path string, screen *MainScreen) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var settings persistedAppSettings
	if json.Unmarshal(data, &settings) != nil || settings.Version != appSettingsVersion {
		return
	}
	settings.BandName = englishBandName(settings.BandName)
	settings.APRSView = englishAPRSView(settings.APRSView)
	if validTheme(settings.Theme) {
		screen.themeName = strings.ToUpper(settings.Theme)
	}
	if settings.Language != "" {
		screen.language = settings.Language
	}
	if settings.ITURegion != "" {
		screen.ituRegion = settings.ITURegion
	}
	if settings.Country != "" {
		screen.country = settings.Country
	}
	applyBandPlan(bandplan.Must(bandplan.Resolve(screen.ituRegion, screen.country)))
	if validBand(settings.BandCategory, settings.BandName) {
		screen.bandCategory = settings.BandCategory
		screen.bandName = settings.BandName
	}
	if validDemodMode(settings.Mode) {
		screen.savedMode = settings.Mode
	}
	if settings.FrequencyHz >= 1_000 {
		screen.frequencyHz = settings.FrequencyHz
	}
	if settings.CenterFrequencyHz >= 1_000 {
		screen.centerFrequencyHz = settings.CenterFrequencyHz
	} else {
		screen.centerFrequencyHz = screen.frequencyHz
	}
	if validSpan(settings.SpanHz) {
		screen.spanHz = settings.SpanHz
	}
	if validTuningStep(settings.TuningStepHz) {
		screen.tuningStepHz = settings.TuningStepHz
	}
	if validTool(settings.ActiveTool) {
		screen.activeTool = settings.ActiveTool
	}
	if settings.ViewMode >= 1 && settings.ViewMode <= 2 {
		screen.viewMode = settings.ViewMode
	}
	if settings.ScanCenterToMemory != nil {
		screen.scanCenterToMemory = *settings.ScanCenterToMemory
	}
	if validScanResume(settings.ScanResume) {
		screen.scanResume = settings.ScanResume
	}
	if settings.ScanPolicy == "CURRENT" || settings.ScanPolicy == "STRONGER" {
		screen.scanPolicy = settings.ScanPolicy
	}
	if settings.ScanDwellMs >= 1000 && settings.ScanDwellMs <= 10000 {
		screen.scanDwellMs = settings.ScanDwellMs
	}
	if settings.ScanMinimumHz >= 1000 && settings.ScanMaximumHz > settings.ScanMinimumHz {
		screen.scanMinimumHz, screen.scanMaximumHz = settings.ScanMinimumHz, settings.ScanMaximumHz
	}
	if settings.SquelchEnabled != nil {
		screen.squelchEnabled = *settings.SquelchEnabled
	}
	if settings.SquelchThreshold >= -140 && settings.SquelchThreshold < 0 {
		screen.squelchThreshold = settings.SquelchThreshold
	}
	if settings.SquelchHoldMs >= 0 && settings.SquelchHoldMs <= 300 {
		screen.squelchHoldMs = settings.SquelchHoldMs
	}
	if settings.SquelchCloseMs >= 20 && settings.SquelchCloseMs <= 500 {
		screen.squelchCloseMs = settings.SquelchCloseMs
	}
	if settings.SpectrumMinimumDB >= -140 && settings.SpectrumMaximumDB <= 20 && settings.SpectrumMaximumDB-settings.SpectrumMinimumDB >= 10 {
		screen.spectrumMinimumDB, screen.spectrumMaximumDB = settings.SpectrumMinimumDB, settings.SpectrumMaximumDB
	}
	if settings.FFTAveragingMs >= 10 && settings.FFTAveragingMs <= 300 {
		screen.fftAveragingMs = settings.FFTAveragingMs
	}
	if settings.FFTRefreshFPS >= 5 && settings.FFTRefreshFPS <= 60 {
		screen.fftRefreshFPS = settings.FFTRefreshFPS
	}
	if settings.FFTPeakHold != nil {
		screen.fftPeakHold = *settings.FFTPeakHold
	}
	if settings.RemoveDCSpike != nil && screen.receiver != nil {
		screen.receiver.SetRemoveDCSpike(*settings.RemoveDCSpike)
	}
	if settings.FFTPeakDecay >= 1 && settings.FFTPeakDecay <= 10 {
		screen.fftPeakDecay = settings.FFTPeakDecay
	}
	if validFFTWindow(settings.FFTWindow) {
		screen.fftWindow = settings.FFTWindow
	}
	if settings.WaterfallSpeed >= 5 && settings.WaterfallSpeed <= 60 {
		screen.waterfallSettings.LinesPerSecond = settings.WaterfallSpeed
	}
	if settings.WaterfallContrast >= 25 && settings.WaterfallContrast <= 200 {
		screen.waterfallSettings.Contrast = settings.WaterfallContrast
	}
	if settings.WaterfallOffsetDB >= -80 && settings.WaterfallOffsetDB <= 40 {
		screen.waterfallSettings.ColorOffsetDB = settings.WaterfallOffsetDB
	}
	if settings.WaterfallMinimum >= -140 && settings.WaterfallMaximum <= 20 && settings.WaterfallMaximum-settings.WaterfallMinimum >= 10 {
		screen.waterfallSettings.MinimumDBm, screen.waterfallSettings.MaximumDBm = settings.WaterfallMinimum, settings.WaterfallMaximum
	}
	if validWaterfallPalette(settings.WaterfallPalette) {
		screen.waterfallSettings.Palette = settings.WaterfallPalette
	}
	if settings.MemoryViewEnabled != nil {
		screen.memoryViewEnabled = *settings.MemoryViewEnabled
	}
	if settings.RecorderSkipSilence != nil {
		screen.recorderSkipSilence = *settings.RecorderSkipSilence
	}
	if settings.RecorderFormat == recorderFormatWAV || settings.RecorderFormat == recorderFormatMP3 {
		screen.recorderFormat = settings.RecorderFormat
	}
	if settings.Volume != nil && *settings.Volume >= 0 && *settings.Volume <= 100 {
		screen.volume = *settings.Volume
	}
	if settings.Muted != nil {
		screen.muted = *settings.Muted
	}
	if settings.DMRAutoCenter != nil {
		screen.dmrAutoCenter = *settings.DMRAutoCenter
	}
	if settings.DMRAudioSlot == "AUTO" || settings.DMRAudioSlot == "TS1" || settings.DMRAudioSlot == "TS2" {
		screen.dmrAudioSlot = settings.DMRAudioSlot
	}
	if settings.RTL433FrequencyHz >= 1_000 {
		screen.rtl433FrequencyHz = settings.RTL433FrequencyHz
	}
	if radiosonde.ValidFamily(settings.RadiosondeFamily) {
		screen.radiosondeFamily = settings.RadiosondeFamily
	}
	if settings.RadiosondeFrequencyHz >= 1_000 {
		screen.radiosondeFrequencyHz = settings.RadiosondeFrequencyHz
	}
	if settings.RTL433BandwidthHz == 250_000 || settings.RTL433BandwidthHz == 500_000 || settings.RTL433BandwidthHz == 1_000_000 || settings.RTL433BandwidthHz == 2_000_000 {
		screen.rtl433BandwidthHz = settings.RTL433BandwidthHz
	}
	if settings.APRSView == "PACKETS" || settings.APRSView == "STATIONS" || settings.APRSView == "MESSAGES" || settings.APRSView == "RADAR" || settings.APRSView == "RAW" {
		screen.aprsView = settings.APRSView
	}
	if settings.SubtoneMode == "AUTO" || settings.SubtoneMode == "CTCSS" || settings.SubtoneMode == "DCS" || settings.SubtoneMode == "OFF" {
		screen.subtoneMode = settings.SubtoneMode
	}
	if settings.SSTVAutomatic != nil {
		screen.sstvAutomatic = *settings.SSTVAutomatic
	}
	if validSSTVMode(settings.SSTVMode) {
		screen.sstvMode = settings.SSTVMode
	}
	for i, mode := range settings.SSTVCandidateModes {
		if validSSTVMode(mode) {
			screen.sstvCandidateModes[i] = mode
		}
	}
	if settings.Hardware != nil {
		hardware := *settings.Hardware
		screen.savedHardware = &hardware
	}
	screen.centerMode = settings.CenterMode
	if screen.centerMode {
		screen.centerFrequencyHz = screen.frequencyHz
	}
}

func (screen *MainScreen) markSettingsDirty() {
	screen.settingsDirty = true
	screen.nextSettingsSave = time.Now().Add(400 * time.Millisecond)
}

func (screen *MainScreen) flushSettings(force bool) {
	if !screen.settingsDirty || (!force && time.Now().Before(screen.nextSettingsSave)) {
		return
	}
	mode := screen.savedMode
	if screen.mode != nil && screen.mode.SelectedText() != "" {
		mode = screen.mode.SelectedText()
	}
	settings := persistedAppSettings{
		Version:  appSettingsVersion,
		Theme:    screen.themeName,
		Language: screen.language, ITURegion: screen.ituRegion, Country: screen.country,
		BandCategory: screen.bandCategory, BandName: screen.bandName,
		Mode: mode, FrequencyHz: screen.frequencyHz, CenterFrequencyHz: screen.centerFrequencyHz,
		SpanHz: screen.spanHz, TuningStepHz: screen.tuningStepHz, CenterMode: screen.centerMode,
		ActiveTool: screen.activeTool, ViewMode: screen.viewMode,
		SquelchEnabled: boolSetting(screen.squelchEnabled), SquelchThreshold: screen.squelchThreshold,
		SquelchHoldMs: screen.squelchHoldMs, SquelchCloseMs: screen.squelchCloseMs,
		SpectrumMinimumDB: screen.spectrumMinimumDB, SpectrumMaximumDB: screen.spectrumMaximumDB,
		FFTAveragingMs: screen.fftAveragingMs, FFTRefreshFPS: screen.fftRefreshFPS,
		FFTPeakHold: boolSetting(screen.fftPeakHold), RemoveDCSpike: boolSetting(true), FFTPeakDecay: screen.fftPeakDecay, FFTWindow: screen.fftWindow,
		WaterfallSpeed: screen.waterfallSettings.LinesPerSecond, WaterfallContrast: screen.waterfallSettings.Contrast,
		WaterfallOffsetDB: screen.waterfallSettings.ColorOffsetDB, WaterfallMinimum: screen.waterfallSettings.MinimumDBm,
		WaterfallMaximum: screen.waterfallSettings.MaximumDBm, WaterfallPalette: screen.waterfallSettings.Palette,
		MemoryViewEnabled:   boolSetting(screen.memoryViewEnabled),
		RecorderSkipSilence: boolSetting(screen.recorderSkipSilence),
		RecorderFormat:      screen.recorderFormat,
		Volume:              float32Setting(screen.volume), Muted: boolSetting(screen.muted),
		DMRAutoCenter: boolSetting(screen.dmrAutoCenter), DMRAudioSlot: screen.dmrAudioSlot,
		RTL433FrequencyHz:     screen.rtl433FrequencyHz,
		RadiosondeFamily:      screen.radiosondeFamily,
		RadiosondeFrequencyHz: screen.radiosondeFrequencyHz,
		RTL433BandwidthHz:     screen.rtl433BandwidthHz,
		APRSView:              screen.aprsView,
		SubtoneMode:           screen.subtoneMode,
		SSTVAutomatic:         boolSetting(screen.sstvAutomatic),
		SSTVMode:              screen.sstvMode,
		SSTVCandidateModes:    screen.sstvCandidateModes,
	}
	if screen.rtl433Panel != nil {
		settings.RTL433FrequencyHz = screen.rtl433Panel.targetHz
		settings.RTL433BandwidthHz = screen.rtl433Panel.bandwidthHz
	}
	if screen.scanPanel != nil {
		settings.ScanCenterToMemory = boolSetting(screen.scanPanel.centerToMemory)
		settings.ScanResume, settings.ScanPolicy = screen.scanPanel.resume, screen.scanPanel.policy
		settings.ScanDwellMs = screen.scanPanel.dwellMs
		settings.ScanMinimumHz, settings.ScanMaximumHz = screen.scanPanel.minimumHz, screen.scanPanel.maximumHz
	}
	if screen.receiver != nil {
		settings.RemoveDCSpike = boolSetting(screen.receiver.RemoveDCSpike())
		hardware := screen.receiver.HardwareSettings()
		if hardware.Available {
			settings.Hardware = &hardware
		}
	}
	if writeAppSettings(screen.settingsPath, settings) == nil {
		screen.settingsDirty = false
	}
}

func boolSetting(value bool) *bool          { return &value }
func float32Setting(value float32) *float32 { return &value }

func validFFTWindow(value string) bool {
	for _, candidate := range []string{"HANN", "BLACKMAN-HARRIS", "FLAT TOP", "RECTANGULAR"} {
		if value == candidate {
			return true
		}
	}
	return false
}

func validWaterfallPalette(value string) bool {
	for _, candidate := range []string{"BLUE", "VIRIDIS", "FIRE", "GRAY"} {
		if value == candidate {
			return true
		}
	}
	return false
}

func validSSTVMode(value string) bool {
	for _, candidate := range []string{"M1", "M2", "S1", "S2", "SDX", "R36", "R72", "PD50", "PD90", "PD120", "PD160", "PD180", "PD240", "PD290"} {
		if value == candidate {
			return true
		}
	}
	return false
}

func writeAppSettings(path string, settings persistedAppSettings) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".go-zero-settings-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if _, err = temporary.Write(data); err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(temporaryPath, path); err != nil {
		return err
	}
	removeTemporary = false
	return nil
}

func validBand(category, name string) bool {
	for _, band := range bandsByCategory[category] {
		if band.Name == name {
			return true
		}
	}
	return false
}

func validDemodMode(mode string) bool {
	for _, candidate := range []string{"AM", "NFM", "WFM", "USB", "LSB", "CW", "DMR BETA", "ADS-B", "UAT", "TETRA"} {
		if mode == candidate {
			return true
		}
	}
	return false
}

func validSpan(span int64) bool {
	for _, candidate := range []int64{50_000, 100_000, 250_000, 500_000, 1_000_000, 2_000_000} {
		if span == candidate {
			return true
		}
	}
	return false
}

func validTuningStep(step int64) bool {
	for _, candidate := range tuningStepsHz {
		if step == candidate {
			return true
		}
	}
	return false
}

func validTool(tool string) bool {
	// Accepted for migration from releases where these utilities occupied the
	// lower workspace. CreateControls moves them into the fixed sidebar.
	if tool == "SCAN" || tool == "MEMORIES" || tool == "RECORDER" {
		return true
	}
	for _, item := range toolMenuItems {
		if tool == item.id {
			return true
		}
	}
	return false
}

func validScanResume(value string) bool {
	return value == "AUTO" || value == "DELAY" || value == "HOLD"
}

func englishAPRSView(view string) string {
	switch view {
	case "PAQUETES":
		return "PACKETS"
	case "ESTACIONES":
		return "STATIONS"
	case "MENSAJES":
		return "MESSAGES"
	default:
		return view
	}
}

func englishBandName(name string) string {
	switch name {
	case "SONDAS":
		return "RADIOSONDES"
	case "FUERA DE BANDA":
		return "OUT OF BAND"
	default:
		return name
	}
}
