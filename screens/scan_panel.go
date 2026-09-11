package screens

import (
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
		activePeakHz: -1, currentMemory: -1, status: "READY", overlayVisible: true}
}

func (p *ScanPanel) Enter() {
	low, high := p.screen.centerFrequencyHz-p.screen.spanHz/2, p.screen.centerFrequencyHz+p.screen.spanHz/2
	if p.minimumHz >= p.maximumHz || p.minimumHz < low || p.maximumHz > high {
		p.minimumHz = low + p.screen.spanHz/10
		p.maximumHz = high - p.screen.spanHz/10
	} else if p.minimumHz >= p.maximumHz {
		p.minimumHz, p.maximumHz = low, high
	}
}

func (p *ScanPanel) Update(spectrum []float32) {
	if !p.running || len(spectrum) == 0 || p.screen.stats.SampleRate <= 0 {
		return
	}
	p.updateWatch(spectrum)
}

func (p *ScanPanel) UpdateInput() { p.updateInput() }

func (p *ScanPanel) updateWatch(spectrum []float32) {
	if p.listening {
		half := max(int64(1500), int64(p.screen.demodBandwidthHz/2))
		current := p.findPeak(spectrum, p.activePeakHz-half, p.activePeakHz+half, 1, 0)
		level := float32(-140)
		if current != nil {
			level = current.levelDB
		}
		if p.policy == "STRONGER" {
			alternative := p.findPeak(spectrum, p.minimumHz, p.maximumHz, p.activePeakHz-half, p.activePeakHz+half)
			if alternative != nil && alternative.levelDB >= p.thresholdDB() && alternative.levelDB >= level+6 {
				p.activePeakHz = alternative.frequencyHz
				p.tuneDetected(alternative)
				p.lastSignalAt = rl.GetTime()
				if p.currentMemory >= 0 {
					p.status = "JUMP MEMORY  " + p.screen.memoryPanel.memories[p.currentMemory].Name
				} else {
					p.status = "JUMP  " + formatScanMHz(alternative.frequencyHz)
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
	if memory.Mode == "USB" {
		return memory.FrequencyHz, memory.FrequencyHz + bw
	}
	if memory.Mode == "LSB" {
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
		p.status = "SCANNING"
		return
	}
	p.consecutiveHits++
	if p.consecutiveHits < 3 {
		p.status = fmt.Sprintf("VERIFY  %.0f dB", peak.levelDB)
		return
	}
	p.activePeakHz = peak.frequencyHz
	p.tuneDetected(peak)
	p.listening, p.consecutiveHits, p.lastSignalAt = true, 0, rl.GetTime()
	if p.currentMemory >= 0 {
		p.status = "LISTENING  " + p.screen.memoryPanel.memories[p.currentMemory].Name
	} else {
		p.status = "SIGNAL  " + formatScanMHz(peak.frequencyHz)
	}
}

func (p *ScanPanel) processListening(level float32) {
	if level >= p.thresholdDB()-5 {
		p.lastSignalAt = rl.GetTime()
		return
	}
	if p.resume == "HOLD" {
		p.status = "HOLD"
		return
	}
	delay := .3
	if p.resume == "DELAY" {
		delay = float64(p.dwellMs) / 1000
	}
	if rl.GetTime()-p.lastSignalAt >= delay {
		p.stopListening()
	}
}

func (p *ScanPanel) thresholdDB() float32 { return float32(p.screen.squelchThreshold) }

func (p *ScanPanel) stopListening() {
	p.listening, p.consecutiveHits = false, 0
	p.currentMemory, p.activePeakHz, p.status = -1, -1, "SCANNING"
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
			p.screen.memoryPanel.recall(memory)
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
		if p.screen.mode.SelectedText() == "USB" {
			dial -= int64(p.screen.audioPanel.pbtLow)
		} else if p.screen.mode.SelectedText() == "LSB" {
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
		p.status = "SCANNING"
	} else {
		p.status = "READY"
		p.syncModeSwitch()
	}
}

func (p *ScanPanel) DrawPanel() {
	simpleui.DrawTextStyled("SCANNER", 42, 644, 16, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawTextStyled(p.displayStatus(), 165, 643, 15, simpleui.FontSemiBold, func() rl.Color {
		if p.running {
			return colors.green
		}
		return colors.muted
	}())
	simpleui.DrawTextStyled(fmt.Sprintf("RANGO %.5f–%.5f MHz  ·  DISPARO SQL %d dBm", float64(p.minimumHz)/1e6, float64(p.maximumHz)/1e6, p.screen.squelchThreshold), 720, 645, 13, simpleui.FontSemiBold, colors.muted)
	memoryColor := rl.Color{R: 45, G: 58, B: 72, A: 255}
	memoryDetail := "OFF · Tune the peak"
	if p.centerToMemory {
		memoryColor = rl.Color{R: 20, G: 120, B: 155, A: 255}
		memoryDetail = "ON · If it matches a channel"
	}
	drawScanButton(42, 678, 220, 58, "SNAP TO MEMORY", memoryDetail, memoryColor)
	drawScanButton(276, 678, 220, 58, "WHEN SIGNAL LOST  ▾", p.resumeDescription(), rl.Color{R: 150, G: 95, B: 18, A: 255})
	drawScanButton(510, 678, 190, 58, fmt.Sprintf("ESPERA %d s  ▾", p.dwellMs/1000), "Antes de continuar", rl.Color{R: 70, G: 68, B: 55, A: 255})
	startColor, startText, startDetail := rl.Color{R: 25, G: 125, B: 65, A: 255}, "START SCAN", "Search between MIN and MAX"
	if p.running {
		startColor, startText, startDetail = rl.Color{R: 155, G: 42, B: 35, A: 255}, "STOP SCAN", "Keep current frequency"
	}
	drawScanButton(714, 678, 190, 58, startText, startDetail, startColor)
	saveTitle, saveDetail := "SAVE MEMORY", "Current frequency and settings"
	saveColor := rl.Color{R: 25, G: 85, B: 145, A: 255}
	if rl.GetTime() < p.saveFeedbackUntil {
		saveTitle, saveDetail = "MEMORY SAVED  ✓", p.savedMemoryName
		saveColor = rl.Color{R: 24, G: 125, B: 70, A: 255}
	}
	drawScanButton(918, 678, 190, 58, saveTitle, saveDetail, saveColor)
	drawScanButton(1122, 678, 260, 58, "DURANTE LA LISTEN  ▾", p.policyDescription(), rl.Color{R: 45, G: 58, B: 72, A: 255})
	if p.choiceMenu != "" {
		p.drawChoiceStrip()
	} else {
		simpleui.DrawTextStyled("Drag MIN and MAX on the FFT · SQL level is the scanner trigger", 42, 760, 14, simpleui.FontRegular, colors.muted)
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
	if p.screen.activeTool != "SCAN" {
		if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
			p.beginGuideDrag(mouse)
		}
		if p.dragTarget != 0 && rl.IsMouseButtonDown(rl.MouseButtonLeft) {
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
			p.beginGuideDrag(mouse)
		}
	}
	if p.dragTarget != 0 && rl.IsMouseButtonDown(rl.MouseButtonLeft) {
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
		selected := map[string]int{"AUTO": 0, "DELAY": 1, "HOLD": 2}[p.resume]
		return []string{"CONTINUE FAST", "WAIT AND CONTINUE", "STAY ON SIGNAL"}, selected
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
		if p.policy == "STRONGER" {
			selected = 1
		}
		return []string{"KEEP CURRENT SIGNAL", "JUMP TO A STRONGER ONE"}, selected
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
		p.resume = []string{"AUTO", "DELAY", "HOLD"}[index]
	case "dwell":
		p.dwellMs = []int{1000, 2000, 3000, 5000, 10000}[index]
	case "policy":
		p.policy = []string{"CURRENT", "STRONGER"}[index]
	}
	p.choiceMenu = ""
	p.screen.markSettingsDirty()
	return true
}

func (p *ScanPanel) resumeDescription() string {
	switch p.resume {
	case "AUTO":
		return "Continue quickly"
	case "HOLD":
		return "Stay stopped"
	default:
		return "Wait and continue"
	}
}

func (p *ScanPanel) policyDescription() string {
	if p.policy == "STRONGER" {
		return "Jump to a stronger one"
	}
	return "Keep current signal"
}

func (p *ScanPanel) displayStatus() string {
	switch {
	case p.status == "READY":
		return "READY"
	case p.status == "SCANNING":
		return "SEARCHING FOR TRANSMISSIONS"
	case p.status == "HOLD":
		return "HOLDING AUDIO"
	case strings.HasPrefix(p.status, "VERIFY"):
		return "VERIFYING" + strings.TrimPrefix(p.status, "VERIFY")
	case strings.HasPrefix(p.status, "SIGNAL"):
		return "LISTENING" + strings.TrimPrefix(p.status, "SIGNAL")
	case strings.HasPrefix(p.status, "LISTENING"):
		return "LISTENING" + strings.TrimPrefix(p.status, "LISTENING")
	case strings.HasPrefix(p.status, "JUMP"):
		return "JUMP" + strings.TrimPrefix(p.status, "JUMP")
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
	if mouse.Y >= y && mouse.Y <= y+h && float32(math.Abs(float64(mouse.X-minX))) < 14 {
		p.dragTarget = 1
	} else if mouse.Y >= y && mouse.Y <= y+h && float32(math.Abs(float64(mouse.X-maxX))) < 14 {
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

func (p *ScanPanel) DrawSpectrumOverlay(x, y, w, h float32) {
	if !p.overlayVisible {
		return
	}
	if p.overlayVisible {
		minX, maxX := p.frequencyX(p.minimumHz, x, w), p.frequencyX(p.maximumHz, x, w)
		minX, maxX = max(x, min(minX, x+w)), max(x, min(maxX, x+w))
		rl.DrawRectangleRec(rl.Rectangle{X: minX, Y: y, Width: max(0, maxX-minX), Height: h - 26}, rl.Color{R: 25, G: 155, B: 220, A: 25})
		rl.DrawLineEx(rl.Vector2{X: minX, Y: y}, rl.Vector2{X: minX, Y: y + h - 26}, 2, colors.cyan)
		rl.DrawLineEx(rl.Vector2{X: maxX, Y: y}, rl.Vector2{X: maxX, Y: y + h - 26}, 2, colors.cyan)
		drawScanTag(fmt.Sprintf("MIN %.5f", float64(p.minimumHz)/1e6), minX, y+50)
		drawScanTag(fmt.Sprintf("MAX %.5f", float64(p.maximumHz)/1e6), maxX, y+78)
	}
	if p.running && p.screen.activeTool != "SCAN" {
		p.drawCompact(x+w-330, y+8)
	}
}
func (p *ScanPanel) frequencyX(hz int64, x, w float32) float32 {
	return x + w*(.5+float32(hz-p.screen.centerFrequencyHz)/float32(p.screen.spanHz))
}
func drawScanTag(text string, x, y float32) {
	width := simpleui.MeasureTextStyled(text, 12, simpleui.FontSemiBold).X + 16
	rl.DrawRectangleRounded(rl.Rectangle{X: x - width/2, Y: y, Width: width, Height: 24}, .2, 6, rl.Color{R: 5, G: 20, B: 28, A: 235})
	rl.DrawRectangleRoundedLinesEx(rl.Rectangle{X: x - width/2, Y: y, Width: width, Height: 24}, .2, 6, 1, colors.cyan)
	simpleui.DrawTextStyled(text, x-width/2+8, y+5, 12, simpleui.FontSemiBold, colors.cyan)
}
func (p *ScanPanel) drawCompact(x, y float32) {
	bounds := rl.Rectangle{X: x, Y: y, Width: 315, Height: 28}
	background := rl.Color{R: 8, G: 18, B: 24, A: 240}
	rl.DrawRectangleRounded(bounds, .2, 6, background)
	rl.DrawRectangleRoundedLinesEx(bounds, .2, 6, 1, colors.cyan)
	rl.DrawCircle(int32(x+15), int32(y+14), 5, colors.green)
	textColor := simpleui.EnsureTextContrast(colors.text, background)
	simpleui.DrawTextStyled("SCAN  ·  "+p.status, x+29, y+6, 12, simpleui.FontSemiBold, textColor)
	if rl.IsMouseButtonPressed(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(simpleui.MousePosition(), bounds) {
		p.screen.uiSounds.PlayToolSelect()
		p.screen.selectTool("SCAN")
	}
}
func formatScanMHz(hz int64) string { return fmt.Sprintf("%.5f MHz", float64(hz)/1e6) }
