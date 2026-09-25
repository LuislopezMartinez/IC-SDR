package screens

import (
	"go-zero/internal/i18n"

	"fmt"
	"math"
	"strings"

	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type scanPeak struct {
	frequencyHz int64
	levelDB     float32
}

type ScanPanel struct {
	screen                 *MainScreen
	resume, policy, status string
	centerToMemory         bool
	dwellMs                int
	minimumHz, maximumHz   int64
	running, listening     bool
	overlayVisible         bool
	activePeakHz           int64
	currentMemory          int
	consecutiveHits        int
	lastSignalAt           float64
	dragTarget             int
	choiceMenu             string
	saveFeedbackUntil      float64
	savedMemoryName        string
}

func NewScanPanel(screen *MainScreen) *ScanPanel {
	half := screen.spanHz / 2
	minimum, maximum := screen.scanMinimumHz, screen.scanMaximumHz
	if minimum >= maximum {
		minimum, maximum = screen.centerFrequencyHz-half*4/5, screen.centerFrequencyHz+half*4/5
	}
	return &ScanPanel{screen: screen, centerToMemory: screen.scanCenterToMemory, resume: screen.scanResume, policy: screen.scanPolicy, dwellMs: screen.scanDwellMs,
		minimumHz: minimum, maximumHz: maximum,
		activePeakHz: -1, currentMemory: -1, status: i18n.Source("text.c2e3ac47f4a3"), overlayVisible: true}
}

func (p *ScanPanel) Enter() {
	low, high := p.screen.centerFrequencyHz-p.screen.spanHz/2, p.screen.centerFrequencyHz+p.screen.spanHz/2
	if p.minimumHz >= p.maximumHz {
		p.minimumHz = low + p.screen.spanHz/10
		p.maximumHz = high - p.screen.spanHz/10
	}
}

func (p *ScanPanel) Stop() {
	p.running, p.listening = false, false
	p.consecutiveHits, p.currentMemory, p.activePeakHz = 0, -1, -1
	p.status = i18n.Source("text.c2e3ac47f4a3")
}

func (p *ScanPanel) Update(spectrum []float32) {
	if !p.running || len(spectrum) == 0 || p.screen.stats.SampleRate <= 0 {
		return
	}
	p.updateWatch(spectrum)
}

func (p *ScanPanel) UpdateInput() {
	if p.screen.webServer != nil && p.screen.webServer.RemoteActive() {
		return
	}
	p.updateInput()
}

func (p *ScanPanel) updateWatch(spectrum []float32) {
	if p.listening {
		half := max(int64(1500), int64(p.screen.demodBandwidthHz/2))
		current := p.findPeak(spectrum, p.activePeakHz-half, p.activePeakHz+half, 1, 0)
		level := float32(-140)
		if current != nil {
			level = current.levelDB
		}
		if p.policy == i18n.Source("text.8cb51251cc49") {
			alternative := p.findPeak(spectrum, p.minimumHz, p.maximumHz, p.activePeakHz-half, p.activePeakHz+half)
			if alternative != nil && alternative.levelDB >= p.thresholdDB() && alternative.levelDB >= level+6 {
				p.activePeakHz = alternative.frequencyHz
				p.tuneDetected(alternative)
				p.lastSignalAt = rl.GetTime()
				if p.currentMemory >= 0 {
					p.status = i18n.Source("text.d21296b55fce") + p.screen.memoryPanel.memories[p.currentMemory].Name
				} else {
					p.status = i18n.Source("text.f27222c817b4") + formatScanMHz(alternative.frequencyHz)
				}
				return
			}
		}
		p.processListening(level)
		return
	}
	p.processCandidate(p.findPeak(spectrum, p.minimumHz, p.maximumHz, 1, 0))
}

func memoryPassband(memory MemoryEntry) (int64, int64) {
	bw := int64(max(memory.FilterBandwidthHz, 1000))
	if memory.Mode == i18n.Source("text.61f0acff1735") {
		return memory.FrequencyHz, memory.FrequencyHz + bw
	}
	if memory.Mode == i18n.Source("text.6323db4948ad") {
		return memory.FrequencyHz - bw, memory.FrequencyHz
	}
	return memory.FrequencyHz - bw/2, memory.FrequencyHz + bw/2
}

func (p *ScanPanel) processCandidate(peak *scanPeak) {
	if p.listening {
		level := float32(-140)
		if peak != nil {
			level = peak.levelDB
		}
		p.processListening(level)
		return
	}
	if peak == nil || peak.levelDB < p.thresholdDB() {
		p.consecutiveHits = 0
		p.status = i18n.Source("text.368328f69b28")
		return
	}
	p.consecutiveHits++
	if p.consecutiveHits < 3 {
		p.status = fmt.Sprintf(i18n.Source("text.594e6fa45a21"), peak.levelDB)
		return
	}
	p.activePeakHz = peak.frequencyHz
	p.tuneDetected(peak)
	p.listening, p.consecutiveHits, p.lastSignalAt = true, 0, rl.GetTime()
	if p.currentMemory >= 0 {
		p.status = i18n.Source("text.58e004a8ae5d") + p.screen.memoryPanel.memories[p.currentMemory].Name
	} else {
		p.status = i18n.Source("text.9a686f8614c7") + formatScanMHz(peak.frequencyHz)
	}
}

func (p *ScanPanel) processListening(level float32) {
	if level >= p.thresholdDB()-5 {
		p.lastSignalAt = rl.GetTime()
		return
	}
	if p.resume == i18n.Source("text.aacf94b7be62") {
		p.status = i18n.Source("text.aacf94b7be62")
		return
	}
	delay := .3
	if p.resume == i18n.Source("text.85135a165905") {
		delay = float64(p.dwellMs) / 1000
	}
	if rl.GetTime()-p.lastSignalAt >= delay {
		p.stopListening()
	}
}

func (p *ScanPanel) thresholdDB() float32 { return float32(p.screen.squelchThreshold) }

func (p *ScanPanel) stopListening() {
	p.listening, p.consecutiveHits = false, 0
	p.currentMemory, p.activePeakHz, p.status = -1, -1, i18n.Source("text.368328f69b28")
}

func (p *ScanPanel) tuneDetected(peak *scanPeak) {
	p.activePeakHz = peak.frequencyHz
	p.currentMemory = -1
	if p.centerToMemory {
		if index := p.matchingMemory(peak.frequencyHz); index >= 0 {
			p.currentMemory = index
			memory := p.screen.memoryPanel.memories[index]
			preservedCenter := p.screen.centerFrequencyHz
			p.screen.centerMode = false
			p.screen.memoryPanel.recallForScanner(memory)
			p.screen.centerFrequencyHz = preservedCenter
			if p.screen.receiver != nil {
				p.screen.receiver.SetCenterFrequency(preservedCenter)
			}
			p.syncModeSwitch()
			return
		}
	}
	p.tunePeak(peak.frequencyHz, false)
}

func (p *ScanPanel) matchingMemory(frequencyHz int64) int {
	best, bestDistance := -1, int64(math.MaxInt64)
	for index, memory := range p.screen.memoryPanel.memories {
		if !memory.ScanEnabled {
			continue
		}
		low, high := memoryPassband(memory)
		if frequencyHz < low || frequencyHz > high {
			continue
		}
		if distance := absInt64(memory.FrequencyHz - frequencyHz); distance < bestDistance {
			best, bestDistance = index, distance
		}
	}
	return best
}

func (p *ScanPanel) tunePeak(frequencyHz int64, centered bool) {
	dial := frequencyHz
	if p.screen.audioPanel != nil {
		if p.screen.mode.SelectedText() == i18n.Source("text.61f0acff1735") {
			dial -= int64(p.screen.audioPanel.pbtLow)
		} else if p.screen.mode.SelectedText() == i18n.Source("text.6323db4948ad") {
			dial += int64(p.screen.audioPanel.pbtLow)
		}
	}
	dial = int64(math.Round(float64(dial)/1000) * 1000)
	p.screen.frequencyHz = max(dial, int64(1000))
	p.screen.centerMode = centered
	if centered {
		p.screen.centerFrequencyHz = p.screen.frequencyHz
	}
	p.syncTuning(centered)
}

func (p *ScanPanel) syncTuning(centerChanged bool) {
	p.syncModeSwitch()
	if p.screen.receiver != nil {
		if centerChanged {
			p.screen.receiver.SetCenterFrequency(p.screen.centerFrequencyHz)
		}
		p.screen.receiver.SetDemodulator(p.screen.mode.SelectedText(), p.screen.frequencyHz, p.screen.demodBandwidthHz)
	}
	p.screen.markSettingsDirty()
}

func (p *ScanPanel) syncModeSwitch() {
	if p.screen.vfoModeSwitch != nil {
		p.screen.vfoModeSwitch.SetActive(!p.screen.centerMode)
	}
}

func (p *ScanPanel) findPeak(spectrum []float32, requestedLow, requestedHigh, excludedLow, excludedHigh int64) *scanPeak {
	visibleLow := p.screen.centerFrequencyHz - p.screen.spanHz/2
	visibleHigh := p.screen.centerFrequencyHz + p.screen.spanHz/2
	low, high := max(requestedLow, visibleLow), min(requestedHigh, visibleHigh)
	if high <= low || p.screen.stats.SampleRate <= 0 {
		return nil
	}
	best := &scanPeak{frequencyHz: low, levelDB: -140}
	for index := range spectrum {
		hz := p.screen.centerFrequencyHz + int64(math.Round((float64(index)-float64(len(spectrum))/2)*p.screen.stats.SampleRate/float64(len(spectrum))))
		if hz < low || hz > high || (excludedLow <= excludedHigh && hz >= excludedLow && hz <= excludedHigh) {
			continue
		}
		if excludedLow > excludedHigh && !p.listening && absInt64(hz-p.screen.centerFrequencyHz) < 1500 {
			continue
		}
		level := spectrum[index]
		if index > 0 && index+1 < len(spectrum) {
			level = (spectrum[index-1] + 2*spectrum[index] + spectrum[index+1]) / 4
		}
		if level > best.levelDB {
			best.frequencyHz, best.levelDB = hz, level
		}
	}
	return best
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

func (p *ScanPanel) ToggleRunning() {
	p.running = !p.running
	p.listening, p.consecutiveHits, p.currentMemory, p.activePeakHz = false, 0, -1, -1
	if p.running {
		p.screen.centerMode = false
		p.syncModeSwitch()
		p.status = i18n.Source("text.368328f69b28")
	} else {
		p.status = i18n.Source("text.c2e3ac47f4a3")
		p.syncModeSwitch()
	}
}

func (p *ScanPanel) DrawPanel() {
	simpleui.DrawTextStyled(i18n.Source("text.3ab76426b6b1"), 42, 644, 16, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawTextStyled(p.displayStatus(), 165, 643, 15, simpleui.FontSemiBold, func() rl.Color {
		if p.running {
			return colors.green
		}
		return colors.muted
	}())
	simpleui.DrawTextStyled(fmt.Sprintf(i18n.Source("text.b36443dc69a7"), float64(p.minimumHz)/1e6, float64(p.maximumHz)/1e6, p.screen.squelchThreshold), 720, 645, 13, simpleui.FontSemiBold, colors.muted)
	memoryColor := rl.Color{R: 45, G: 58, B: 72, A: 255}
	memoryDetail := i18n.Source("text.65d91a8169bb")
	if p.centerToMemory {
		memoryColor = rl.Color{R: 20, G: 120, B: 155, A: 255}
		memoryDetail = i18n.Source("text.61e9965fdaeb")
	}
	drawScanButton(42, 678, 220, 58, i18n.Source("text.1ab4548ebac8"), memoryDetail, memoryColor)
	drawScanButton(276, 678, 220, 58, i18n.Source("text.97d93859b616"), p.resumeDescription(), rl.Color{R: 150, G: 95, B: 18, A: 255})
	drawScanButton(510, 678, 190, 58, fmt.Sprintf(i18n.Source("text.0941f1ae0f33"), p.dwellMs/1000), i18n.Source("text.ab2f402a25a7"), rl.Color{R: 70, G: 68, B: 55, A: 255})
	startColor, startText, startDetail := rl.Color{R: 25, G: 125, B: 65, A: 255}, i18n.Source("text.b4fe709f8dde"), i18n.Source("text.00c06d82f25a")
	if p.running {
		startColor, startText, startDetail = rl.Color{R: 155, G: 42, B: 35, A: 255}, i18n.Source("text.5a97c42b8106"), i18n.Source("text.e726c37337e1")
	}
	drawScanButton(714, 678, 190, 58, startText, startDetail, startColor)
	saveTitle, saveDetail := i18n.Source("text.6ff697259e83"), i18n.Source("text.241615dbf1e4")
	saveColor := rl.Color{R: 25, G: 85, B: 145, A: 255}
	if rl.GetTime() < p.saveFeedbackUntil {
		saveTitle, saveDetail = i18n.Source("text.0da8ee259f58"), p.savedMemoryName
		saveColor = rl.Color{R: 24, G: 125, B: 70, A: 255}
	}
	drawScanButton(918, 678, 190, 58, saveTitle, saveDetail, saveColor)
	drawScanButton(1122, 678, 260, 58, i18n.Source("text.f00673420f00"), p.policyDescription(), rl.Color{R: 45, G: 58, B: 72, A: 255})
	if p.choiceMenu != "" {
		p.drawChoiceStrip()
	} else {
		simpleui.DrawTextStyled(i18n.Source("text.7c2a22ae4d58"), 42, 760, 14, simpleui.FontRegular, colors.muted)
	}
}

func drawScanButton(x, y, w, h float32, title, detail string, background rl.Color) {
	bounds := rl.Rectangle{X: x, Y: y, Width: w, Height: h}
	rl.DrawRectangleRounded(bounds, .12, 7, background)
	rl.DrawRectangleRoundedLinesEx(bounds, .12, 7, 1.5, colors.cyan)
	titleColor := simpleui.EnsureTextContrast(colors.text, background)
	detailColor := simpleui.EnsureTextContrast(colors.muted, background)
	titleSize := simpleui.MeasureTextStyled(title, 14, simpleui.FontSemiBold)
	detailSize := simpleui.MeasureTextStyled(detail, 12, simpleui.FontRegular)
	simpleui.DrawTextStyled(title, x+(w-titleSize.X)/2, y+9, 14, simpleui.FontSemiBold, titleColor)
	simpleui.DrawTextStyled(detail, x+(w-detailSize.X)/2, y+33, 12, simpleui.FontRegular, detailColor)
}

func (p *ScanPanel) updateInput() {
	if p.screen.viewMode != 1 || p.screen.overlayOpen() || !p.overlayVisible {
		p.dragTarget = 0
		return
	}
	mouse := simpleui.MousePosition()
	// The scanner now lives permanently in the sidebar. Keep its MIN/MAX
	// guides draggable on the FFT without activating the removed legacy panel.
	if p.screen.activeTool != i18n.Source("text.7a1580c49e45") {
		if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
			if !p.handleOverlayClick(mouse) {
				p.beginGuideDrag(mouse)
			}
		}
		if (p.dragTarget == 1 || p.dragTarget == 2) && rl.IsMouseButtonDown(rl.MouseButtonLeft) {
			p.dragGuide(mouse)
		}
		if rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
			p.dragTarget = 0
		}
		return
	}
	if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		if p.choiceMenu != "" && p.selectChoice(mouse) {
			simpleui.PlayActivationFeedback()
			return
		}
		switch {
		case pointIn(mouse, 42, 678, 220, 58):
			simpleui.PlayActivationFeedback()
			p.centerToMemory = !p.centerToMemory
			p.choiceMenu = ""
			p.screen.markSettingsDirty()
		case pointIn(mouse, 276, 678, 220, 58):
			simpleui.PlayActivationFeedback()
			p.toggleChoiceMenu("resume")
		case pointIn(mouse, 510, 678, 190, 58):
			simpleui.PlayActivationFeedback()
			p.toggleChoiceMenu("dwell")
		case pointIn(mouse, 714, 678, 190, 58):
			simpleui.PlayActivationFeedback()
			p.choiceMenu = ""
			p.ToggleRunning()
		case pointIn(mouse, 918, 678, 190, 58):
			simpleui.PlayActivationFeedback()
			p.choiceMenu = ""
			p.screen.memoryPanel.openSaveModal()
		case pointIn(mouse, 1122, 678, 260, 58):
			simpleui.PlayActivationFeedback()
			p.toggleChoiceMenu("policy")
		default:
			p.choiceMenu = ""
			if !p.handleOverlayClick(mouse) {
				p.beginGuideDrag(mouse)
			}
		}
	}
	if (p.dragTarget == 1 || p.dragTarget == 2) && rl.IsMouseButtonDown(rl.MouseButtonLeft) {
		p.dragGuide(mouse)
	}
	if rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
		p.dragTarget = 0
	}
}

func (p *ScanPanel) ShowMemorySaved(name string) {
	p.savedMemoryName = name
	p.saveFeedbackUntil = rl.GetTime() + 1.8
}

func (p *ScanPanel) toggleChoiceMenu(name string) {
	if p.choiceMenu == name {
		p.choiceMenu = ""
	} else {
		p.choiceMenu = name
	}
}

func (p *ScanPanel) choiceOptions() ([]string, int) {
	switch p.choiceMenu {
	case "resume":
		selected := map[string]int{i18n.Source("text.6ea56fae9eac"): 0, i18n.Source("text.85135a165905"): 1, i18n.Source("text.aacf94b7be62"): 2}[p.resume]
		return []string{i18n.Source("text.6644e058a60c"), i18n.Source("text.f59d9399e141"), i18n.Source("text.892e8fdfcc94")}, selected
	case "dwell":
		values := []int{1000, 2000, 3000, 5000, 10000}
		selected := 0
		labels := make([]string, len(values))
		for index, value := range values {
			labels[index] = fmt.Sprintf("%d s", value/1000)
			if value == p.dwellMs {
				selected = index
			}
		}
		return labels, selected
	case "policy":
		selected := 0
		if p.policy == i18n.Source("text.8cb51251cc49") {
			selected = 1
		}
		return []string{i18n.Source("text.f16e1a114189"), i18n.Source("text.5b7e51d47e4a")}, selected
	}
	return nil, -1
}

func (p *ScanPanel) drawChoiceStrip() {
	options, selected := p.choiceOptions()
	if len(options) == 0 {
		return
	}
	x, y, width, height := float32(300), float32(748), float32(1000), float32(48)
	itemWidth := width / float32(len(options))
	for index, option := range options {
		background := rl.Color{R: 28, G: 39, B: 49, A: 255}
		border := colors.border
		if index == selected {
			background, border = rl.Color{R: 20, G: 105, B: 135, A: 255}, colors.cyan
		}
		bounds := rl.Rectangle{X: x + float32(index)*itemWidth + 3, Y: y, Width: itemWidth - 6, Height: height}
		rl.DrawRectangleRounded(bounds, .12, 6, background)
		rl.DrawRectangleRoundedLinesEx(bounds, .12, 6, 1.5, border)
		measured := simpleui.MeasureTextStyled(option, 12, simpleui.FontSemiBold)
		simpleui.DrawTextStyled(option, bounds.X+(bounds.Width-measured.X)/2, bounds.Y+16, 12, simpleui.FontSemiBold, colors.text)
	}
}

func (p *ScanPanel) selectChoice(mouse rl.Vector2) bool {
	options, _ := p.choiceOptions()
	if len(options) == 0 || !pointIn(mouse, 300, 748, 1000, 48) {
		return false
	}
	index := min(max(int((mouse.X-300)/(1000/float32(len(options)))), 0), len(options)-1)
	switch p.choiceMenu {
	case "resume":
		p.resume = []string{i18n.Source("text.6ea56fae9eac"), i18n.Source("text.85135a165905"), i18n.Source("text.aacf94b7be62")}[index]
	case "dwell":
		p.dwellMs = []int{1000, 2000, 3000, 5000, 10000}[index]
	case "policy":
		p.policy = []string{i18n.Source("text.e3cc57e193d6"), i18n.Source("text.8cb51251cc49")}[index]
	}
	p.choiceMenu = ""
	p.screen.markSettingsDirty()
	return true
}

func (p *ScanPanel) resumeDescription() string {
	switch p.resume {
	case "AUTO":
		return i18n.Source("text.630bf4d7beca")
	case "HOLD":
		return i18n.Source("text.5d729fabca83")
	default:
		return i18n.Source("text.a7f460cb08f7")
	}
}

func (p *ScanPanel) policyDescription() string {
	if p.policy == i18n.Source("text.8cb51251cc49") {
		return i18n.Source("text.8dbbb1195b28")
	}
	return i18n.Source("text.a525b7eaa09c")
}

func (p *ScanPanel) displayStatus() string {
	switch {
	case p.status == "READY":
		return i18n.Source("text.d78afe9b19e4")
	case p.status == "SCANNING":
		return i18n.Source("text.56307fcca4b2")
	case p.status == "HOLD":
		return i18n.Source("text.8938a45da268")
	case strings.HasPrefix(p.status, "VERIFY"):
		return i18n.Source("text.d6d6ffba9552") + strings.TrimPrefix(p.status, i18n.Source("text.188a5356a091"))
	case strings.HasPrefix(p.status, "SIGNAL"):
		return i18n.Source("text.b81e489cf9d6") + strings.TrimPrefix(p.status, i18n.Source("text.8e1a5272bdd1"))
	case strings.HasPrefix(p.status, "LISTENING"):
		return i18n.Source("text.b81e489cf9d6") + strings.TrimPrefix(p.status, i18n.Source("text.18f4914611d3"))
	case strings.HasPrefix(p.status, "JUMP"):
		return i18n.Source("text.1bd8ee8cff59") + strings.TrimPrefix(p.status, i18n.Source("text.f8b3c726c4df"))
	default:
		return p.status
	}
}

func pointIn(point rl.Vector2, x, y, w, h float32) bool {
	return rl.CheckCollisionPointRec(point, rl.Rectangle{X: x, Y: y, Width: w, Height: h})
}

func (p *ScanPanel) beginGuideDrag(mouse rl.Vector2) {
	x, y, w, h := p.screen.spectrumGeometry()
	minX := p.frequencyX(p.minimumHz, x, w)
	maxX := p.frequencyX(p.maximumHz, x, w)
	if minX >= x && minX <= x+w && mouse.Y >= y && mouse.Y <= y+h && float32(math.Abs(float64(mouse.X-minX))) < 14 {
		p.dragTarget = 1
	} else if maxX >= x && maxX <= x+w && mouse.Y >= y && mouse.Y <= y+h && float32(math.Abs(float64(mouse.X-maxX))) < 14 {
		p.dragTarget = 2
	}
}
func (p *ScanPanel) dragGuide(mouse rl.Vector2) {
	x, _, w, _ := p.screen.spectrumGeometry()
	hz := p.screen.centerFrequencyHz - p.screen.spanHz/2 + int64(math.Round(float64(min(max((mouse.X-x)/w, 0), 1)*float32(p.screen.spanHz))))
	if p.dragTarget == 1 {
		p.minimumHz = min(hz, p.maximumHz-1000)
	} else {
		p.maximumHz = max(hz, p.minimumHz+1000)
	}
	p.screen.markSettingsDirty()
}
func (p *ScanPanel) ConsumesSpectrumInput() bool { return p.running || p.dragTarget != 0 }

const (
	scanLimitInside = iota
	scanLimitLeft
	scanLimitRight
)

func scanLimitSide(hz, visibleLow, visibleHigh int64) int {
	if hz < visibleLow {
		return scanLimitLeft
	}
	if hz > visibleHigh {
		return scanLimitRight
	}
	return scanLimitInside
}

func scanFitSpan(minimumHz, maximumHz int64) int64 {
	width := max(int64(1), maximumHz-minimumHz)
	required := int64(math.Ceil(float64(width) * 1.2))
	for _, span := range []int64{50_000, 100_000, 250_000, 500_000, 1_000_000, 2_000_000} {
		if span >= required {
			return span
		}
	}
	return 2_000_000
}

func scanFitButtonBounds(x, y, w float32) rl.Rectangle {
	width := min(float32(210), max(float32(150), w-24))
	return rl.Rectangle{X: x + (w-width)/2, Y: y + 8, Width: width, Height: 26}
}

func scanEdgeTagBounds(side, row int, x, y, w float32) rl.Rectangle {
	width := min(float32(188), max(float32(128), w*.28))
	bounds := rl.Rectangle{X: x + 8, Y: y + 43 + float32(row)*43, Width: width, Height: 36}
	if side == scanLimitRight {
		bounds.X = x + w - width - 8
	}
	return bounds
}

func (p *ScanPanel) handleOverlayClick(mouse rl.Vector2) bool {
	x, y, w, _ := p.screen.spectrumGeometry()
	low, high := p.screen.centerFrequencyHz-p.screen.spanHz/2, p.screen.centerFrequencyHz+p.screen.spanHz/2
	minSide, maxSide := scanLimitSide(p.minimumHz, low, high), scanLimitSide(p.maximumHz, low, high)
	if minSide == scanLimitInside && maxSide == scanLimitInside {
		return false
	}
	if rl.CheckCollisionPointRec(mouse, scanFitButtonBounds(x, y, w)) {
		p.fitSegmentToFFT()
		p.dragTarget = 5
		simpleui.PlayActivationFeedback()
		return true
	}
	if minSide != scanLimitInside && rl.CheckCollisionPointRec(mouse, scanEdgeTagBounds(minSide, 0, x, y, w)) {
		p.centerFFTAt(p.minimumHz)
		p.dragTarget = 3
		simpleui.PlayActivationFeedback()
		return true
	}
	if maxSide != scanLimitInside && rl.CheckCollisionPointRec(mouse, scanEdgeTagBounds(maxSide, 1, x, y, w)) {
		p.centerFFTAt(p.maximumHz)
		p.dragTarget = 4
		simpleui.PlayActivationFeedback()
		return true
	}
	return false
}

func (p *ScanPanel) centerFFTAt(frequencyHz int64) {
	p.screen.centerFrequencyHz = frequencyHz
	p.screen.centerMode = false
	p.syncModeSwitch()
	if p.screen.receiver != nil {
		p.screen.receiver.SetCenterFrequency(frequencyHz)
	}
	p.screen.waterfall.Reset()
	p.screen.markSettingsDirty()
}

func (p *ScanPanel) fitSegmentToFFT() {
	p.screen.spanHz = scanFitSpan(p.minimumHz, p.maximumHz)
	p.centerFFTAt(p.minimumHz + (p.maximumHz-p.minimumHz)/2)
}

func (p *ScanPanel) DrawSpectrumOverlay(x, y, w, h float32) {
	if !p.overlayVisible {
		return
	}
	low, high := p.screen.centerFrequencyHz-p.screen.spanHz/2, p.screen.centerFrequencyHz+p.screen.spanHz/2
	minSide, maxSide := scanLimitSide(p.minimumHz, low, high), scanLimitSide(p.maximumHz, low, high)
	minX, maxX := p.frequencyX(p.minimumHz, x, w), p.frequencyX(p.maximumHz, x, w)
	visibleMinX, visibleMaxX := max(x, min(minX, x+w)), max(x, min(maxX, x+w))
	if p.maximumHz >= low && p.minimumHz <= high {
		rl.DrawRectangleRec(rl.Rectangle{X: visibleMinX, Y: y, Width: max(0, visibleMaxX-visibleMinX), Height: h - 26}, rl.Color{R: 25, G: 155, B: 220, A: 25})
	}
	if minSide == scanLimitInside {
		rl.DrawLineEx(rl.Vector2{X: minX, Y: y}, rl.Vector2{X: minX, Y: y + h - 26}, 2, colors.cyan)
		drawScanTag(fmt.Sprintf(i18n.Source("text.93f24c02fb1a"), float64(p.minimumHz)/1e6), minX, y+50, x, w)
	} else {
		drawScanContinuation(minSide, x, y, w, h)
		drawScanEdgeTag(fmt.Sprintf(i18n.Source("text.93f24c02fb1a"), float64(p.minimumHz)/1e6), p.minimumHz, minSide, 0, low, high, x, y, w)
	}
	if maxSide == scanLimitInside {
		rl.DrawLineEx(rl.Vector2{X: maxX, Y: y}, rl.Vector2{X: maxX, Y: y + h - 26}, 2, colors.cyan)
		drawScanTag(fmt.Sprintf(i18n.Source("text.68ab84e8e4f6"), float64(p.maximumHz)/1e6), maxX, y+78, x, w)
	} else {
		drawScanContinuation(maxSide, x, y, w, h)
		drawScanEdgeTag(fmt.Sprintf(i18n.Source("text.68ab84e8e4f6"), float64(p.maximumHz)/1e6), p.maximumHz, maxSide, 1, low, high, x, y, w)
	}
	if minSide != scanLimitInside || maxSide != scanLimitInside {
		drawScanFitButton(scanFitButtonBounds(x, y, w))
	}
	if p.running && p.screen.activeTool != i18n.Source("text.7a1580c49e45") {
		p.drawCompact(x+w-330, y+8)
	}
}
func (p *ScanPanel) frequencyX(hz int64, x, w float32) float32 {
	return x + w*(.5+float32(hz-p.screen.centerFrequencyHz)/float32(p.screen.spanHz))
}
func drawScanTag(text string, markerX, y, chartX, chartW float32) {
	width := simpleui.MeasureTextStyled(text, 12, simpleui.FontSemiBold).X + 16
	centerX := min(max(markerX, chartX+width/2+4), chartX+chartW-width/2-4)
	bounds := rl.Rectangle{X: centerX - width/2, Y: y, Width: width, Height: 24}
	background := mixColor(colors.panel, colors.blue, .16)
	textColor := simpleui.EnsureTextContrast(colors.cyan, background)
	rl.DrawRectangleRounded(bounds, .2, 6, background)
	rl.DrawRectangleRoundedLinesEx(bounds, .2, 6, 1, colors.cyan)
	simpleui.DrawTextStyled(text, bounds.X+8, y+5, 12, simpleui.FontSemiBold, textColor)
}

func drawScanContinuation(side int, x, y, w, h float32) {
	lineX := x + 2
	if side == scanLimitRight {
		lineX = x + w - 2
	}
	for lineY := y + 38; lineY < y+h-26; lineY += 12 {
		rl.DrawLineEx(rl.Vector2{X: lineX, Y: lineY}, rl.Vector2{X: lineX, Y: min(lineY+7, y+h-26)}, 2, colors.cyan)
	}
}

func drawScanEdgeTag(text string, frequencyHz int64, side, row int, low, high int64, x, y, w float32) {
	bounds := scanEdgeTagBounds(side, row, x, y, w)
	background := mixColor(colors.panel, colors.blue, .22)
	rl.DrawRectangleRounded(bounds, .18, 6, background)
	rl.DrawRectangleRoundedLinesEx(bounds, .18, 6, 1.5, colors.cyan)
	arrow := "<  "
	if side == scanLimitRight {
		arrow = "  >"
	}
	title := arrow + text
	if side == scanLimitRight {
		title = text + arrow
	}
	distance := frequencyHz - low
	if side == scanLimitRight {
		distance = frequencyHz - high
	}
	simpleui.DrawTextStyled(title, bounds.X+9, bounds.Y+4, 11, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawTextStyled(formatScanDistance(distance), bounds.X+9, bounds.Y+20, 10, simpleui.FontRegular, colors.muted)
}

func formatScanDistance(distanceHz int64) string {
	sign := "+"
	if distanceHz < 0 {
		sign = "-"
	}
	abs := absInt64(distanceHz)
	if abs >= 1_000_000 {
		return fmt.Sprintf("%s%.3f MHz", sign, float64(abs)/1e6)
	}
	if abs >= 1_000 {
		return fmt.Sprintf("%s%.1f kHz", sign, float64(abs)/1e3)
	}
	return fmt.Sprintf("%s%d Hz", sign, abs)
}

func drawScanFitButton(bounds rl.Rectangle) {
	background := rl.Color{R: 12, G: 72, B: 100, A: 238}
	rl.DrawRectangleRounded(bounds, .22, 7, background)
	rl.DrawRectangleRoundedLinesEx(bounds, .22, 7, 1, colors.cyan)
	text := "AJUSTAR FFT AL SEGMENTO"
	measured := simpleui.MeasureTextStyled(text, 10, simpleui.FontSemiBold)
	simpleui.DrawTextStyled(text, bounds.X+(bounds.Width-measured.X)/2, bounds.Y+8, 10, simpleui.FontSemiBold, simpleui.EnsureTextContrast(colors.text, background))
}
func (p *ScanPanel) drawCompact(x, y float32) {
	bounds := rl.Rectangle{X: x, Y: y, Width: 315, Height: 28}
	background := rl.Color{R: 8, G: 18, B: 24, A: 240}
	rl.DrawRectangleRounded(bounds, .2, 6, background)
	rl.DrawRectangleRoundedLinesEx(bounds, .2, 6, 1, colors.cyan)
	rl.DrawCircle(int32(x+15), int32(y+14), 5, colors.green)
	textColor := simpleui.EnsureTextContrast(colors.text, background)
	simpleui.DrawTextStyled(i18n.Source("text.312992971874")+p.status, x+29, y+6, 12, simpleui.FontSemiBold, textColor)
	if !p.screen.overlayOpen() && rl.IsMouseButtonPressed(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(simpleui.MousePosition(), bounds) {
		p.screen.uiSounds.PlayToolSelect()
		p.screen.selectTool(i18n.Source("text.7a1580c49e45"))
	}
}
func formatScanMHz(hz int64) string {
	return fmt.Sprintf(i18n.Source("text.4d983ee4ca80"), float64(hz)/1e6)
}
