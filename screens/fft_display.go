package screens

import (
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
	fftRangeLabel, waterfallRangeLabel                                *simpleui.Label
	average, refresh, decay, speed, contrast                          *simpleui.Slider
	peak                                                              *simpleui.Switch
	windowButton, paletteButton                                       *simpleui.Button
	fftRange, waterfallRange                                          *simpleui.RangeSlider
}

func NewFFTDisplay(screen *MainScreen) *FFTDisplay {
	panel := &FFTDisplay{screen: screen, averagingMs: screen.fftAveragingMs, refreshFPS: screen.fftRefreshFPS, peakHold: screen.fftPeakHold, peakDecay: screen.fftPeakDecay, window: screen.fftWindow}
	label := func(id string, x, y, width float32, text string) *simpleui.Label {
		value := simpleui.NewLabel(id, x, y, width, 18, text, 12)
		value.SetAlignment(simpleui.AlignCenter)
		return value
	}
	panel.averageLabel = label("fftAverageLabel", 40, 654, 190, "")
	panel.average = simpleui.NewSlider("fftAverage", 48, 680, 174, 20, 10, 300, float32(panel.averagingMs))
	panel.average.SetStep(1)
	panel.refreshLabel = label("fftRefreshLabel", 240, 654, 190, "")
	panel.refresh = simpleui.NewSlider("fftRefresh", 248, 680, 174, 20, 5, 60, float32(panel.refreshFPS))
	panel.refresh.SetStep(1)
	panel.peak = simpleui.NewSwitch("fftPeak", 445, 650, 180, 28, "PEAK HOLD", panel.peakHold, 12)
	panel.decayLabel = label("fftDecayLabel", 445, 692, 180, "PEAK DECAY  3 dB/s")
	panel.decay = simpleui.NewSlider("fftDecay", 453, 718, 164, 18, 1, 10, panel.peakDecay)
	panel.decay.SetStep(1)
	panel.windowButton = simpleui.NewButton("fftWindow", 45, 740, 210, 43, "WINDOW  HANN", 12)
	size := label("fftSize", 265, 740, 155, "FFT 4096\n500 Hz/bin")
	panel.fftRangeLabel = label("fftRangeLabel", 430, 752, 205, "FFT RANGE  -37 / 0 dB")
	panel.fftRange = simpleui.NewRangeSlider("fftRange", 438, 782, 190, 18, -140, 0, -37, 0)
	panel.fftRange.SetMinimumGap(10)
	panel.fftRange.SetRangeDragging(false)

	panel.speedLabel = label("fftWaterfallSpeedLabel", 710, 654, 185, "SPEED  15 lines/s")
	panel.speed = simpleui.NewSlider("fftWaterfallSpeed", 718, 680, 170, 20, 5, 60, 15)
	panel.speed.SetStep(1)
	panel.contrastLabel = label("fftWaterfallContrastLabel", 905, 654, 185, "CONTRAST  102 %")
	panel.contrast = simpleui.NewSlider("fftWaterfallContrast", 913, 680, 170, 20, 25, 200, 102)
	panel.contrast.SetStep(1)
	panel.waterfallRangeLabel = label("fftWaterfallRangeLabel", 710, 720, 185, "LEVEL  -60 / 0 dB")
	panel.waterfallRange = simpleui.NewRangeSlider("fftWaterfallRange", 718, 750, 170, 18, -120, 0, -60, 0)
	panel.waterfallRange.SetMinimumGap(10)
	panel.waterfallRange.SetRangeDragging(false)
	panel.paletteButton = simpleui.NewButton("fftPalette", 905, 735, 120, 43, "PALETTE GRAY", 12)
	reset := simpleui.NewButton("fftWaterfallReset", 1035, 735, 82, 43, "RESET", 12)
	adjust := simpleui.NewButton("fftWaterfallAdjust", 1127, 735, 105, 43, "ADJUST", 12)

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
	panel.fftRange.OnChange(func(low, high float32) {
		screen.spectrumMinimumDB, screen.spectrumMaximumDB = low, high
		screen.syncSquelchToSpectrumRange()
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
	adjust.OnClick(func() { screen.selectTool("WATERFALL_ADJUST") })

	panel.controls = []simpleui.Element{panel.averageLabel, panel.average, panel.refreshLabel, panel.refresh,
		panel.peak, panel.decayLabel, panel.decay, panel.windowButton, size, panel.fftRangeLabel, panel.fftRange,
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
	panel.speed.SetValue(float32(panel.screen.waterfallSettings.LinesPerSecond))
	panel.contrast.SetValue(float32(panel.screen.waterfallSettings.Contrast))
	panel.waterfallRange.SetValues(panel.screen.waterfallSettings.MinimumDBm, panel.screen.waterfallSettings.MaximumDBm)
	panel.fftRange.SetValues(panel.screen.spectrumMinimumDB, panel.screen.spectrumMaximumDB)
	panel.refreshLabels()
}

func (panel *FFTDisplay) DrawPanel() {
	rl.DrawLineEx(rl.Vector2{X: 690, Y: toolY + 12}, rl.Vector2{X: 690, Y: toolY + toolH - 12}, 1.5, colors.border)
	drawCentered("FFT DISPLAY", rl.Rectangle{X: 260, Y: toolY + 7, Width: 220, Height: 20}, 14, colors.cyan)
	drawCentered("WATERFALL", rl.Rectangle{X: 850, Y: toolY + 7, Width: 220, Height: 20}, 14, colors.blue)
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
	windows := []string{"HANN", "BLACKMAN-HARRIS", "FLAT TOP", "RECTANGULAR"}
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
	palettes := []string{"BLUE", "VIRIDIS", "FIRE", "GRAY"}
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
	panel.averageLabel.SetText(fmt.Sprintf("%s  %d ms", T("AVERAGING"), panel.averagingMs))
	panel.refreshLabel.SetText(fmt.Sprintf("%s  %d FPS", T("REFRESH"), panel.refreshFPS))
	panel.decayLabel.SetText(fmt.Sprintf("%s  %.0f dB/s", T("PEAK DECAY"), panel.peakDecay))
	panel.speedLabel.SetText(fmt.Sprintf("%s  %d %s", T("SPEED"), panel.screen.waterfallSettings.LinesPerSecond, T("lines/s")))
	panel.contrastLabel.SetText(fmt.Sprintf("%s  %d %%", T("CONTRAST"), panel.screen.waterfallSettings.Contrast))
	panel.windowButton.SetLabel(T("WINDOW") + "  " + panel.window)
	panel.paletteButton.SetLabel(T("PALETTE") + " " + T(panel.screen.waterfallSettings.Palette))
	panel.fftRangeLabel.SetText(fmt.Sprintf("%s  %.0f / %.0f dB", T("FFT RANGE"), panel.screen.spectrumMinimumDB, panel.screen.spectrumMaximumDB))
	panel.waterfallRangeLabel.SetText(fmt.Sprintf("%s  %.0f / %.0f dB", T("LEVEL"), panel.screen.waterfallSettings.MinimumDBm, panel.screen.waterfallSettings.MaximumDBm))
	panel.peak.SetLabel(T("PEAK HOLD"))
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
