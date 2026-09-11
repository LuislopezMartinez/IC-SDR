package screens

import (
	"fmt"
	"math"
	"time"

	"go-zero/internal/sdr"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	designWidth  = float32(1600)
	designHeight = float32(900)
	waterfallY   = float32(450)
	waterfallH   = float32(170)
	toolY        = float32(630)
	toolH        = float32(196)
	// New panels must not introduce text below this readable baseline.
	uiMinimumFontSize = int32(13)
	uiControlFontSize = int32(14)
	legacyToolX       = float32(24)
	legacyToolWidth   = float32(1552)
	toolContentX      = utilitiesRight + 10
	toolContentRight  = float32(1592)
)

const toolContentScaleX = (toolContentRight - toolContentX) / legacyToolWidth

// compactToolControls maps the controls created for the former full-width
// workspace into the area remaining beside the fixed utilities sidebar. Only
// horizontal geometry changes, so text and controls retain their readable height.
func compactToolControls(elements []simpleui.Element) {
	for _, element := range elements {
		bounds := element.Bounds()
		if bounds.Y < toolY || bounds.Y >= toolY+toolH {
			continue
		}
		bounds.X = toolContentX + (bounds.X-legacyToolX)*toolContentScaleX
		bounds.Width *= toolContentScaleX
		element.SetBounds(bounds)
	}
}

func drawCompactedTool(draw func()) {
	rl.PushMatrix()
	rl.Translatef(toolContentX, 0, 0)
	rl.Scalef(toolContentScaleX, 1, 1)
	rl.Translatef(-legacyToolX, 0, 0)
	simpleui.SetHorizontalDrawScale(toolContentScaleX)
	draw()
	simpleui.SetHorizontalDrawScale(1)
	rl.PopMatrix()
}

type uiPalette struct {
	background, panel, panelAlt, border, grid   rl.Color
	cyan, blue, green, orange, red, text, muted rl.Color
}

var darkPalette = uiPalette{
	background: rl.Color{R: 7, G: 10, B: 15, A: 255},
	panel:      rl.Color{R: 10, G: 14, B: 19, A: 255},
	panelAlt:   rl.Color{R: 14, G: 19, B: 26, A: 255},
	border:     rl.Color{R: 48, G: 64, B: 82, A: 255},
	grid:       rl.Color{R: 31, G: 43, B: 56, A: 255},
	cyan:       rl.Color{R: 45, G: 195, B: 225, A: 255},
	blue:       rl.Color{R: 35, G: 125, B: 205, A: 255},
	green:      rl.Color{R: 70, G: 205, B: 135, A: 255},
	orange:     rl.Color{R: 238, G: 156, B: 24, A: 255},
	red:        rl.Color{R: 235, G: 70, B: 75, A: 255},
	text:       rl.Color{R: 225, G: 232, B: 238, A: 255},
	muted:      rl.Color{R: 112, G: 130, B: 145, A: 255},
}
var colors = darkPalette

// MainScreen is the visual shell of IC-SDR. DSP and hardware are deliberately
// kept out of this first porting stage.
type MainScreen struct {
	receiver               *sdr.Receiver
	mode                   *simpleui.Dropdown
	filter, band           *simpleui.Button
	step                   *simpleui.Button
	viewButton             *simpleui.Button
	themeButton            *simpleui.Button
	themeName              string
	filterSelector         *FilterSelector
	bandSelector           *BandSelector
	stepSelector           *StepSelector
	bandCategory           string
	bandName               string
	frequencyHz            int64 // tuned VFO frequency
	centerFrequencyHz      int64 // center of the SDR IQ capture
	spanHz                 int64
	tuningStepHz           int64
	frequencyDigitExponent int
	centerMode             bool
	draggingSpectrum       bool
	dragStartX             float32
	dragStartCenterHz      int64
	dragStartTunedHz       int64
	nextDragRetune         float64
	vfoModeSwitch          *simpleui.Switch
	memViewSwitch          *simpleui.Switch
	volume                 float32
	muted                  bool
	squelchEnabled         bool
	squelchThreshold       int
	squelchSwitch          *simpleui.Switch
	squelchSlider          *simpleui.Slider
	squelchLabel           *simpleui.Label
	squelchHoldMs          int
	squelchCloseMs         int
	volumeLabel            *simpleui.Label
	volumeSlider           *simpleui.Slider
	device                 *simpleui.Indicator
	deviceLabel            *simpleui.Label
	spectrum               []float32
	stats                  sdr.Stats
	nextSpectrumUpdate     float64
	waterfall              *Waterfall
	waterfallSettings      WaterfallSettings
	waterfallControls      []simpleui.Element
	waterfallVisible       bool
	wfOffsetLabel          *simpleui.Label
	wfContrastLabel        *simpleui.Label
	wfRangeLabel           *simpleui.Label
	wfSpeedLabel           *simpleui.Label
	wfPaletteButton        *simpleui.Button
	wfOffsetSlider         *simpleui.Slider
	wfContrastSlider       *simpleui.Slider
	wfRangeSlider          *simpleui.RangeSlider
	wfSpeedSlider          *simpleui.Slider
	toolMenu               *ToolMenu
	sdrSettings            *SDRSettings
	sdrHeader              *SDRHeaderPanel
	audioPlayer            *AudioPlayer
	uiSounds               *UISounds
	audioMeterDB           float32
	recorder               *AudioRecorder
	recorderPanel          *RecorderPanel
	recorderSkipSilence    bool
	recorderFormat         string
	demodBandwidthHz       int
	activeTool             string
	viewMode               int
	fftDisplay             *FFTDisplay
	fftAveragingMs         int
	fftRefreshFPS          int
	fftPeakHold            bool
	fftPeakDecay           float32
	fftWindow              string
	audioPanel             *AudioPanel
	memoryPanel            *MemoryPanel
	scanPanel              *ScanPanel
	dmrPanel               *DMRPanel
	aprsPanel              *APRSPanel
	aprsView               string
	rtl433Panel            *RTL433Panel
	radiosondePanel        *RadiosondePanel
	aisPanel               *AISPanel
	aircraftPanel          *AircraftPanel
	satellitePanel         *SatellitePanel
	radiosondeFamily       string
	radiosondeFrequencyHz  int64
	sstvPanel              *SSTVPanel
	tetraPanel             *TETRAPanel
	utilitiesSidebar       *UtilitiesSidebar
	sstvAutomatic          bool
	sstvMode               string
	sstvCandidateModes     [4]string
	rtl433FrequencyHz      int64
	rtl433BandwidthHz      int
	subtonePanel           *SubtonePanel
	subtoneMode            string
	dmrAutoCenter          bool
	dmrAudioSlot           string
	scanCenterToMemory     bool
	scanResume             string
	scanPolicy             string
	scanDwellMs            int
	scanMinimumHz          int64
	scanMaximumHz          int64
	memoryViewEnabled      bool
	sMeter                 *SMeter
	spectrumMinimumDB      float32
	spectrumMaximumDB      float32
	savedMode              string
	settingsPath           string
	settingsDirty          bool
	nextSettingsSave       time.Time
	savedHardware          *sdr.HardwareSettings
}

func NewMainScreen(receiver *sdr.Receiver) *MainScreen {
	screen := &MainScreen{
		receiver:               receiver,
		frequencyHz:            14_261_000,
		centerFrequencyHz:      14_261_000,
		spanHz:                 2_000_000,
		tuningStepHz:           100,
		frequencyDigitExponent: -1,
		centerMode:             true,
		volume:                 62,
		audioMeterDB:           -60,
		squelchThreshold:       -100,
		squelchHoldMs:          80,
		squelchCloseMs:         125,
		spectrum:               make([]float32, 4096),
		waterfallSettings:      defaultWaterfallSettings(),
		waterfallVisible:       true,
		activeTool:             "WATERFALL_ADJUST",
		viewMode:               1,
		themeName:              themeDark,
		spectrumMinimumDB:      -37,
		spectrumMaximumDB:      0,
		fftAveragingMs:         107,
		fftRefreshFPS:          60,
		fftPeakHold:            true,
		fftPeakDecay:           3,
		fftWindow:              "HANN",
		memoryViewEnabled:      true,
		recorderFormat:         recorderFormatMP3,
		scanResume:             "DELAY",
		scanPolicy:             "CURRENT",
		scanDwellMs:            3000,
		demodBandwidthHz:       9_000,
		bandCategory:           "HAM",
		bandName:               "20 m",
		savedMode:              "USB",
		dmrAutoCenter:          true,
		dmrAudioSlot:           "AUTO",
		aprsView:               "PACKETS",
		rtl433FrequencyHz:      433_920_000,
		rtl433BandwidthHz:      500_000,
		sstvAutomatic:          true,
		sstvMode:               "R36",
		sstvCandidateModes:     [4]string{"R36", "R72", "M1", "S1"},
		subtoneMode:            "AUTO",
		settingsPath:           defaultAppSettingsPath(),
		sMeter:                 &SMeter{},
	}
	loadAppSettings(screen.settingsPath, screen)
	screen.waterfall = NewWaterfall(&screen.waterfallSettings, len(screen.spectrum))
	screen.audioPlayer = NewAudioPlayer(receiver, nil)
	screen.uiSounds = &UISounds{}
	return screen
}

func (screen *MainScreen) CreateControls() {
	if screen.activeTool == "SCAN" || screen.activeTool == "MEMORIES" || screen.activeTool == "RECORDER" {
		screen.activeTool = "WATERFALL_ADJUST"
	}
	screen.filterSelector = NewFilterSelector(screen.selectFilter)
	screen.mode = simpleui.NewDropdown("mode", 24, 16, 180, 48, "MODE",
		[]string{"AM", "NFM", "WFM", "USB", "LSB", "CW", "DMR BETA", "ADS-B", "UAT", "TETRA"}, 16)
	for index, item := range screen.mode.Items() {
		if item == screen.savedMode {
			screen.mode.SetSelected(index)
			break
		}
	}
	screen.mode.SetMaxVisibleItems(6)
	screen.mode.OnChange(func(_ int, mode string) {
		screen.savedMode = mode
		screen.selectFilter(screen.filterSelector.Current(mode))
		if mode == "DMR BETA" && screen.activeTool != "SCAN" {
			screen.selectTool("DMR_MONITOR")
		}
		screen.markSettingsDirty()
	})

	initialFilter := screen.filterSelector.Current(screen.mode.SelectedText())
	screen.demodBandwidthHz = initialFilter.BandwidthHz
	screen.filter = simpleui.NewButton("filter", 216, 16, 130, 48, filterButtonLabel(initialFilter), 14)
	screen.filter.OnClick(func() { screen.filterSelector.Open(screen.mode.SelectedText()) })

	screen.band = simpleui.NewButton("band", 358, 16, 150, 48, "BAND  "+screen.bandName, 14)
	screen.bandSelector = NewBandSelector(screen.bandCategory, screen.bandName, screen.selectBand)
	screen.band.OnClick(screen.bandSelector.Open)

	squelch := simpleui.NewSwitch("squelch", 972, 24, 82, 28, "SQL", screen.squelchEnabled, 12)
	screen.squelchSwitch = squelch
	squelch.OnChange(func(active bool) { screen.squelchEnabled = active; screen.applySquelch(); screen.markSettingsDirty() })
	screen.squelchLabel = simpleui.NewLabel("squelchLabel", 1059, 24, 103, 22, fmt.Sprintf("LEVEL %d dBm", screen.squelchThreshold), 10)
	screen.squelchLabel.SetColor(colors.orange)
	screen.squelchSlider = simpleui.NewSlider("squelchLevel", 1152, 27, 92, 20, screen.spectrumMinimumDB, screen.spectrumMaximumDB, float32(screen.squelchThreshold))
	screen.squelchSlider.SetStep(1)
	screen.squelchSlider.OnChange(func(value float32) {
		screen.squelchThreshold = int(value)
		screen.squelchLabel.SetText(fmt.Sprintf("LEVEL %.0f dBm", value))
		screen.applySquelch()
		screen.markSettingsDirty()
	})
	holdLabel := simpleui.NewLabel("holdLabel", 972, 83, 108, 20, fmt.Sprintf("HOLD TIME %d ms", screen.squelchHoldMs), 10)
	holdLabel.SetColor(colors.muted)
	holdSlider := simpleui.NewSlider("hold", 1082, 85, 162, 18, 0, 300, float32(screen.squelchHoldMs))
	holdSlider.SetStep(1)
	holdSlider.OnChange(func(value float32) {
		screen.squelchHoldMs = int(value)
		holdLabel.SetText(fmt.Sprintf("HOLD TIME %.0f ms", value))
		screen.applySquelch()
		screen.markSettingsDirty()
	})
	closeLabel := simpleui.NewLabel("closeLabel", 972, 116, 108, 20, fmt.Sprintf("CLOSE TIME %d ms", screen.squelchCloseMs), 10)
	closeLabel.SetColor(colors.muted)
	closeSlider := simpleui.NewSlider("close", 1082, 118, 162, 18, 20, 500, float32(screen.squelchCloseMs))
	closeSlider.SetStep(1)
	closeSlider.OnChange(func(value float32) {
		screen.squelchCloseMs = int(value)
		closeLabel.SetText(fmt.Sprintf("CLOSE TIME %.0f ms", value))
		screen.applySquelch()
		screen.markSettingsDirty()
	})
	screen.syncSquelchToSpectrumRange()

	mute := simpleui.NewSwitch("mute", 338, 111, 166, 28, "MUTE", screen.muted, uiMinimumFontSize)
	screen.volumeLabel = simpleui.NewLabel("volumeLabel", 338, 148, 58, 22, fmt.Sprintf("VOL %.0f%%", screen.volume), uiMinimumFontSize)
	screen.volumeLabel.SetColor(colors.cyan)
	screen.volumeSlider = simpleui.NewSlider("volume", 397, 149, 107, 22, 0, 100, screen.volume)
	screen.volumeSlider.SetStep(1)
	screen.volumeSlider.OnChange(func(value float32) {
		screen.volume = value
		screen.audioPlayer.SetVolume(value / 100)
		screen.volumeLabel.SetText(fmt.Sprintf("VOL %.0f%%", value))
		screen.markSettingsDirty()
	})
	mute.OnChange(func(active bool) {
		screen.muted = active
		screen.audioPlayer.SetMuted(active)
		screen.volumeSlider.SetEnabled(!active)
		if active {
			screen.volumeLabel.SetText("MUTED")
			screen.volumeLabel.SetColor(colors.red)
		} else {
			screen.volumeLabel.SetText(fmt.Sprintf("VOL %.0f%%", screen.volume))
			screen.volumeLabel.SetColor(colors.cyan)
		}
		screen.markSettingsDirty()
	})

	modeLabel := "CENTER"
	if !screen.centerMode {
		modeLabel = "FIX"
	}
	screen.vfoModeSwitch = simpleui.NewSwitch("vfoMode", 1110, 159, 136, 28, modeLabel, !screen.centerMode, 12)
	screen.vfoModeSwitch.SetTrackColors(rl.Color{R: 51, G: 61, B: 70, A: 255}, colors.orange)
	screen.vfoModeSwitch.OnChange(func(fixed bool) {
		screen.centerMode = !fixed
		screen.vfoModeSwitch.SetLabel("FIX")
		if screen.centerMode {
			screen.vfoModeSwitch.SetLabel("CENTER")
			screen.centerFrequencyHz = screen.frequencyHz
		}
		if screen.receiver != nil {
			if screen.centerMode {
				screen.receiver.SetCenterFrequency(screen.centerFrequencyHz)
				screen.waterfall.Reset()
			}
			screen.receiver.SetDemodulator(screen.mode.SelectedText(), screen.frequencyHz, screen.demodBandwidthHz)
		}
		screen.markSettingsDirty()
	})
	screen.memViewSwitch = simpleui.NewSwitch("memView", 972, 159, 128, 28, "MEM VIEW", screen.memoryViewEnabled, 12)
	screen.memViewSwitch.SetTrackColors(rl.Color{R: 51, G: 61, B: 70, A: 255}, colors.blue)
	screen.memViewSwitch.OnChange(screen.setMemoryView)
	spanDown := simpleui.NewButton("spanDown", frequencyPanelX+16, frequencyPanelY+36, 36, 32, "-", 16)
	spanUp := simpleui.NewButton("spanUp", frequencyPanelX+58, frequencyPanelY+36, 36, 32, "+", 16)
	menu := simpleui.NewButton("menu", toolContentX, 842, 130, 40, "MENU", 15)
	screen.viewButton = simpleui.NewButton("view", toolContentX+140, 842, 115, 40, "VIEW 1", 14)
	screen.step = simpleui.NewButton("step", toolContentX+265, 842, 250, 40, "STEP  "+formatStep(screen.tuningStepHz), 15)
	screen.themeButton = simpleui.NewButton("theme", toolContentX+525, 842, 190, 40, "THEME  "+themeDisplayName(screen.themeName), 13)
	spanDown.OnClick(func() { screen.changeSpan(-1) })
	spanUp.OnClick(func() { screen.changeSpan(1) })
	screen.toolMenu = NewToolMenu(screen.activeTool, screen.selectTool)
	screen.toolMenu.SetSelectSound(screen.uiSounds.PlayToolSelect)
	simpleui.SetActivationFeedback(screen.uiSounds.PlayButton)
	if screen.receiver != nil && screen.savedHardware != nil {
		screen.receiver.ApplyHardwareSettings(*screen.savedHardware)
	}
	screen.sdrHeader = NewSDRHeaderPanel(screen.receiver, screen.markSettingsDirty)
	screen.stepSelector = NewStepSelector(screen.tuningStepHz, screen.selectTuningStep)
	menu.OnClick(screen.toolMenu.Open)
	screen.viewButton.OnClick(screen.cycleViewMode)
	screen.step.OnClick(screen.stepSelector.Open)
	screen.themeButton.OnClick(screen.cycleTheme)

	screen.createWaterfallControls()
	screen.fftDisplay = NewFFTDisplay(screen)
	screen.audioPanel = NewAudioPanel(screen)
	screen.memoryPanel = NewMemoryPanel(screen)
	screen.subtonePanel = NewSubtonePanel(screen)
	screen.scanPanel = NewScanPanel(screen)
	screen.dmrPanel = NewDMRPanel(screen)
	screen.aprsPanel = NewAPRSPanel(screen)
	if screen.aprsView != "" {
		screen.aprsPanel.view = screen.aprsView
	}
	screen.rtl433Panel = NewRTL433Panel(screen)
	screen.radiosondePanel = NewRadiosondePanel(screen)
	screen.aisPanel = NewAISPanel(screen)
	screen.aircraftPanel = NewAircraftPanel(screen)
	screen.satellitePanel = NewSatellitePanel(screen)
	screen.sstvPanel = NewSSTVPanel(screen)
	screen.tetraPanel = NewTETRAPanel(screen)
	if screen.rtl433FrequencyHz >= 1_000 {
		screen.rtl433Panel.targetHz = screen.rtl433FrequencyHz
	}
	if screen.rtl433BandwidthHz > 0 {
		screen.rtl433Panel.bandwidthHz = screen.rtl433BandwidthHz
	}
	if screen.activeTool == "SCAN" {
		screen.scanPanel.Enter()
	}
	screen.recorder = NewAudioRecorder()
	screen.recorder.SetSkipSquelchSilence(screen.recorderSkipSilence)
	screen.recorder.SetFormat(screen.recorderFormat)
	screen.audioPlayer.SetRecorder(screen.recorder)
	screen.recorderPanel = NewRecorderPanel(screen, screen.recorder)
	screen.utilitiesSidebar = NewUtilitiesSidebar(screen)
	screen.audioPlayer.SetVolume(screen.volume / 100)
	screen.audioPlayer.SetMuted(screen.muted)

	// Every tool except TETRA still describes its layout in the original
	// full-width coordinate system. Fit its interactive controls beside the
	// permanent utilities column using the same mapping as its drawn content.
	compactToolControls(screen.waterfallControls)
	compactToolControls(screen.fftDisplay.controls)
	compactToolControls(screen.audioPanel.controls)
	compactToolControls(screen.dmrPanel.controls)
	compactToolControls(screen.aprsPanel.controls)
	compactToolControls(screen.rtl433Panel.controls)
	compactToolControls(screen.radiosondePanel.controls)
	compactToolControls(screen.aisPanel.controls)
	compactToolControls(screen.aircraftPanel.controls)
	compactToolControls(screen.satellitePanel.controls)
	compactToolControls(screen.sstvPanel.controls)

	for _, element := range []simpleui.Element{
		screen.mode, screen.filter, screen.band,
		squelch, screen.squelchLabel, screen.squelchSlider, holdLabel, holdSlider, closeLabel, closeSlider,
		mute, screen.volumeLabel, screen.volumeSlider, screen.vfoModeSwitch, screen.memViewSwitch,
		spanDown, spanUp, menu, screen.viewButton, screen.step, screen.themeButton,
	} {
		simpleui.Add(element)
	}
	for _, element := range screen.waterfallControls {
		simpleui.Add(element)
	}
	for _, element := range screen.fftDisplay.controls {
		simpleui.Add(element)
	}
	for _, element := range screen.audioPanel.controls {
		simpleui.Add(element)
	}
	for _, element := range screen.memoryPanel.controls {
		simpleui.Add(element)
	}
	simpleui.Add(screen.memoryPanel)
	for _, element := range screen.recorderPanel.toolControls {
		simpleui.Add(element)
	}
	for _, element := range screen.dmrPanel.controls {
		simpleui.Add(element)
	}
	for _, element := range screen.aprsPanel.controls {
		simpleui.Add(element)
	}
	for _, element := range screen.rtl433Panel.controls {
		simpleui.Add(element)
	}
	for _, element := range screen.radiosondePanel.controls {
		simpleui.Add(element)
	}
	for _, element := range screen.aisPanel.controls {
		simpleui.Add(element)
	}
	for _, element := range screen.aircraftPanel.controls {
		simpleui.Add(element)
	}
	for _, element := range screen.satellitePanel.controls {
		simpleui.Add(element)
	}
	for _, element := range screen.sstvPanel.controls {
		simpleui.Add(element)
	}
	for _, element := range screen.tetraPanel.controls {
		simpleui.Add(element)
	}
	for _, element := range screen.utilitiesSidebar.controls {
		simpleui.Add(element)
	}
	for _, element := range screen.subtonePanel.controls() {
		simpleui.Add(element)
	}
	for _, element := range screen.sdrHeader.controls {
		simpleui.Add(element)
	}
	simpleui.Add(screen.toolMenu)
	simpleui.Add(screen.bandSelector)
	simpleui.Add(screen.stepSelector)
	simpleui.Add(screen.filterSelector)
	simpleui.Add(screen.recorderPanel)
	screen.applyTheme(screen.themeName)
	// Apply the restored workspace only after every tool control exists.
	screen.setViewMode(screen.viewMode)
	if screen.activeTool == "RTL_433" {
		screen.rtl433Panel.Enter()
	}
	if screen.activeTool == "RADIOSONDE" {
		screen.radiosondePanel.Enter()
	}
	if screen.activeTool == "AIS" {
		screen.aisPanel.Enter()
	}
	if screen.activeTool == "AIRCRAFT" {
		screen.aircraftPanel.Enter()
	}
	if screen.activeTool == "SATELLITES" {
		screen.satellitePanel.Enter()
	}
	if screen.activeTool == "APRS" {
		screen.aprsPanel.Enter()
	}
	if screen.activeTool == "SSTV" {
		screen.sstvPanel.Enter()
	}
	if screen.activeTool == "TETRA" {
		screen.tetraPanel.Enter()
	}
	if screen.receiver != nil {
		screen.receiver.SetCenterFrequency(screen.centerFrequencyHz)
		screen.receiver.SetDemodulator(screen.mode.SelectedText(), screen.frequencyHz, screen.demodBandwidthHz)
		screen.receiver.SetDMRAutoCenter(screen.dmrAutoCenter)
		screen.receiver.SetDMRAudioSlot(screen.dmrAudioSlot)
	}
	screen.markSettingsDirty()
}

func (screen *MainScreen) Draw() {
	if screen.vfoModeSwitch != nil {
		if screen.centerMode {
			screen.vfoModeSwitch.SetLabel("CENTER")
		} else {
			screen.vfoModeSwitch.SetLabel("FIX")
		}
	}
	screen.flushSettings(false)
	screen.sdrHeader.Tick()
	screen.recorderPanel.Tick()
	// Valid DMR voice frames are already gated by DSDcc; the RF squelch must
	// never cut or omit decoded digital audio from a recording.
	digitalAudio := screen.mode != nil && (screen.mode.SelectedText() == "DMR BETA" || screen.mode.SelectedText() == "TETRA")
	screen.audioPlayer.SetRecorderSquelch(screen.squelchEnabled && !digitalAudio, screen.stats.SquelchOpen)
	screen.audioPlayer.Pump()
	screen.uiSounds.EnsureLoaded()
	screen.updateAudioMeter()
	screen.audioPanel.UpdateSpectrum()
	screen.memoryPanel.Tick()
	if screen.scanPanel != nil {
		screen.scanPanel.UpdateInput()
	}
	if screen.utilitiesSidebar != nil {
		screen.utilitiesSidebar.UpdateInput()
	}
	if screen.rtl433Panel != nil {
		screen.rtl433Panel.Tick()
	}
	if screen.radiosondePanel != nil {
		screen.radiosondePanel.Tick()
	}
	if screen.aisPanel != nil {
		screen.aisPanel.Tick()
	}
	if screen.aircraftPanel != nil {
		screen.aircraftPanel.Tick()
	}
	if screen.satellitePanel != nil {
		screen.satellitePanel.Tick()
	}
	if screen.aprsPanel != nil {
		screen.aprsPanel.Tick()
	}
	if screen.sstvPanel != nil {
		screen.sstvPanel.Tick()
	}
	if screen.tetraPanel != nil {
		screen.tetraPanel.Tick()
	}
	screen.updateFrequencyInteraction()
	screen.updateSpectrumDrag()
	if screen.receiver != nil && rl.GetTime() >= screen.nextSpectrumUpdate {
		screen.stats = screen.receiver.Snapshot(screen.spectrum)
		if screen.stats.FFTBlocks > 0 {
			screen.sMeter.Update(screen.stats.SignalDBm)
		}
		screen.nextSpectrumUpdate = rl.GetTime() + 1.0/float64(screen.fftDisplay.refreshFPS)
		screen.fftDisplay.UpdatePeaks(screen.spectrum)
		if screen.scanPanel != nil {
			screen.scanPanel.Update(screen.spectrum)
		}
	}
	rl.DrawRectangle(0, 0, int32(designWidth), int32(designHeight), colors.background)
	screen.drawHeader()
	screen.drawSpectrum()
	if screen.receiver != nil {
		screen.waterfall.Update(screen.receiver, screen.stats.SampleRate, screen.spanHz)
	}
	screen.drawWaterfall()
	if screen.viewMode == 1 {
		screen.drawLowerWorkspace()
	}
	screen.drawFooterBackground()
	screen.utilitiesSidebar.Draw()
}

func (screen *MainScreen) Close() {
	screen.flushSettings(true)
	simpleui.SetActivationFeedback(nil)
	if screen.uiSounds != nil {
		screen.uiSounds.Close()
	}
	screen.audioPlayer.Close()
	if screen.recorder != nil {
		screen.recorder.Close()
	}
	if screen.rtl433Panel != nil {
		screen.rtl433Panel.Close()
	}
	if screen.radiosondePanel != nil {
		screen.radiosondePanel.Close()
	}
	if screen.aisPanel != nil {
		screen.aisPanel.Close()
	}
	if screen.aircraftPanel != nil {
		screen.aircraftPanel.Close()
	}
	if screen.aprsPanel != nil {
		screen.aprsPanel.Close()
	}
	if screen.sstvPanel != nil {
		screen.sstvPanel.Leave()
		screen.sstvPanel.Close()
	}
	if screen.tetraPanel != nil {
		screen.tetraPanel.Close()
	}
	if screen.satellitePanel != nil {
		screen.satellitePanel.Close()
	}
}

func (screen *MainScreen) applySquelch() {
	if screen.receiver != nil {
		screen.receiver.SetSquelch(screen.squelchEnabled, float32(screen.squelchThreshold), screen.squelchHoldMs, screen.squelchCloseMs)
	}
}

func (screen *MainScreen) setMemoryView(visible bool) {
	screen.memoryViewEnabled = visible
	if screen.memViewSwitch != nil {
		screen.memViewSwitch.SetActive(visible)
	}
	if screen.memoryPanel != nil {
		screen.memoryPanel.SetMarkersVisible(visible)
	}
	screen.markSettingsDirty()
}

// syncSquelchToSpectrumRange keeps both the control and DSP threshold inside
// the dB interval that is currently visible in the FFT graph.
func (screen *MainScreen) syncSquelchToSpectrumRange() {
	if screen.squelchSlider == nil {
		return
	}
	screen.squelchSlider.SetRange(screen.spectrumMinimumDB, screen.spectrumMaximumDB)
	threshold := min(max(float32(screen.squelchThreshold), screen.spectrumMinimumDB), screen.spectrumMaximumDB)
	screen.squelchThreshold = int(math.Round(float64(threshold)))
	screen.squelchSlider.SetValue(float32(screen.squelchThreshold))
	screen.squelchLabel.SetText(fmt.Sprintf("LEVEL %d dBm", screen.squelchThreshold))
	screen.applySquelch()
}

func (screen *MainScreen) drawHeader() {
	rl.DrawRectangle(0, 0, int32(designWidth), 205, colors.panel)
	rl.DrawLine(0, 204, int32(designWidth), 204, colors.border)
	screen.drawSquelchPanel()
	screen.sdrHeader.DrawBackground()
	screen.subtonePanel.Draw()

	screen.sMeter.Draw(24, 94, 290, 96)
	drawPanel(326, 94, 190, 96)
	simpleui.DrawTextStyled(fmt.Sprintf("AUDIO  %.0f dB", screen.audioMeterDB), 338, 101, 12, simpleui.FontSemiBold, colors.cyan)
	drawCompactAudioMeter(338, 178, 166, 7, screen.audioMeterDB)

	screen.drawFrequencyDisplay()
}

const (
	frequencyPanelX   = float32(530)
	frequencyPanelY   = float32(94)
	frequencyPanelW   = float32(420)
	frequencyPanelH   = float32(96)
	frequencyFontSize = int32(39)
	frequencyDividerX = frequencyPanelX + 108
	frequencyDialX    = frequencyDividerX + 1
	frequencyDialW    = frequencyPanelX + frequencyPanelW - frequencyDialX
)

func (screen *MainScreen) drawFrequencyDisplay() {
	drawPanel(frequencyPanelX, frequencyPanelY, frequencyPanelW, frequencyPanelH)
	rl.DrawLineEx(rl.Vector2{X: frequencyDividerX, Y: frequencyPanelY + 7}, rl.Vector2{X: frequencyDividerX, Y: frequencyPanelY + frequencyPanelH - 7}, 1, colors.border)
	simpleui.DrawTextStyled("SPAN", frequencyPanelX+16, frequencyPanelY+9, 11, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawTextStyled("VFO A  ·  "+screen.mode.SelectedText(), frequencyDialX+15, frequencyPanelY+8, 12, simpleui.FontSemiBold, colors.cyan)

	formatted := formatDialFrequency(screen.frequencyHz)
	totalWidth := simpleui.MeasureTextStyled(formatted, frequencyFontSize, simpleui.FontMono).X
	cursor := frequencyDialX + (frequencyDialW-totalWidth)*.5
	digitCount := countFrequencyDigits(formatted)
	seenDigits := 0
	for _, character := range formatted {
		text := string(character)
		characterWidth := simpleui.MeasureTextStyled(text, frequencyFontSize, simpleui.FontMono).X
		exponent := -1
		if character >= '0' && character <= '9' {
			exponent = digitCount - seenDigits - 1
			seenDigits++
		}
		selected := exponent == screen.frequencyDigitExponent
		if selected {
			rl.DrawRectangleRounded(rl.Rectangle{X: cursor - 2, Y: frequencyPanelY + 24, Width: characterWidth + 4, Height: 45}, .18, 5, rl.Color{R: 25, G: 125, B: 190, A: 150})
		}
		color := colors.text
		if selected {
			color = rl.White
		}
		simpleui.DrawTextStyled(text, cursor, frequencyPanelY+25, frequencyFontSize, simpleui.FontMono, color)
		cursor += characterWidth
	}

	footerY := frequencyPanelY + frequencyPanelH - 18
	spanValue := fmt.Sprintf("%.3f MHz", float64(screen.spanHz)/1_000_000)
	spanWidth := simpleui.MeasureTextStyled(spanValue, 11, simpleui.FontSemiBold).X
	simpleui.DrawTextStyled(spanValue, frequencyPanelX+(108-spanWidth)*.5, footerY, 11, simpleui.FontSemiBold, colors.muted)
	if screen.frequencyDigitExponent >= 0 {
		label := "DIGIT STEP  " + formatStep(screen.digitStepHz())
		width := simpleui.MeasureTextStyled(label, 14, simpleui.FontSemiBold).X
		simpleui.DrawTextStyled(label, frequencyDialX+(frequencyDialW-width)*.5, footerY-1, 14, simpleui.FontSemiBold, rl.Color{R: 125, G: 205, B: 255, A: 255})
	} else {
		label := "STEP  " + formatStep(screen.tuningStepHz)
		width := simpleui.MeasureTextStyled(label, 12, simpleui.FontSemiBold).X
		simpleui.DrawTextStyled(label, frequencyDialX+frequencyDialW*.47-width*.5, footerY, 12, simpleui.FontSemiBold, colors.orange)
	}
	centerLabel, centerColor := "FIX", colors.orange
	if screen.centerMode {
		centerLabel, centerColor = "CENTER", colors.green
	}
	centerWidth := simpleui.MeasureTextStyled(centerLabel, 12, simpleui.FontSemiBold).X
	simpleui.DrawTextStyled(centerLabel, frequencyPanelX+frequencyPanelW-18-centerWidth, footerY, 12, simpleui.FontSemiBold, centerColor)
}

func formatDialFrequency(hz int64) string {
	return fmt.Sprintf("%d.%03d.%03d", hz/1_000_000, (hz/1_000)%1_000, hz%1_000)
}

func countFrequencyDigits(value string) int {
	count := 0
	for _, character := range value {
		if character >= '0' && character <= '9' {
			count++
		}
	}
	return count
}

func (screen *MainScreen) updateAudioMeter() {
	targetDB := float32(-60)
	if peak := screen.audioPlayer.ConsumeAudioPeak(); peak > 0 {
		targetDB = max(20*float32(math.Log10(float64(peak))), -60)
	}

	// Fast attack makes speech peaks visible; the slower release avoids flicker
	// between audio callbacks and matches the behaviour of a physical meter.
	delta := targetDB - screen.audioMeterDB
	rate := float32(4)
	if delta > 0 {
		rate = 22
	}
	amount := min(rate*rl.GetFrameTime(), 1)
	screen.audioMeterDB += delta * amount
}

func drawCompactAudioMeter(x, y, width, height, db float32) {
	rl.DrawRectangleRounded(rl.Rectangle{X: x, Y: y, Width: width, Height: height}, 1, 5, rl.Color{R: 38, G: 49, B: 57, A: 255})
	fraction := min(max((db+60)/60, 0), 1)
	color := colors.cyan
	if db > -12 {
		color = colors.orange
	}
	if db > -3 {
		color = colors.red
	}
	rl.DrawRectangleRounded(rl.Rectangle{X: x, Y: y, Width: width * fraction, Height: height}, 1, 5, color)
}

func (screen *MainScreen) drawSquelchPanel() {
	panel := rl.Rectangle{X: 960, Y: 16, Width: 296, Height: 134}
	rl.DrawRectangleRounded(panel, .06, 8, colors.panel)
	rl.DrawRectangleRoundedLinesEx(panel, .06, 8, 1.5, colors.border)

	timelineX, timelineY, timelineWidth := float32(972), float32(64), float32(272)
	total := max(screen.squelchHoldMs+screen.squelchCloseMs, 1)
	holdWidth := timelineWidth * float32(screen.squelchHoldMs) / float32(total)
	rl.DrawRectangleRounded(rl.Rectangle{X: timelineX, Y: timelineY, Width: holdWidth, Height: 9}, .4, 6, mixColor(colors.panelAlt, colors.orange, .18))
	rl.DrawRectangleRounded(rl.Rectangle{X: timelineX + holdWidth, Y: timelineY, Width: timelineWidth - holdWidth, Height: 9}, .4, 6, mixColor(colors.panelAlt, colors.red, .18))
	if screen.squelchEnabled {
		rl.DrawRectangleRounded(rl.Rectangle{X: timelineX, Y: timelineY, Width: holdWidth, Height: 9}, .4, 6, colors.orange)
	}
	simpleui.DrawTextStyled("HOLD", timelineX, timelineY-17, 9, simpleui.FontSemiBold, colors.orange)
	closeText := "CLOSE → 0"
	closeWidth := simpleui.MeasureTextStyled(closeText, 9, simpleui.FontSemiBold).X
	simpleui.DrawTextStyled(closeText, timelineX+timelineWidth-closeWidth, timelineY-17, 9, simpleui.FontSemiBold, colors.red)

	dock := rl.Rectangle{X: 960, Y: 156, Width: 296, Height: 34}
	rl.DrawRectangleRounded(dock, .2, 6, colors.panel)
	rl.DrawRectangleRoundedLinesEx(dock, .2, 6, 1, colors.border)
	rl.DrawLine(1105, 160, 1105, 186, colors.border)
}

func (screen *MainScreen) drawSpectrum() {
	x, y, width, height := screen.spectrumGeometry()
	drawPanel(x, y, width, height)
	drawGrid(x, y, width, height, 10, 6)
	if screen.rtl433Panel != nil {
		screen.rtl433Panel.DrawSpectrumOverlay(x, y, width, height)
	}
	// Leave the left dB scale its own lane so the spectrum title never overlaps it.
	drawSmallText("RF SPECTRUM", x+58, y+10, colors.cyan)
	for division := 0; division <= 5; division++ {
		fraction := float32(division) / 5
		level := screen.spectrumMaximumDB + (screen.spectrumMinimumDB-screen.spectrumMaximumDB)*fraction
		labelY := screen.spectrumY(level, y, height)
		drawSmallText(fmt.Sprintf("%.0f dBm", level), x+5, labelY+2, colors.muted)
	}

	if screen.stats.FFTBlocks > 0 {
		// A solid blue spectrum makes occupied channels much easier to identify
		// at a glance. Each narrow column receives its own vertical gradient so
		// the live outline remains exact while the grid stays visible beneath it.
		traceColor := rl.Color{R: 205, G: 238, B: 255, A: 255}
		fillTop := rl.Color{R: 64, G: 164, B: 238, A: 205}
		fillBottom := rl.Color{R: 7, G: 28, B: 105, A: 135}
		graphBottom := y + height - 27
		var previous rl.Vector2
		hasPrevious := false
		for pixel := 0; pixel < int(width); pixel++ {
			value, visible := screen.interpolatedSpectrumValue(screen.spectrum, float32(pixel)/width)
			if !visible {
				hasPrevious = false
				continue
			}
			current := rl.Vector2{X: x + float32(pixel), Y: screen.spectrumY(value, y, height)}
			columnTop := min(max(current.Y, y+1), graphBottom)
			columnHeight := graphBottom - columnTop
			if columnHeight > 0 {
				rl.DrawRectangleGradientV(int32(current.X), int32(columnTop), 2, int32(columnHeight+1), fillTop, fillBottom)
			}
			if hasPrevious {
				rl.DrawLineEx(previous, current, 1.35, traceColor)
			}
			previous = current
			hasPrevious = true
		}
		if screen.fftDisplay.peakHold {
			screen.drawPeakSpectrum(x, y, width, height)
		}
	} else {
		message := screen.stats.Status
		if message == "" {
			message = "Waiting for IQ stream…"
		}
		simpleui.DrawText(message, x+48, y+height*.5, 13, colors.orange)
	}
	// Operational overlays stay above the blue fill so their boundaries and
	// labels never disappear inside a strong carrier.
	screen.drawDemodulatedBandwidth(x, y, width, height)
	// Paint the live tuning cursor before memory markers. Memory lines and
	// labels then remain readable where they cross the current VFO.
	centerX := x + width*.5
	cursorX := centerX + float32(screen.frequencyHz-screen.centerFrequencyHz)/float32(screen.spanHz)*width
	if cursorX >= x && cursorX <= x+width {
		markerColor := rl.Color{R: 45, G: 255, B: 110, A: 255}
		rl.DrawLineEx(rl.Vector2{X: cursorX, Y: y}, rl.Vector2{X: cursorX, Y: y + height - 26}, 2, markerColor)
		screen.drawTuningCursorLabel(cursorX, y, x, width, markerColor)
	}
	if screen.memoryPanel != nil {
		screen.memoryPanel.DrawMarkers(x, y, width, height)
	}
	// Preserve the original IC-SDR band-plan ribbon immediately above the
	// frequency scale. It owns this layer and is painted after memory markers,
	// guaranteeing that its text and boundaries always remain unobstructed.
	screen.drawBandPlanStrip(x, y+height-52, width, 26)
	if screen.scanPanel != nil {
		screen.scanPanel.DrawSpectrumOverlay(x, y, width, height)
	}
	scannerUsesSQL := screen.scanPanel != nil && (screen.activeTool == "SCAN" || screen.scanPanel.running)
	if screen.squelchEnabled || scannerUsesSQL {
		sqlY := screen.spectrumY(float32(screen.squelchThreshold), y, height)
		rl.DrawLineEx(rl.Vector2{X: x, Y: sqlY}, rl.Vector2{X: x + width, Y: sqlY}, 1.5, rl.Color{R: 255, G: 105, B: 45, A: 175})
		label := fmt.Sprintf("SQL %d", screen.squelchThreshold)
		labelWidth := simpleui.MeasureTextStyled(label, 11, simpleui.FontSemiBold).X
		tag := rl.Rectangle{X: x + 5, Y: sqlY - 10, Width: labelWidth + 12, Height: 18}
		rl.DrawRectangleRounded(tag, .2, 6, rl.Color{R: 15, G: 18, B: 26, A: 220})
		simpleui.DrawTextStyled(label, x+11, sqlY-7, 11, simpleui.FontSemiBold, rl.Color{R: 255, G: 135, B: 75, A: 255})
	}
	axisBackground := colors.panelAlt
	axisBackground.A = 245
	rl.DrawRectangle(int32(x), int32(y+height-26), int32(width), 26, axisBackground)
	// Match the vertical grid with a readable frequency at every division.
	// The previous five tiny labels left too much distance between references.
	for mark := 0; mark <= 10; mark++ {
		markX := x + width*float32(mark)/10
		frequency := float64(screen.centerFrequencyHz-screen.spanHz/2+int64(mark)*screen.spanHz/10) / 1e6
		label := fmt.Sprintf("%.3f", frequency)
		labelWidth := simpleui.MeasureTextStyled(label, 12, simpleui.FontMono).X
		labelX := min(max(markX-labelWidth*.5, x+4), x+width-labelWidth-4)
		simpleui.DrawTextStyled(label, labelX, y+height-21, 12, simpleui.FontMono, colors.text)
	}
	memoryTooltipVisible := false
	if screen.memoryPanel != nil {
		memoryTooltipVisible = screen.memoryPanel.DrawMarkerTooltip(x, y, width, height)
	}
	if !memoryTooltipVisible {
		screen.drawSpectrumHoverTooltip(x, y, width, height)
	}
}

func (screen *MainScreen) drawSpectrumHoverTooltip(x, y, width, height float32) {
	if screen.stats.FFTBlocks == 0 || len(screen.spectrum) == 0 || screen.spanHz <= 0 || screen.overlayOpen() {
		return
	}
	mouse := simpleui.MousePosition()
	graphBottom := y + height - 26
	if mouse.X < x || mouse.X > x+width || mouse.Y < y || mouse.Y > graphBottom {
		return
	}
	fraction := min(max((mouse.X-x)/width, 0), 1)
	level, ok := screen.interpolatedSpectrumValue(screen.spectrum, fraction)
	if !ok {
		return
	}
	frequencyHz := float64(screen.centerFrequencyHz-screen.spanHz/2) + float64(fraction)*float64(screen.spanHz)
	frequencyText := fmt.Sprintf("%.6f MHz", frequencyHz/1e6)
	levelText := fmt.Sprintf("%+.1f dBFS", level)
	fontSize := int32(13)
	textWidth := max(simpleui.MeasureTextStyled(frequencyText, fontSize, simpleui.FontMono).X,
		simpleui.MeasureTextStyled(levelText, fontSize, simpleui.FontMono).X)
	box := rl.Rectangle{X: mouse.X + 14, Y: mouse.Y + 14, Width: textWidth + 22, Height: 48}
	if box.X+box.Width > x+width-5 {
		box.X = mouse.X - box.Width - 14
	}
	if box.Y+box.Height > graphBottom-5 {
		box.Y = mouse.Y - box.Height - 14
	}
	box.X = min(max(box.X, x+5), x+width-box.Width-5)
	box.Y = min(max(box.Y, y+5), graphBottom-box.Height-5)
	// A subtle vertical guide makes the sampled FFT bin unambiguous without
	// obscuring the trace or the memory markers.
	guideColor := colors.cyan
	guideColor.A = 120
	rl.DrawLineEx(rl.Vector2{X: mouse.X, Y: y + 1}, rl.Vector2{X: mouse.X, Y: graphBottom}, 1, guideColor)
	tooltipBackground := colors.panel
	tooltipBackground.A = 248
	rl.DrawRectangleRounded(box, .14, 7, tooltipBackground)
	rl.DrawRectangleRoundedLinesEx(box, .14, 7, 1, colors.border)
	simpleui.DrawTextStyled(frequencyText, box.X+11, box.Y+6, fontSize, simpleui.FontMono, simpleui.EnsureTextContrast(colors.text, tooltipBackground))
	simpleui.DrawTextStyled(levelText, box.X+11, box.Y+26, fontSize, simpleui.FontMono, simpleui.EnsureTextContrast(colors.cyan, tooltipBackground))
}

func (screen *MainScreen) drawDemodulatedBandwidth(x, y, width, height float32) {
	if screen.spanHz <= 0 || screen.demodBandwidthHz <= 0 {
		return
	}
	cursorX := x + width*(.5+float32(screen.frequencyHz-screen.centerFrequencyHz)/float32(screen.spanHz))
	bandPixels := width * float32(screen.demodBandwidthHz) / float32(screen.spanHz)
	left, right := cursorX-bandPixels*.5, cursorX+bandPixels*.5
	mode := screen.mode.SelectedText()
	if (mode == "USB" || mode == "LSB") && screen.audioPanel != nil && !screen.audioPanel.pbtBypassed {
		lowPixels := width * float32(screen.audioPanel.pbtLow) / float32(screen.spanHz)
		highPixels := width * float32(screen.audioPanel.pbtHigh) / float32(screen.spanHz)
		if mode == "USB" {
			left, right = cursorX+lowPixels, cursorX+highPixels
		} else {
			left, right = cursorX-highPixels, cursorX-lowPixels
		}
	} else {
		switch mode {
		case "USB", "CW":
			left, right = cursorX, cursorX+bandPixels
		case "LSB":
			left, right = cursorX-bandPixels, cursorX
		}
	}
	visibleLeft, visibleRight := max(left, x), min(right, x+width)
	if visibleRight <= visibleLeft {
		return
	}
	fill := rl.Color{R: colors.orange.R, G: colors.orange.G, B: colors.orange.B, A: 38}
	rl.DrawRectangleRec(rl.Rectangle{X: visibleLeft, Y: y + 1, Width: visibleRight - visibleLeft, Height: height - 27}, fill)
	rl.DrawLineEx(rl.Vector2{X: visibleLeft, Y: y + 1}, rl.Vector2{X: visibleLeft, Y: y + height - 27}, 1, colors.orange)
	rl.DrawLineEx(rl.Vector2{X: visibleRight, Y: y + 1}, rl.Vector2{X: visibleRight, Y: y + height - 27}, 1, colors.orange)
	labelBandwidth := screen.demodBandwidthHz
	if (mode == "USB" || mode == "LSB") && screen.audioPanel != nil && !screen.audioPanel.pbtBypassed {
		labelBandwidth = screen.audioPanel.pbtHigh - screen.audioPanel.pbtLow
	}
	_ = labelBandwidth // Rendered together with the frequency in the cursor plate.
}

func (screen *MainScreen) drawTuningCursorLabel(cursorX, y, graphX, graphWidth float32, markerColor rl.Color) {
	frequency := fmt.Sprintf("%.6f MHz", float64(screen.frequencyHz)/1e6)
	bandwidth := "BW  " + formatFilterBandwidth(screen.demodBandwidthHz)
	if mode := screen.mode.SelectedText(); (mode == "USB" || mode == "LSB") && screen.audioPanel != nil && !screen.audioPanel.pbtBypassed {
		bandwidth = "PBT  " + formatFilterBandwidth(screen.audioPanel.pbtHigh-screen.audioPanel.pbtLow)
	}
	frequencySize, detailSize := int32(15), int32(12)
	freqWidth := simpleui.MeasureTextStyled(frequency, frequencySize, simpleui.FontMono).X
	detailWidth := simpleui.MeasureTextStyled(bandwidth, detailSize, simpleui.FontSemiBold).X
	plateWidth := max(freqWidth, detailWidth) + 22
	plateX := min(max(cursorX-plateWidth/2, graphX+6), graphX+graphWidth-plateWidth-6)
	plate := rl.Rectangle{X: plateX, Y: y + 25, Width: plateWidth, Height: 50}
	plateBackground := colors.panel
	plateBackground.A = 248
	rl.DrawRectangleRounded(plate, .14, 8, plateBackground)
	rl.DrawRectangleRoundedLinesEx(plate, .14, 8, 1.5, markerColor)
	simpleui.DrawTextStyled(frequency, plate.X+(plate.Width-freqWidth)/2, plate.Y+6, frequencySize, simpleui.FontMono, simpleui.EnsureTextContrast(markerColor, plateBackground))
	simpleui.DrawTextStyled(bandwidth, plate.X+(plate.Width-detailWidth)/2, plate.Y+29, detailSize, simpleui.FontSemiBold, simpleui.EnsureTextContrast(colors.orange, plateBackground))
}

func (screen *MainScreen) drawLowerWorkspace() {
	drawPanel(toolContentX, toolY, toolContentRight-toolContentX, toolH)
	if screen.activeTool == "TETRA" && !screen.waterfallVisible {
		screen.tetraPanel.DrawPanel()
		return
	}
	drawCompactedTool(func() {
		if screen.waterfallVisible {
			drawSmallText("WATERFALL ADJUST", 40, toolY+12, colors.cyan)
		} else if screen.activeTool == "FFT" {
			screen.fftDisplay.DrawPanel()
		} else if screen.activeTool == "PBT_AUDIO" {
			screen.audioPanel.DrawPanel()
		} else if screen.activeTool == "DMR_MONITOR" {
			screen.dmrPanel.DrawPanel()
		} else if screen.activeTool == "RTL_433" {
			screen.rtl433Panel.DrawPanel()
		} else if screen.activeTool == "RADIOSONDE" {
			screen.radiosondePanel.DrawPanel()
		} else if screen.activeTool == "AIS" {
			screen.aisPanel.DrawPanel()
		} else if screen.activeTool == "AIRCRAFT" {
			screen.aircraftPanel.DrawPanel()
		} else if screen.activeTool == "APRS" {
			screen.aprsPanel.DrawPanel()
		} else if screen.activeTool == "SSTV" {
			screen.sstvPanel.DrawPanel()
		} else if screen.activeTool == "SATELLITES" {
			screen.satellitePanel.DrawPanel()
		} else {
			drawSmallText(toolDisplayName(screen.activeTool), 40, toolY+14, colors.cyan)
			simpleui.DrawText("This tool will be implemented in a later phase. Press MENU to change tools.", 40, toolY+48, 10, colors.muted)
		}
	})
}

func (screen *MainScreen) drawWaterfall() {
	tuningFraction := float32(.5) + float32(screen.frequencyHz-screen.centerFrequencyHz)/float32(screen.spanHz)
	x, y, width, height := screen.waterfallGeometry()
	screen.waterfall.Draw(x, y, width, height, tuningFraction)
}

func (screen *MainScreen) drawFooterBackground() {
	rl.DrawRectangle(int32(toolContentX), 830, int32(designWidth-toolContentX), 70, colors.background)
	rl.DrawLine(int32(toolContentX), 830, int32(designWidth), 830, colors.border)
}

func (screen *MainScreen) createWaterfallControls() {
	screen.wfOffsetLabel = simpleui.NewLabel("wfOffsetLabel", 40, 650, 175, 18, "COLOR OFFSET  0 dB", 12)
	screen.wfOffsetLabel.SetAlignment(simpleui.AlignCenter)
	screen.wfOffsetSlider = simpleui.NewSlider("wfOffset", 48, 684, 167, 20, -80, 40, float32(screen.waterfallSettings.ColorOffsetDB))
	screen.wfOffsetSlider.SetStep(1)

	screen.wfContrastLabel = simpleui.NewLabel("wfContrastLabel", 230, 650, 180, 18, "CONTRAST  100 %", 12)
	screen.wfContrastLabel.SetAlignment(simpleui.AlignCenter)
	screen.wfContrastSlider = simpleui.NewSlider("wfContrast", 238, 684, 170, 20, 25, 200, float32(screen.waterfallSettings.Contrast))
	screen.wfContrastSlider.SetStep(1)

	screen.wfRangeLabel = simpleui.NewLabel("wfRangeLabel", 430, 650, 210, 18, "LEVEL  -80 / -20 dBm", 12)
	screen.wfRangeLabel.SetAlignment(simpleui.AlignCenter)
	screen.wfRangeSlider = simpleui.NewRangeSlider("wfRange", 438, 684, 194, 20, -140, 20, screen.waterfallSettings.MinimumDBm, screen.waterfallSettings.MaximumDBm)
	screen.wfRangeSlider.SetStep(1)
	screen.wfRangeSlider.SetMinimumGap(10)
	screen.wfRangeSlider.SetRangeDragging(false)

	screen.wfSpeedLabel = simpleui.NewLabel("wfSpeedLabel", 655, 650, 175, 18, "SPEED  31 lines/s", 12)
	screen.wfSpeedLabel.SetAlignment(simpleui.AlignCenter)
	screen.wfSpeedSlider = simpleui.NewSlider("wfSpeed", 660, 684, 170, 20, 5, 60, float32(screen.waterfallSettings.LinesPerSecond))
	screen.wfSpeedSlider.SetStep(1)

	screen.wfPaletteButton = simpleui.NewButton("wfPalette", 850, 657, 150, 48, "PALETTE  BLUE", 12)
	reset := simpleui.NewButton("wfReset", 1015, 657, 90, 48, "RESET", 12)
	closeButton := simpleui.NewButton("wfClose", 1120, 657, 110, 48, "CLOSE", 12)

	screen.wfOffsetSlider.OnChange(func(value float32) {
		screen.waterfallSettings.ColorOffsetDB = int(value)
		screen.refreshWaterfallControls()
		screen.waterfall.InvalidateColors()
		screen.markSettingsDirty()
	})
	screen.wfContrastSlider.OnChange(func(value float32) {
		screen.waterfallSettings.Contrast = int(value)
		screen.refreshWaterfallControls()
		screen.waterfall.InvalidateColors()
		screen.markSettingsDirty()
	})
	screen.wfRangeSlider.OnChange(func(low, high float32) {
		screen.waterfallSettings.MinimumDBm = low
		screen.waterfallSettings.MaximumDBm = high
		screen.refreshWaterfallControls()
		screen.waterfall.InvalidateColors()
		screen.markSettingsDirty()
	})
	screen.wfSpeedSlider.OnChange(func(value float32) {
		screen.waterfallSettings.LinesPerSecond = int(value)
		screen.refreshWaterfallControls()
		screen.markSettingsDirty()
	})
	screen.wfPaletteButton.OnClick(func() {
		palettes := []string{"BLUE", "VIRIDIS", "FIRE", "GRAY"}
		index := 0
		for current, palette := range palettes {
			if palette == screen.waterfallSettings.Palette {
				index = current
				break
			}
		}
		screen.waterfallSettings.Palette = palettes[(index+1)%len(palettes)]
		screen.refreshWaterfallControls()
		screen.waterfall.InvalidateColors()
		screen.markSettingsDirty()
	})
	reset.OnClick(func() {
		screen.waterfallSettings = factoryWaterfallSettings()
		screen.waterfall.settings = &screen.waterfallSettings
		screen.wfOffsetSlider.SetValue(float32(screen.waterfallSettings.ColorOffsetDB))
		screen.wfContrastSlider.SetValue(float32(screen.waterfallSettings.Contrast))
		screen.wfRangeSlider.SetValues(screen.waterfallSettings.MinimumDBm, screen.waterfallSettings.MaximumDBm)
		screen.wfSpeedSlider.SetValue(float32(screen.waterfallSettings.LinesPerSecond))
		screen.refreshWaterfallControls()
		screen.waterfall.InvalidateColors()
		screen.markSettingsDirty()
	})
	closeButton.OnClick(func() { screen.selectTool("WATERFALL") })

	screen.waterfallControls = []simpleui.Element{
		screen.wfOffsetLabel, screen.wfOffsetSlider,
		screen.wfContrastLabel, screen.wfContrastSlider,
		screen.wfRangeLabel, screen.wfRangeSlider,
		screen.wfSpeedLabel, screen.wfSpeedSlider,
		screen.wfPaletteButton, reset, closeButton,
	}
	screen.refreshWaterfallControls()
}

func (screen *MainScreen) refreshWaterfallControls() {
	settings := screen.waterfallSettings
	offset := fmt.Sprintf("%d", settings.ColorOffsetDB)
	if settings.ColorOffsetDB > 0 {
		offset = "+" + offset
	}
	screen.wfOffsetLabel.SetText("COLOR OFFSET  " + offset + " dB")
	screen.wfContrastLabel.SetText(fmt.Sprintf("CONTRAST  %d %%", settings.Contrast))
	screen.wfRangeLabel.SetText(fmt.Sprintf("LEVEL  %.0f / %.0f dBm", settings.MinimumDBm, settings.MaximumDBm))
	screen.wfSpeedLabel.SetText(fmt.Sprintf("SPEED  %d lines/s", settings.LinesPerSecond))
	screen.wfPaletteButton.SetLabel("PALETTE  " + settings.Palette)
}

func (screen *MainScreen) setWaterfallControlsVisible(visible bool) {
	screen.waterfallVisible = visible
	for _, element := range screen.waterfallControls {
		element.SetVisible(visible)
	}
}

func (screen *MainScreen) selectTool(tool string) {
	previous := screen.activeTool
	if previous == "RADIOSONDE" && tool != "RADIOSONDE" && screen.radiosondePanel != nil {
		screen.radiosondePanel.Leave()
	}
	if previous == "AIS" && tool != "AIS" && screen.aisPanel != nil {
		screen.aisPanel.Leave()
	}
	if previous == "AIRCRAFT" && tool != "AIRCRAFT" && screen.aircraftPanel != nil {
		screen.aircraftPanel.Leave()
	}
	if previous == "RTL_433" && tool != "RTL_433" && screen.rtl433Panel != nil {
		screen.rtl433Panel.Leave()
	}
	if previous == "APRS" && tool != "APRS" && screen.aprsPanel != nil {
		screen.aprsPanel.Leave()
	}
	if previous == "SSTV" && tool != "SSTV" && screen.sstvPanel != nil {
		screen.sstvPanel.Leave()
	}
	if previous == "TETRA" && tool != "TETRA" && screen.tetraPanel != nil {
		screen.tetraPanel.Leave()
	}
	if previous == "SATELLITES" && tool != "SATELLITES" && screen.satellitePanel != nil {
		screen.satellitePanel.Leave()
	}
	// A tool can be selected while VIEW 2 is active and while the menu owns the
	// mouse release. Discard any gesture begun on the old geometry, then publish
	// the new tool before restoring VIEW 1 so visibility is calculated from the
	// new state in one pass.
	screen.draggingSpectrum = false
	if screen.scanPanel != nil {
		screen.scanPanel.dragTarget = 0
	}
	screen.activeTool = tool
	screen.setViewMode(1)
	if tool == "DMR_MONITOR" && screen.mode != nil && screen.mode.SelectedText() != "DMR BETA" {
		for index, item := range screen.mode.Items() {
			if item == "DMR BETA" {
				screen.mode.SetSelected(index)
				break
			}
		}
		screen.savedMode = "DMR BETA"
		screen.selectFilter(screen.filterSelector.Current("DMR BETA"))
	}
	if tool == "SSTV" && screen.mode != nil {
		current := screen.mode.SelectedText()
		if current != "USB" && current != "LSB" && current != "NFM" {
			for index, item := range screen.mode.Items() {
				if item == "USB" {
					screen.mode.SetSelected(index)
					break
				}
			}
			screen.savedMode = "USB"
			screen.selectFilter(screen.filterSelector.Current("USB"))
		}
	}
	if tool == "SCAN" && screen.scanPanel != nil {
		screen.scanPanel.Enter()
	}
	if screen.toolMenu != nil {
		screen.toolMenu.selected = tool
	}
	screen.setWaterfallControlsVisible(tool == "WATERFALL_ADJUST")
	if tool == "WATERFALL_ADJUST" {
		screen.wfOffsetSlider.SetValue(float32(screen.waterfallSettings.ColorOffsetDB))
		screen.wfContrastSlider.SetValue(float32(screen.waterfallSettings.Contrast))
		screen.wfRangeSlider.SetValues(screen.waterfallSettings.MinimumDBm, screen.waterfallSettings.MaximumDBm)
		screen.wfSpeedSlider.SetValue(float32(screen.waterfallSettings.LinesPerSecond))
		screen.refreshWaterfallControls()
	}
	if screen.fftDisplay != nil {
		if tool == "FFT" {
			screen.fftDisplay.Sync()
		}
		screen.fftDisplay.SetVisible(tool == "FFT")
	}
	if screen.audioPanel != nil {
		screen.audioPanel.SetVisible(tool == "PBT_AUDIO")
	}
	if screen.memoryPanel != nil {
		screen.memoryPanel.SetVisible(tool == "MEMORIES")
	}
	if screen.recorderPanel != nil {
		screen.recorderPanel.SetToolVisible(tool == "RECORDER")
	}
	if screen.dmrPanel != nil {
		screen.dmrPanel.SetVisible(tool == "DMR_MONITOR")
	}
	if screen.rtl433Panel != nil {
		screen.rtl433Panel.SetVisible(tool == "RTL_433")
		if tool == "RTL_433" && previous != "RTL_433" {
			screen.rtl433Panel.Enter()
		}
	}
	if screen.radiosondePanel != nil {
		screen.radiosondePanel.SetVisible(tool == "RADIOSONDE")
		if tool == "RADIOSONDE" && previous != "RADIOSONDE" {
			screen.radiosondePanel.Enter()
		}
	}
	if screen.aisPanel != nil {
		screen.aisPanel.SetVisible(tool == "AIS")
		if tool == "AIS" && previous != "AIS" {
			screen.aisPanel.Enter()
		}
	}
	if screen.aircraftPanel != nil {
		screen.aircraftPanel.SetVisible(tool == "AIRCRAFT")
		if tool == "AIRCRAFT" && previous != "AIRCRAFT" {
			screen.aircraftPanel.Enter()
		}
	}
	if screen.aprsPanel != nil {
		screen.aprsPanel.SetVisible(tool == "APRS")
		if tool == "APRS" && previous != "APRS" {
			screen.aprsPanel.Enter()
		}
	}
	if screen.sstvPanel != nil {
		screen.sstvPanel.SetVisible(tool == "SSTV")
		if tool == "SSTV" && previous != "SSTV" {
			screen.sstvPanel.Enter()
		}
	}
	if screen.tetraPanel != nil {
		screen.tetraPanel.SetVisible(tool == "TETRA")
		if tool == "TETRA" && previous != "TETRA" {
			screen.tetraPanel.Enter()
		}
	}
	if screen.satellitePanel != nil {
		screen.satellitePanel.SetVisible(tool == "SATELLITES")
		if tool == "SATELLITES" && previous != "SATELLITES" {
			screen.satellitePanel.Enter()
		}
	}
	screen.markSettingsDirty()
	screen.markSettingsDirty()
}

func (screen *MainScreen) cycleViewMode() {
	next := screen.viewMode + 1
	if next > 3 {
		next = 1
	}
	screen.setViewMode(next)
}

func (screen *MainScreen) setViewMode(mode int) {
	if mode < 1 || mode > 3 {
		mode = 1
	}
	screen.viewMode = mode
	if screen.viewButton != nil {
		screen.viewButton.SetLabel(fmt.Sprintf("VIEW %d", mode))
	}
	showTool := mode == 1
	if screen.waterfallControls != nil {
		screen.setWaterfallControlsVisible(showTool && screen.activeTool == "WATERFALL_ADJUST")
	}
	if screen.fftDisplay != nil {
		screen.fftDisplay.SetVisible(showTool && screen.activeTool == "FFT")
	}
	if screen.audioPanel != nil {
		screen.audioPanel.SetVisible(showTool && screen.activeTool == "PBT_AUDIO")
	}
	if screen.memoryPanel != nil {
		screen.memoryPanel.SetVisible(showTool && screen.activeTool == "MEMORIES")
	}
	if screen.recorderPanel != nil {
		screen.recorderPanel.SetToolVisible(showTool && screen.activeTool == "RECORDER")
	}
	if screen.dmrPanel != nil {
		screen.dmrPanel.SetVisible(showTool && screen.activeTool == "DMR_MONITOR")
	}
	if screen.rtl433Panel != nil {
		screen.rtl433Panel.SetVisible(showTool && screen.activeTool == "RTL_433")
	}
	if screen.radiosondePanel != nil {
		screen.radiosondePanel.SetVisible(showTool && screen.activeTool == "RADIOSONDE")
	}
	if screen.aisPanel != nil {
		screen.aisPanel.SetVisible(showTool && screen.activeTool == "AIS")
	}
	if screen.aircraftPanel != nil {
		screen.aircraftPanel.SetVisible(showTool && screen.activeTool == "AIRCRAFT")
	}
	if screen.aprsPanel != nil {
		screen.aprsPanel.SetVisible(showTool && screen.activeTool == "APRS")
	}
	if screen.sstvPanel != nil {
		screen.sstvPanel.SetVisible(showTool && screen.activeTool == "SSTV")
	}
	if screen.tetraPanel != nil {
		screen.tetraPanel.SetVisible(showTool && screen.activeTool == "TETRA")
	}
	if screen.satellitePanel != nil {
		screen.satellitePanel.SetVisible(showTool && screen.activeTool == "SATELLITES")
	}
	screen.markSettingsDirty()
}

func (screen *MainScreen) spectrumGeometry() (x, y, width, height float32) {
	x, y, width, height = toolContentX, 215, toolContentRight-toolContentX, 235
	if screen.viewMode == 2 {
		height = 470
	}
	return
}

func (screen *MainScreen) waterfallGeometry() (x, y, width, height float32) {
	x, y, width, height = toolContentX, 450, toolContentRight-toolContentX, 170
	if screen.viewMode == 2 {
		y, height = 685, 141
	}
	if screen.viewMode == 3 {
		height = 340
	}
	return
}

func (screen *MainScreen) selectBand(band BandDefinition) {
	screen.setFrequencyDigitExponent(-1)
	screen.bandCategory = band.Category
	screen.bandName = band.Name
	screen.band.SetLabel("BAND  " + band.Name)
	screen.selectMode(recommendedModeForBand(band))
	screen.frequencyHz = band.FrequencyHz
	screen.centerFrequencyHz = band.FrequencyHz
	screen.spanHz = band.SpanHz
	screen.tuningStepHz = recommendedStepForBand(band)
	if screen.step != nil {
		screen.step.SetLabel("STEP  " + formatStep(screen.tuningStepHz))
	}
	if screen.stepSelector != nil {
		screen.stepSelector.SetSelected(screen.tuningStepHz)
	}
	screen.waterfall.Reset()
	if screen.receiver != nil {
		screen.receiver.SetCenterFrequency(band.FrequencyHz)
		screen.receiver.SetDemodulator(screen.mode.SelectedText(), screen.frequencyHz, screen.demodBandwidthHz)
	}
	screen.markSettingsDirty()
}

func (screen *MainScreen) selectTuningStep(stepHz int64) {
	if stepHz <= 0 {
		return
	}
	screen.setFrequencyDigitExponent(-1)
	screen.tuningStepHz = stepHz
	screen.frequencyHz = max(int64(math.Round(float64(screen.frequencyHz)/float64(stepHz)))*stepHz, 1_000)
	centerChanged := false
	if screen.centerMode {
		centerChanged = screen.centerFrequencyHz != screen.frequencyHz
		screen.centerFrequencyHz = screen.frequencyHz
	} else {
		halfSpan := screen.spanHz / 2
		screen.frequencyHz = min(max(screen.frequencyHz, screen.centerFrequencyHz-halfSpan), screen.centerFrequencyHz+halfSpan)
	}
	if screen.step != nil {
		screen.step.SetLabel("STEP  " + formatStep(stepHz))
	}
	if screen.receiver != nil {
		if centerChanged {
			screen.receiver.SetCenterFrequency(screen.centerFrequencyHz)
		}
		screen.receiver.SetDemodulator(screen.mode.SelectedText(), screen.frequencyHz, screen.demodBandwidthHz)
	}
	screen.markSettingsDirty()
}

func (screen *MainScreen) digitStepHz() int64 {
	if screen.frequencyDigitExponent < 0 {
		return 0
	}
	step := int64(1)
	for exponent := 0; exponent < screen.frequencyDigitExponent; exponent++ {
		step *= 10
	}
	return step
}

func (screen *MainScreen) activeTuningStepHz() int64 {
	if step := screen.digitStepHz(); step > 0 {
		return step
	}
	return screen.tuningStepHz
}

func (screen *MainScreen) setFrequencyDigitExponent(exponent int) {
	if exponent == screen.frequencyDigitExponent {
		exponent = -1
	}
	screen.frequencyDigitExponent = exponent
	if exponent >= 0 {
		// Digit tuning is a fine VFO adjustment: preserve the RF scope in FIX.
		screen.centerMode = false
		if screen.vfoModeSwitch != nil {
			screen.vfoModeSwitch.SetActive(true)
		}
		if screen.step != nil {
			screen.step.SetLabel("DIGIT STEP\n" + formatStep(screen.digitStepHz()))
			screen.step.SetColors(rl.Color{R: 25, G: 125, B: 190, A: 255}, rl.Color{R: 125, G: 205, B: 255, A: 255}, rl.White)
		}
	} else if screen.step != nil {
		screen.step.SetLabel("STEP  " + formatStep(screen.tuningStepHz))
		screen.step.ClearColors()
	}
	screen.markSettingsDirty()
}

func (screen *MainScreen) updateFrequencyInteraction() {
	if screen.overlayOpen() {
		return
	}
	mouse := simpleui.MousePosition()
	over := mouse.X >= frequencyDialX && mouse.X <= frequencyPanelX+frequencyPanelW &&
		mouse.Y >= frequencyPanelY && mouse.Y <= frequencyPanelY+frequencyPanelH
	hoveredDigit := false
	if over && mouse.Y >= frequencyPanelY+20 && mouse.Y <= frequencyPanelY+72 {
		if exponent, ok := frequencyDigitExponentAt(mouse.X, formatDialFrequency(screen.frequencyHz)); ok {
			hoveredDigit = true
			screen.setFrequencyDigitHover(exponent)
		}
	}
	if !hoveredDigit {
		screen.setFrequencyDigitHover(-1)
		return
	}
	if steps := wheelSteps(rl.GetMouseWheelMove()); steps != 0 {
		// Digit hover becomes an active fine-tuning gesture only when the wheel
		// moves. Merely crossing the display must not change CENTER/FIX.
		screen.centerMode = false
		if screen.vfoModeSwitch != nil {
			screen.vfoModeSwitch.SetActive(true)
		}
		centerChanged := screen.tuneFixedBySteps(steps)
		if screen.receiver != nil {
			if centerChanged {
				screen.receiver.SetCenterFrequency(screen.centerFrequencyHz)
			}
			screen.receiver.SetDemodulator(screen.mode.SelectedText(), screen.frequencyHz, screen.demodBandwidthHz)
		}
	}
}

func (screen *MainScreen) setFrequencyDigitHover(exponent int) {
	if exponent == screen.frequencyDigitExponent {
		return
	}
	screen.frequencyDigitExponent = exponent
	if screen.step == nil {
		return
	}
	if exponent >= 0 {
		screen.step.SetLabel("DIGIT STEP\n" + formatStep(screen.digitStepHz()))
		screen.step.SetColors(rl.Color{R: 25, G: 125, B: 190, A: 255}, rl.Color{R: 125, G: 205, B: 255, A: 255}, rl.White)
	} else {
		screen.step.SetLabel("STEP  " + formatStep(screen.tuningStepHz))
		screen.step.ClearColors()
	}
}

func frequencyDigitExponentAt(mouseX float32, formatted string) (int, bool) {
	totalWidth := simpleui.MeasureTextStyled(formatted, frequencyFontSize, simpleui.FontMono).X
	cursor := frequencyDialX + (frequencyDialW-totalWidth)*.5
	digitCount := countFrequencyDigits(formatted)
	seenDigits := 0
	for _, character := range formatted {
		text := string(character)
		width := simpleui.MeasureTextStyled(text, frequencyFontSize, simpleui.FontMono).X
		if character >= '0' && character <= '9' {
			exponent := digitCount - seenDigits - 1
			seenDigits++
			if mouseX >= cursor-3 && mouseX <= cursor+width+3 {
				return exponent, true
			}
		}
		cursor += width
	}
	return 0, false
}

func recommendedStepForBand(band BandDefinition) int64 {
	switch band.Category {
	case "HAM":
		switch band.Name {
		case "23 cm":
			return 25_000
		case "4 m", "2 m", "70 cm":
			return 12_500
		default:
			return 100
		}
	case "COMMERCIAL":
		switch band.Name {
		case "FM", "DAB":
			return 100_000
		case "AIR":
			return 12_500
		case "MARINE":
			return 25_000
		default:
			return 1_000
		}
	case "ISM":
		switch band.Name {
		case "CB 27":
			return 10_000
		case "PMR446":
			return 6_250
		case "433 MHz", "868 MHz", "915 MHz":
			return 25_000
		case "2.4 GHz":
			return 100_000
		}
	}
	return 1_000
}

func (screen *MainScreen) selectMode(mode string) {
	for index, item := range screen.mode.Items() {
		if item == mode {
			screen.mode.SetSelected(index)
			screen.savedMode = mode
			screen.selectFilter(screen.filterSelector.Current(mode))
			screen.markSettingsDirty()
			return
		}
	}
}

func recommendedModeForBand(band BandDefinition) string {
	if band.Category == "HAM" {
		switch band.Name {
		case "160 m", "80 m", "40 m":
			return "LSB"
		case "4 m", "2 m", "70 cm", "23 cm":
			return "NFM"
		default:
			return "USB"
		}
	}
	if band.Category == "COMMERCIAL" {
		switch band.Name {
		case "FM", "DAB":
			return "WFM"
		case "MARINE", "RADIOSONDES":
			return "NFM"
		default:
			return "AM"
		}
	}
	if band.Name == "CB 27" {
		return "AM"
	}
	return "NFM"
}

func (screen *MainScreen) selectFilter(preset FilterPreset) {
	screen.demodBandwidthHz = preset.BandwidthHz
	if screen.filter != nil {
		screen.filter.SetLabel(filterButtonLabel(preset))
	}
	if screen.receiver != nil {
		screen.receiver.SetDemodulator(screen.mode.SelectedText(), screen.frequencyHz, screen.demodBandwidthHz)
	}
	screen.markSettingsDirty()
}

func filterButtonLabel(preset FilterPreset) string {
	name := preset.ID
	if name == "CUSTOM" {
		name = "FIL4"
	}
	return name + "  " + formatFilterBandwidth(preset.BandwidthHz)
}

func toolDisplayName(tool string) string {
	for _, item := range toolMenuItems {
		if item.id == tool {
			return item.label
		}
	}
	return tool
}

func (screen *MainScreen) changeSpan(direction int) {
	spans := []int64{50_000, 100_000, 250_000, 500_000, 1_000_000, 2_000_000}
	previousSpan := screen.spanHz
	index := len(spans) - 1
	for current, span := range spans {
		if span == screen.spanHz {
			index = current
			break
		}
	}
	index += direction
	if index < 0 {
		index = 0
	}
	if index >= len(spans) {
		index = len(spans) - 1
	}
	screen.spanHz = spans[index]
	if screen.spanHz == previousSpan {
		return
	}

	// A span change always recenters the capture on the current VFO without
	// changing the tuned frequency or the selected CENTER/FIX interaction mode.
	screen.centerFrequencyHz = screen.frequencyHz
	screen.draggingSpectrum = false
	if screen.waterfall != nil {
		screen.waterfall.Reset()
	}
	if screen.receiver != nil {
		screen.receiver.SetCenterFrequency(screen.centerFrequencyHz)
		screen.receiver.SetDemodulator(screen.mode.SelectedText(), screen.frequencyHz, screen.demodBandwidthHz)
	}
	screen.markSettingsDirty()
}

func (screen *MainScreen) updateSpectrumDrag() {
	if screen.overlayOpen() {
		screen.draggingSpectrum = false
		return
	}
	x, y, width, height := screen.spectrumGeometry()
	mouse := simpleui.MousePosition()
	over := mouse.X >= x && mouse.X <= x+width && mouse.Y >= y && mouse.Y <= y+height
	wheel := rl.GetMouseWheelMove()
	if screen.rtl433Panel != nil && screen.rtl433Panel.HandleSpectrumInput(mouse, x, y, width, height) {
		screen.draggingSpectrum = false
		return
	}
	if screen.scanPanel != nil && screen.scanPanel.ConsumesSpectrumInput() && over {
		// Direct tuning always wins over an automatic scan. Previously a running
		// scanner swallowed every wheel/click event, which looked like a frozen
		// FFT after leaving VIEW 2 or changing tools.
		if screen.scanPanel.running && (wheel != 0 || rl.IsMouseButtonPressed(rl.MouseButtonLeft)) && screen.scanPanel.dragTarget == 0 {
			screen.scanPanel.ToggleRunning()
		} else {
			screen.draggingSpectrum = false
			return
		}
	}
	if rl.IsMouseButtonPressed(rl.MouseButtonLeft) && over && screen.memoryPanel != nil && screen.memoryPanel.HandleMarkerClick(mouse, x, y, width, height) {
		return
	}
	if over {
		if steps := wheelSteps(wheel); steps != 0 {
			previousFrequency := screen.frequencyHz
			if screen.centerMode {
				screen.tuneCenteredBySteps(steps)
				if screen.receiver != nil {
					screen.receiver.SetCenterFrequency(screen.centerFrequencyHz)
					screen.receiver.SetDemodulator(screen.mode.SelectedText(), screen.frequencyHz, screen.demodBandwidthHz)
				}
			} else {
				centerChanged := screen.tuneFixedBySteps(steps)
				if screen.receiver != nil {
					if centerChanged {
						screen.receiver.SetCenterFrequency(screen.centerFrequencyHz)
					}
					screen.receiver.SetDemodulator(screen.mode.SelectedText(), screen.frequencyHz, screen.demodBandwidthHz)
				}
			}
			screen.resetTETRAAfterManualSpectrumTune(previousFrequency)
		}
	}
	if rl.IsMouseButtonPressed(rl.MouseButtonLeft) && over {
		screen.draggingSpectrum = true
		screen.dragStartX = mouse.X
		screen.dragStartCenterHz = screen.centerFrequencyHz
		screen.dragStartTunedHz = screen.frequencyHz
		screen.nextDragRetune = 0
	}
	released := rl.IsMouseButtonReleased(rl.MouseButtonLeft)
	// In FIX mode a short click selects the RF position under the pointer while
	// the IQ capture (and therefore the FFT) remains stationary. A real drag
	// keeps the existing spectrum-pan behaviour.
	if screen.draggingSpectrum && released && !screen.centerMode && float32(math.Abs(float64(mouse.X-screen.dragStartX))) < 6 {
		previousFrequency := screen.frequencyHz
		fraction := min(max((mouse.X-x)/width, 0), 1)
		screen.tuneFixedAtFraction(fraction)
		screen.resetTETRAAfterManualSpectrumTune(previousFrequency)
		screen.draggingSpectrum = false
		if screen.receiver != nil {
			screen.receiver.SetDemodulator(screen.mode.SelectedText(), screen.frequencyHz, screen.demodBandwidthHz)
		}
		return
	}
	if screen.draggingSpectrum && (rl.IsMouseButtonDown(rl.MouseButtonLeft) || released) {
		deltaHz := int64(math.Round(float64(-(mouse.X - screen.dragStartX) / width * float32(screen.spanHz))))
		// A 1 kHz quantization avoids flooding the hardware with insignificant retunes.
		nextCenter := ((screen.dragStartCenterHz + deltaHz) / 1_000) * 1_000
		screen.centerFrequencyHz = max(nextCenter, 1_000)
		if screen.centerMode {
			screen.frequencyHz = screen.centerFrequencyHz
		} else {
			screen.frequencyHz = screen.dragStartTunedHz
		}
		screen.markSettingsDirty()
		now := rl.GetTime()
		// Match IC-SDR: the receiver coalesces retunes while IQ reads continue.
		if screen.receiver != nil && (released || now >= screen.nextDragRetune) {
			screen.receiver.SetCenterFrequency(screen.centerFrequencyHz)
			screen.receiver.SetDemodulator(screen.mode.SelectedText(), screen.frequencyHz, screen.demodBandwidthHz)
			screen.nextDragRetune = now + 1.0/60.0
		}
	}
	if screen.draggingSpectrum && released {
		if screen.centerMode {
			screen.resetTETRAAfterManualSpectrumTune(screen.dragStartTunedHz)
		}
		screen.draggingSpectrum = false
	}
}

func (screen *MainScreen) resetTETRAAfterManualSpectrumTune(previousFrequency int64) {
	if screen.frequencyHz == previousFrequency || screen.activeTool != "TETRA" || screen.tetraPanel == nil {
		return
	}
	screen.tetraPanel.resetAfterManualTune()
}

func (screen *MainScreen) tuneFixedAtFraction(fraction float32) {
	fraction = min(max(fraction, 0), 1)
	halfSpan := screen.spanHz / 2
	clickedHz := screen.centerFrequencyHz - halfSpan + int64(math.Round(float64(fraction*float32(screen.spanHz))))
	if screen.tuningStepHz > 0 {
		clickedHz = int64(math.Round(float64(clickedHz)/float64(screen.tuningStepHz))) * screen.tuningStepHz
	}
	screen.frequencyHz = max(clickedHz, 1_000)
	screen.markSettingsDirty()
}

func wheelSteps(wheel float32) int64 {
	if wheel > 0 {
		return max(int64(math.Round(float64(wheel))), 1)
	}
	if wheel < 0 {
		return min(int64(math.Round(float64(wheel))), -1)
	}
	return 0
}

func (screen *MainScreen) tuneCenteredBySteps(steps int64) {
	stepHz := screen.activeTuningStepHz()
	if steps == 0 || stepHz <= 0 {
		return
	}
	nextFrequency := screen.frequencyHz + steps*stepHz
	nextFrequency = max((nextFrequency/stepHz)*stepHz, 1_000)
	screen.frequencyHz = nextFrequency
	screen.centerFrequencyHz = nextFrequency
	screen.markSettingsDirty()
}

// tuneFixedBySteps follows the original IC-SDR FIX behaviour: the VFO marker
// moves through the stationary spectrum and the IQ center only pans when the
// marker approaches an edge.
func (screen *MainScreen) tuneFixedBySteps(steps int64) bool {
	stepHz := screen.activeTuningStepHz()
	if steps == 0 || screen.spanHz <= 0 || stepHz <= 0 {
		return false
	}

	nextFrequency := screen.frequencyHz + steps*stepHz
	nextFrequency = max((nextFrequency/stepHz)*stepHz, 1_000)

	halfSpan := screen.spanHz / 2
	passbandGuard := int64(screen.demodBandwidthHz / 2)
	mode := ""
	if screen.mode != nil {
		mode = screen.mode.SelectedText()
	}
	if mode == "USB" || mode == "LSB" {
		passbandGuard = int64(screen.demodBandwidthHz)
	}
	guardHz := max(int64(math.Round(float64(screen.spanHz)*0.05)), passbandGuard)
	guardHz = min(guardHz, int64(math.Round(float64(screen.spanHz)*0.40)))
	lowerGuard := screen.centerFrequencyHz - halfSpan + guardHz
	upperGuard := screen.centerFrequencyHz + halfSpan - guardHz

	previousCenter := screen.centerFrequencyHz
	screen.frequencyHz = nextFrequency
	if screen.frequencyHz < lowerGuard {
		screen.centerFrequencyHz += screen.frequencyHz - lowerGuard
	} else if screen.frequencyHz > upperGuard {
		screen.centerFrequencyHz += screen.frequencyHz - upperGuard
	}
	screen.centerFrequencyHz = max(screen.centerFrequencyHz, halfSpan)
	screen.markSettingsDirty()
	return screen.centerFrequencyHz != previousCenter
}

func (screen *MainScreen) overlayOpen() bool {
	return simpleui.PointerInputBlocked() ||
		(screen.toolMenu != nil && screen.toolMenu.OverlayOpen()) ||
		(screen.bandSelector != nil && screen.bandSelector.OverlayOpen()) ||
		(screen.sdrSettings != nil && screen.sdrSettings.OverlayOpen()) ||
		(screen.stepSelector != nil && screen.stepSelector.OverlayOpen()) ||
		(screen.filterSelector != nil && screen.filterSelector.OverlayOpen()) ||
		(screen.memoryPanel != nil && screen.memoryPanel.OverlayOpen())
}

func (screen *MainScreen) visibleBin(normalized float32) (int, bool) {
	if screen.stats.SampleRate <= 0 {
		return 0, false
	}
	visibleOffset := (float64(normalized) - .5) * float64(screen.spanHz) / screen.stats.SampleRate
	position := .5 + visibleOffset
	if position < 0 || position >= 1 {
		return 0, false
	}
	bin := int(position * float64(len(screen.spectrum)))
	return min(bin, len(screen.spectrum)-1), true
}

// interpolatedSpectrumValue resamples the visible FFT to screen pixels. At
// narrow spans several pixels fall between adjacent FFT bins; interpolation
// avoids repeating bins as flat horizontal steps without changing DSP data.
func (screen *MainScreen) interpolatedSpectrumValue(spectrum []float32, normalized float32) (float32, bool) {
	if screen.stats.SampleRate <= 0 || len(spectrum) == 0 {
		return 0, false
	}
	visibleOffset := (float64(normalized) - .5) * float64(screen.spanHz) / screen.stats.SampleRate
	position := (.5 + visibleOffset) * float64(len(spectrum))
	if position < 0 || position > float64(len(spectrum)-1) {
		return 0, false
	}
	left := int(math.Floor(position))
	right := min(left+1, len(spectrum)-1)
	fraction := float32(position - float64(left))
	return spectrum[left] + (spectrum[right]-spectrum[left])*fraction, true
}

func drawPanel(x, y, width, height float32) {
	rectangle := rl.Rectangle{X: x, Y: y, Width: width, Height: height}
	rl.DrawRectangleRounded(rectangle, .035, 6, colors.panel)
	rl.DrawRectangleRoundedLinesEx(rectangle, .035, 6, 1, colors.border)
}

func drawGrid(x, y, width, height float32, columns, rows int) {
	for index := 1; index < columns; index++ {
		px := x + width*float32(index)/float32(columns)
		rl.DrawLine(int32(px), int32(y), int32(px), int32(y+height), colors.grid)
	}
	for index := 1; index < rows; index++ {
		py := y + height*float32(index)/float32(rows)
		rl.DrawLine(int32(x), int32(py), int32(x+width), int32(py), colors.grid)
	}
}

func drawSidebarSection(x, y, width, height float32, title string, accent rl.Color) {
	drawPanel(x, y, width, height)
	rl.DrawRectangleRounded(rl.Rectangle{X: x, Y: y, Width: 4, Height: height}, .5, 4, accent)
	drawSmallText(title, x+16, y+10, accent)
}

func drawStatusDot(x, y float32, active bool, activeText, inactiveText string) {
	color := rl.Color{R: 80, G: 85, B: 90, A: 255}
	text := inactiveText
	if active {
		color = colors.green
		text = activeText
	}
	rl.DrawCircle(int32(x), int32(y), 4, color)
	size := simpleui.MeasureText(text, 8)
	simpleui.DrawText(text, x-10-size.X, y-5, 8, color)
}

func drawActivity(x, y float32, name string, active bool, detail string) {
	color := rl.Color{R: 80, G: 85, B: 90, A: 255}
	if active {
		color = colors.green
	}
	rl.DrawCircle(int32(x), int32(y), 4, color)
	simpleui.DrawText(name, x+14, y-6, 10, colors.text)
	width := simpleui.MeasureText(detail, 8).X
	simpleui.DrawText(detail, 1570-width, y-5, 8, colors.muted)
}

func drawLevelMeter(x, y, width, height float32) {
	drawPanel(x, y, width, height)
	railY := y + 10
	rl.DrawLine(int32(x+8), int32(railY), int32(x+width-8), int32(railY), rl.Color{R: 58, G: 68, B: 80, A: 255})
	marks := []int{-60, -45, -30, -15, 0}
	for index, mark := range marks {
		px := x + 8 + (width-16)*float32(index)/float32(len(marks)-1)
		rl.DrawLine(int32(px), int32(railY-4), int32(px), int32(railY+4), colors.muted)
		text := fmt.Sprint(mark)
		textWidth := simpleui.MeasureText(text, 7).X
		simpleui.DrawText(text, px-textWidth*.5, y+16, 7, colors.muted)
	}
}

func drawSmallText(text string, x, y float32, color rl.Color) {
	simpleui.DrawText(text, x, y, 12, color)
}

func (screen *MainScreen) spectrumY(db float32, y, height float32) float32 {
	value := max(screen.spectrumMinimumDB, min(screen.spectrumMaximumDB, db))
	return y + 5 + (screen.spectrumMaximumDB-value)/(screen.spectrumMaximumDB-screen.spectrumMinimumDB)*(height-5)
}
