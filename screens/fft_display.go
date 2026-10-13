package screens

import (
	"go-zero/internal/i18n"

	"fmt"

	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// FFTDisplay is the live FFT/waterfall control surface from the original
// IC-SDR. The controls change the running DSP, not merely their labels.
type FFTDisplay struct {
	screen   *MainScreen
	controls []simpleui.Element

	averagingMs, refreshFPS  int
	peakHold                 bool
	peakDecay                float32
	window                   string
	peakSpectrum             []float32
	lastPeakUpdate           float64
	peakCenterHz, peakSpanHz int64

	averageLabel, refreshLabel, decayLabel, speedLabel, contrastLabel *simpleui.Label
	fftFloorLabel, fftFloorHelp, fftCeilingLabel, fftCeilingHelp      *simpleui.Label
	waterfallRangeLabel                                               *simpleui.Label
	average, refresh, decay, speed, contrast                          *simpleui.Slider
	fftFloor, fftCeiling                                              *simpleui.Slider
	peak                                                              *simpleui.Switch
	windowButton, paletteButton                                       *simpleui.Button
	waterfallRange                                                    *simpleui.RangeSlider
}

func NewFFTDisplay(screen *MainScreen) *FFTDisplay {
	panel := &FFTDisplay{screen: screen, averagingMs: screen.fftAveragingMs, refreshFPS: screen.fftRefreshFPS, peakHold: screen.fftPeakHold, peakDecay: screen.fftPeakDecay, window: screen.fftWindow}
	label := func(id string, x, y, width float32, text string) *simpleui.Label {
		value := simpleui.NewLabel(id, x, y, width, 18, text, 12)
		value.SetAlignment(simpleui.AlignCenter)
		return value
	}
	panel.averageLabel = label("fftAverageLabel", 60, 724, 260, "")
	panel.average = simpleui.NewSlider("fftAverage", 70, 755, 240, 20, 10, 300, float32(panel.averagingMs))
	panel.average.SetStep(1)
	panel.refreshLabel = label("fftRefreshLabel", 60, 794, 260, "")
	panel.refresh = simpleui.NewSlider("fftRefresh", 70, 825, 240, 20, 5, 60, float32(panel.refreshFPS))
	panel.refresh.SetStep(1)
	panel.peak = simpleui.NewSwitch("fftPeak", 375, 724, 250, 28, i18n.Source("text.72a3162cd4ec"), panel.peakHold, 12)
	panel.decayLabel = label("fftDecayLabel", 375, 766, 250, i18n.Source("text.ab8cb66fa1d4"))
	panel.decay = simpleui.NewSlider("fftDecay", 385, 798, 230, 18, 1, 10, panel.peakDecay)
	panel.decay.SetStep(1)
	panel.windowButton = simpleui.NewButton("fftWindow", 375, 825, 170, 36, i18n.Source("text.c65a97b8b4fe"), 10)
	panel.windowButton.SetColors(mixColor(colors.panelAlt, colors.cyan, .14), colors.cyan, colors.text)
	size := label("fftSize", 550, 825, 80, i18n.Source("text.847683cb656d"))
	panel.fftFloorLabel = label("fftFloorLabel", 720, 724, 230, "")
	panel.fftFloorHelp = simpleui.NewLabel("fftFloorHelp", 720, 748, 230, 18, i18n.Source("text.1747791c89a1"), 9)
	panel.fftFloorHelp.SetAlignment(simpleui.AlignCenter)
	panel.fftFloor = simpleui.NewSlider("fftFloor", 730, 774, 210, 18, -140, 10, screen.spectrumMinimumDB)
	panel.fftFloor.SetStep(1)
	panel.fftCeilingLabel = label("fftCeilingLabel", 720, 806, 230, "")
	panel.fftCeilingHelp = simpleui.NewLabel("fftCeilingHelp", 720, 828, 230, 18, i18n.Source("text.7f0a441cc6dc"), 9)
	panel.fftCeilingHelp.SetAlignment(simpleui.AlignCenter)
	panel.fftCeiling = simpleui.NewSlider("fftCeiling", 730, 850, 210, 18, -130, 20, screen.spectrumMaximumDB)
	panel.fftCeiling.SetStep(1)

	panel.speedLabel = label("fftWaterfallSpeedLabel", 1000, 724, 230, i18n.Source("text.bd5876221bce"))
	panel.speed = simpleui.NewSlider("fftWaterfallSpeed", 1010, 755, 210, 20, 5, 60, 15)
	panel.speed.SetStep(1)
	panel.contrastLabel = label("fftWaterfallContrastLabel", 1000, 794, 230, i18n.Source("text.e9ec9039db61"))
	panel.contrast = simpleui.NewSlider("fftWaterfallContrast", 1010, 825, 210, 20, 25, 200, 102)
	panel.contrast.SetStep(1)
	panel.waterfallRangeLabel = label("fftWaterfallRangeLabel", 1280, 724, 250, i18n.Source("text.460e892c8110"))
	panel.waterfallRange = simpleui.NewRangeSlider("fftWaterfallRange", 1290, 755, 230, 18, -120, 0, -60, 0)
	panel.waterfallRange.SetMinimumGap(10)
	panel.waterfallRange.SetRangeDragging(false)
	panel.paletteButton = simpleui.NewButton("fftPalette", 1280, 795, 120, 38, i18n.Source("text.40e4bc6977aa"), 10)
	reset := simpleui.NewButton("fftWaterfallReset", 1410, 795, 110, 38, i18n.Source("text.7ef2fad58d1f"), 10)
	adjust := simpleui.NewButton("fftWaterfallAdjust", 1280, 840, 240, 30, i18n.Source("text.b0da66f86167"), 10)
	panel.paletteButton.SetColors(mixColor(colors.panelAlt, colors.blue, .18), colors.blue, colors.text)
	reset.SetColors(mixColor(colors.panelAlt, colors.orange, .15), colors.orange, colors.text)
	adjust.SetColors(mixColor(colors.panelAlt, colors.cyan, .14), colors.cyan, colors.text)

	panel.average.OnChange(func(value float32) {
		panel.averagingMs = int(value)
		screen.fftAveragingMs = panel.averagingMs
		if screen.receiver != nil {
			screen.receiver.SetSpectrumAveraging(panel.averagingMs)
		}
		panel.refreshLabels()
		screen.markSettingsDirty()
	})
	panel.refresh.OnChange(func(value float32) {
		panel.refreshFPS = int(value)
		screen.fftRefreshFPS = panel.refreshFPS
		panel.refreshLabels()
		screen.markSettingsDirty()
	})
	panel.peak.OnChange(func(value bool) {
		panel.peakHold = value
		screen.fftPeakHold = value
		panel.decay.SetEnabled(value)
		screen.markSettingsDirty()
	})
	panel.decay.OnChange(func(value float32) {
		panel.peakDecay = value
		screen.fftPeakDecay = value
		panel.refreshLabels()
		screen.markSettingsDirty()
	})
	panel.windowButton.OnClick(panel.cycleWindow)
	panel.fftFloor.OnChange(func(value float32) {
		value = min(value, screen.spectrumMaximumDB-10)
		panel.fftFloor.SetValue(value)
		screen.spectrumMinimumDB = value
		panel.refreshLabels()
		screen.markSettingsDirty()
	})
	panel.fftCeiling.OnChange(func(value float32) {
		value = max(value, screen.spectrumMinimumDB+10)
		panel.fftCeiling.SetValue(value)
		screen.spectrumMaximumDB = value
		panel.refreshLabels()
		screen.markSettingsDirty()
	})
	panel.speed.OnChange(func(value float32) {
		screen.waterfallSettings.LinesPerSecond = int(value)
		panel.refreshLabels()
		screen.markSettingsDirty()
	})
	panel.contrast.OnChange(func(value float32) {
		screen.waterfallSettings.Contrast = int(value)
		screen.waterfall.InvalidateColors()
		panel.refreshLabels()
		screen.markSettingsDirty()
	})
	panel.waterfallRange.OnChange(func(low, high float32) {
		screen.waterfallSettings.MinimumDBm, screen.waterfallSettings.MaximumDBm = low, high
		screen.waterfall.InvalidateColors()
		panel.refreshLabels()
		screen.markSettingsDirty()
	})
	panel.paletteButton.OnClick(panel.cyclePalette)
	reset.OnClick(panel.resetWaterfall)
	adjust.OnClick(func() { screen.selectTool(i18n.Source("text.9b6bb9932898")) })

	panel.controls = []simpleui.Element{panel.averageLabel, panel.average, panel.refreshLabel, panel.refresh,
		panel.peak, panel.decayLabel, panel.decay, panel.windowButton, size,
		panel.fftFloorLabel, panel.fftFloorHelp, panel.fftFloor, panel.fftCeilingLabel, panel.fftCeilingHelp, panel.fftCeiling,
		panel.speedLabel, panel.speed, panel.contrastLabel, panel.contrast, panel.waterfallRangeLabel,
		panel.waterfallRange, panel.paletteButton, reset, adjust}
	panel.SetVisible(false)
	panel.refreshLabels()
	panel.decay.SetEnabled(panel.peakHold)
	if screen.receiver != nil {
		screen.receiver.SetSpectrumAveraging(panel.averagingMs)
		screen.receiver.SetFFTWindow(panel.window)
	}
	return panel
}

func (panel *FFTDisplay) SetVisible(visible bool) {
	for _, control := range panel.controls {
		control.SetVisible(visible)
	}
}

func (panel *FFTDisplay) Sync() {
	panel.averagingMs = panel.screen.fftAveragingMs
	panel.refreshFPS = panel.screen.fftRefreshFPS
	panel.peakHold = panel.screen.fftPeakHold
	panel.peakDecay = panel.screen.fftPeakDecay
	panel.window = panel.screen.fftWindow
	panel.average.SetValue(float32(panel.averagingMs))
	panel.refresh.SetValue(float32(panel.refreshFPS))
	panel.peak.SetActive(panel.peakHold)
	panel.decay.SetValue(panel.peakDecay)
	panel.decay.SetEnabled(panel.peakHold)
	panel.speed.SetValue(float32(panel.screen.waterfallSettings.LinesPerSecond))
	panel.contrast.SetValue(float32(panel.screen.waterfallSettings.Contrast))
	panel.waterfallRange.SetValues(panel.screen.waterfallSettings.MinimumDBm, panel.screen.waterfallSettings.MaximumDBm)
	panel.fftFloor.SetValue(panel.screen.spectrumMinimumDB)
	panel.fftCeiling.SetValue(panel.screen.spectrumMaximumDB)
	panel.refreshLabels()
}

func (panel *FFTDisplay) DrawPanel() {
	drawGuidedHeader(i18n.Source("text.fftGuideTitle"), i18n.Source("text.fftGuideIntro"))
	drawGuidedControlCard(rl.Rectangle{X: 40, Y: 665, Width: 300, Height: 210}, i18n.Source("text.fftStabilityTitle"), i18n.Source("text.fftStabilityHelp"), colors.cyan)
	drawGuidedControlCard(rl.Rectangle{X: 350, Y: 665, Width: 300, Height: 210}, i18n.Source("text.fftPeaksTitle"), i18n.Source("text.fftPeaksHelp"), colors.orange)
	drawGuidedControlCard(rl.Rectangle{X: 700, Y: 665, Width: 270, Height: 210}, i18n.Source("text.fftScaleTitle"), i18n.Source("text.fftScaleHelp"), colors.green)
	drawGuidedControlCard(rl.Rectangle{X: 980, Y: 665, Width: 270, Height: 210}, i18n.Source("text.fftWaterfallTitle"), i18n.Source("text.fftWaterfallHelp"), colors.blue)
	drawGuidedControlCard(rl.Rectangle{X: 1260, Y: 665, Width: 300, Height: 210}, i18n.Source("text.fftColorsTitle"), i18n.Source("text.fftColorsHelp"), colors.orange)
}

func (panel *FFTDisplay) UpdatePeaks(spectrum []float32) {
	now := rl.GetTime()
	centerHz := panel.screen.stats.SpectrumCenterHz
	spanHz := panel.screen.spanHz
	if centerHz != panel.peakCenterHz || spanHz != panel.peakSpanHz {
		panel.peakSpectrum = append(panel.peakSpectrum[:0], spectrum...)
		panel.peakCenterHz, panel.peakSpanHz = centerHz, spanHz
		panel.lastPeakUpdate = now
		return
	}
	if len(panel.peakSpectrum) != len(spectrum) {
		panel.peakSpectrum = append([]float32(nil), spectrum...)
		panel.lastPeakUpdate = now
		return
	}
	if !panel.peakHold {
		copy(panel.peakSpectrum, spectrum)
		panel.lastPeakUpdate = now
		return
	}
	decay := panel.peakDecay * float32(max(now-panel.lastPeakUpdate, 0))
	for index, value := range spectrum {
		panel.peakSpectrum[index] = max(value, panel.peakSpectrum[index]-decay)
	}
	panel.lastPeakUpdate = now
}

func (panel *FFTDisplay) cycleWindow() {
	windows := []string{i18n.Source("text.26701b540b4b"), i18n.Source("text.dbcf1c5bae70"), i18n.Source("text.18012268eaac"), i18n.Source("text.2ac455cdbd57")}
	index := indexOf(windows, panel.window)
	panel.window = windows[(index+1)%len(windows)]
	panel.screen.fftWindow = panel.window
	if panel.screen.receiver != nil {
		panel.screen.receiver.SetFFTWindow(panel.window)
	}
	panel.refreshLabels()
	panel.screen.markSettingsDirty()
}

func (panel *FFTDisplay) cyclePalette() {
	palettes := []string{i18n.Source("text.24a866f4940f"), i18n.Source("text.ddacfc88b465"), i18n.Source("text.f27f17e3f063"), i18n.Source("text.2d71cca47c3a")}
	index := indexOf(palettes, panel.screen.waterfallSettings.Palette)
	panel.screen.waterfallSettings.Palette = palettes[(index+1)%len(palettes)]
	panel.screen.waterfall.InvalidateColors()
	panel.refreshLabels()
	panel.screen.markSettingsDirty()
}

func (panel *FFTDisplay) resetWaterfall() {
	settings := factoryWaterfallSettings()
	panel.screen.waterfallSettings = settings
	panel.screen.waterfall.settings = &panel.screen.waterfallSettings
	panel.speed.SetValue(float32(settings.LinesPerSecond))
	panel.contrast.SetValue(float32(settings.Contrast))
	panel.waterfallRange.SetValues(settings.MinimumDBm, settings.MaximumDBm)
	panel.screen.waterfall.InvalidateColors()
	panel.refreshLabels()
	panel.screen.markSettingsDirty()
}

func (panel *FFTDisplay) refreshLabels() {
	panel.averageLabel.SetText(fmt.Sprintf(i18n.Source("text.79e61f99e79e"), panel.averagingMs))
	panel.refreshLabel.SetText(fmt.Sprintf(i18n.Source("text.3bddd0dd403c"), panel.refreshFPS))
	panel.decayLabel.SetText(fmt.Sprintf(i18n.Source("text.a9affd4760b0"), panel.peakDecay))
	panel.speedLabel.SetText(fmt.Sprintf(i18n.Source("text.1bd26570468c"), panel.screen.waterfallSettings.LinesPerSecond))
	panel.contrastLabel.SetText(fmt.Sprintf(i18n.Source("text.c74ba288f018"), panel.screen.waterfallSettings.Contrast))
	panel.windowButton.SetLabel(i18n.Source("text.4f11817cc853") + panel.window)
	panel.paletteButton.SetLabel(i18n.Source("text.aed5c2bb0f22") + panel.screen.waterfallSettings.Palette)
	panel.fftFloorLabel.SetText(fmt.Sprintf(i18n.Source("text.e8c1f7b93ef7"), panel.screen.spectrumMinimumDB))
	panel.fftCeilingLabel.SetText(fmt.Sprintf(i18n.Source("text.9b5e239bafc9"), panel.screen.spectrumMaximumDB))
	panel.waterfallRangeLabel.SetText(fmt.Sprintf(i18n.Source("text.f6ac728540d4"), panel.screen.waterfallSettings.MinimumDBm, panel.screen.waterfallSettings.MaximumDBm))
}

func indexOf(values []string, wanted string) int {
	for index, value := range values {
		if value == wanted {
			return index
		}
	}
	return 0
}

func (screen *MainScreen) drawPeakSpectrum(x, y, width, height float32) {
	if screen.fftDisplay == nil || len(screen.fftDisplay.peakSpectrum) != len(screen.spectrum) {
		return
	}
	if screen.stats.SpectrumCenterHz != screen.centerFrequencyHz || screen.fftDisplay.peakSpanHz != screen.spanHz {
		return
	}
	var previous rl.Vector2
	hasPrevious := false
	for pixel := 0; pixel < int(width); pixel++ {
		value, visible := screen.interpolatedSpectrumValue(screen.fftDisplay.peakSpectrum, float32(pixel)/width)
		if !visible {
			hasPrevious = false
			continue
		}
		current := rl.Vector2{X: x + float32(pixel), Y: screen.spectrumY(value, y, height)}
		if hasPrevious {
			rl.DrawLineEx(previous, current, 1, rl.Color{R: 255, G: 190, B: 45, A: 210})
		}
		previous, hasPrevious = current, true
	}
}
