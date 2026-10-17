package screens

import (
	"fmt"
	"strings"
	"time"

	"go-zero/internal/ft8"
	"go-zero/internal/i18n"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const ft8ToolID = "FT8"

type FT8Panel struct {
	simpleui.BaseElement
	screen                     *MainScreen
	controls                   []simpleui.Element
	enable                     *simpleui.Switch
	clear                      *simpleui.Button
	openActivity               *simpleui.Button
	depth                      *simpleui.Dropdown
	status                     ft8.Status
	tableOffset, tableSelected int
	lastClickKey               string
	lastClickAt                time.Time
	nextStatusRefresh          time.Time
	mapOpen                    bool
	mapDecodes                 []ft8.Decode
	mapSelected                int
	mapLat, mapLon, mapSpan    float64
	mapDragging                bool
	mapDragLast                rl.Vector2
	mapWindow                  time.Duration
	mapFilter                  string
	mapOnlyPosition            bool
	mapRegionOffset            int
	mapRegionFilter            string
	mapRegionOrder             []string
	mapNextRegionRank          time.Time
	mapHomeLat, mapHomeLon     float64
	mapHomeSet, viewerMode     bool
	viewer                     *ft8ViewerProcess
	snapshotPath, lastSnapshot string
	viewerSelectedKey          string
}

func NewFT8Panel(screen *MainScreen) *FT8Panel {
	p := &FT8Panel{BaseElement: simpleui.NewBaseElement("ft8MapOverlay", 0, 0, designWidth, designHeight), screen: screen, tableSelected: -1, mapSelected: -1, mapLat: 15, mapSpan: 300, mapWindow: time.Hour, mapFilter: "all"}
	p.enable = simpleui.NewSwitch("ft8Enabled", 382, toolY+38, 185, 30, "DECODER", false, uiMinimumFontSize)
	p.enable.SetTrackColors(colors.panelAlt, colors.green)
	p.enable.OnChange(func(active bool) {
		if screen.receiver != nil {
			screen.receiver.ConfigureFT8(active)
		}
	})
	p.snapshotPath = ft8ViewerSnapshotPath()
	p.depth = simpleui.NewDropdown("ft8Depth", 382, toolY+184, 105, 30, "PROFUNDIDAD", []string{"RÁPIDO", "EQUILIBRADO", "PROFUNDO"}, uiCaptionFontSize)
	depthIndex := 1
	switch screen.ft8Depth {
	case ft8.DepthFast:
		depthIndex = 0
	case ft8.DepthDeep:
		depthIndex = 2
	default:
		screen.ft8Depth = ft8.DepthBalanced
	}
	p.depth.SetSelected(depthIndex)
	p.depth.OnChange(func(index int, _ string) {
		profiles := []string{ft8.DepthFast, ft8.DepthBalanced, ft8.DepthDeep}
		if index < 0 || index >= len(profiles) {
			return
		}
		screen.ft8Depth = profiles[index]
		if screen.receiver != nil {
			screen.receiver.SetFT8Depth(screen.ft8Depth)
		}
		screen.markSettingsDirty()
	})
	p.clear = simpleui.NewButton("ft8Clear", 493, toolY+184, 74, 30, "LIMPIAR", uiCaptionFontSize)
	p.clear.SetColors(mixColor(colors.panelAlt, colors.red, .12), colors.red, colors.text)
	p.clear.OnClick(func() {
		if screen.receiver != nil {
			screen.receiver.ClearFT8()
		}
	})
	p.openActivity = simpleui.NewButton("ft8OpenActivity", 382, toolY+220, 185, 28, "ABRIR ACTIVIDAD GLOBAL", uiCaptionFontSize)
	p.openActivity.SetColors(mixColor(colors.panelAlt, colors.cyan, .1), colors.cyan, colors.text)
	p.openActivity.OnClick(func() { p.openExternalViewer(-1) })
	p.controls = []simpleui.Element{p.enable, p.clear, p.depth, p.openActivity}
	if screen.receiver != nil {
		screen.receiver.SetFT8Depth(screen.ft8Depth)
	}
	p.SetVisible(false)
	return p
}

func (p *FT8Panel) Enter() {
	if p.screen.mode != nil {
		for index, item := range p.screen.mode.Items() {
			if item == i18n.Source("text.61f0acff1735") {
				p.screen.mode.SetSelected(index)
				break
			}
		}
	}
	p.screen.savedMode = i18n.Source("text.61f0acff1735")
	p.screen.demodBandwidthHz = 4_000
	if p.screen.filter != nil {
		p.screen.filter.SetLabel("FIL 4 kHz")
	}
	if p.screen.receiver != nil {
		p.screen.receiver.SetDemodulator(i18n.Source("text.61f0acff1735"), p.screen.frequencyHz, 4_000)
	}
	p.enable.SetActive(true)
	if p.screen.receiver != nil {
		p.screen.receiver.ConfigureFT8(true)
	}
}

func (p *FT8Panel) Leave() {
	p.mapOpen = false
	p.enable.SetActive(false)
	if p.screen.receiver != nil {
		p.screen.receiver.ConfigureFT8(false)
	}
}

func (p *FT8Panel) Close() {
	p.Leave()
	p.closeExternalViewer()
}

func (p *FT8Panel) OverlayOpen() bool { return p.mapOpen }

func (p *FT8Panel) Draw() {}

func ft8MainTableBounds() rl.Rectangle {
	return rl.Rectangle{X: 597, Y: toolY + 60, Width: 982, Height: 181}
}

func (p *FT8Panel) interactiveTableBounds() rl.Rectangle {
	bounds := ft8MainTableBounds()
	if p.screen.utilitiesCollapsed {
		bounds = collapsedToolBounds(bounds)
	}
	return bounds
}

func (p *FT8Panel) Update(input simpleui.Input) bool {
	if p.mapOpen || !p.Visible() {
		return false
	}
	bounds := p.interactiveTableBounds()
	maxOffset := max(0, len(p.status.Decodes)-6)
	if input.Over(bounds) && input.Wheel != 0 {
		p.tableOffset = min(max(p.tableOffset-int(input.Wheel)*3, 0), maxOffset)
		return true
	}
	if input.Released && input.Over(bounds) && input.Pointer.Y >= bounds.Y+30 {
		rowHeight := (bounds.Height - 30) / 6
		row := int((input.Pointer.Y - bounds.Y - 30) / rowHeight)
		index := p.tableOffset + row
		if row >= 0 && row < 6 && index >= 0 && index < len(p.status.Decodes) {
			now := time.Now()
			item := p.status.Decodes[index]
			key := fmt.Sprintf("%d|%d|%.3f|%.1f|%s", item.Slot.Unix(), item.SNR, item.DT, item.Frequency, item.Message)
			double := key == p.lastClickKey && now.Sub(p.lastClickAt) <= 450*time.Millisecond
			p.tableSelected = index
			p.lastClickKey, p.lastClickAt = key, now
			if double {
				p.openMap(index)
			}
			return true
		}
	}
	return false
}

func (p *FT8Panel) Tick() {
	if time.Now().Before(p.nextStatusRefresh) {
		return
	}
	p.nextStatusRefresh = time.Now().Add(100 * time.Millisecond)
	if p.screen.receiver != nil {
		p.status = p.screen.receiver.FT8Status()
	}
	if p.mapOpen {
		p.refreshOpenMap()
	}
	p.updateExternalViewer()
	p.tableOffset = min(p.tableOffset, max(0, len(p.status.Decodes)-6))
	if p.tableSelected >= len(p.status.Decodes) {
		p.tableSelected = -1
	}
}

func (p *FT8Panel) SetVisible(visible bool) {
	p.BaseElement.SetVisible(visible)
	for _, control := range p.controls {
		control.SetVisible(visible)
	}
}

func (p *FT8Panel) DrawPanel() {
	status := p.status
	drawFT8Card(rl.Rectangle{X: 365, Y: toolY + 8, Width: 220, Height: 248}, colors.cyan)
	drawFT8Card(rl.Rectangle{X: 597, Y: toolY + 8, Width: 982, Height: 248}, colors.blue)
	simpleui.DrawTextStyled("RECEPCIÓN FT8", 382, toolY+15, uiSectionFontSize, simpleui.FontSemiBold, colors.cyan)

	stateColor := colors.orange
	if status.State == "ESCUCHANDO" || status.State == "DECODIFICANDO" {
		stateColor = colors.green
	} else if status.State == "ERROR" || status.State == "NO DISPONIBLE" {
		stateColor = colors.red
	}
	simpleui.DrawTextStyled(status.State, 382, toolY+84, uiMinimumFontSize, simpleui.FontSemiBold, stateColor)
	detail := status.Detail
	if detail == "" {
		detail = "Sincronizado con ciclos UTC de 15 s"
	}
	drawFT8Wrapped(detail, 382, toolY+108, 28, 2, colors.muted)
	clockText, clockColor := ft8ClockPresentation(status)
	simpleui.DrawTextStyled(clockText, 382, toolY+154, uiCaptionFontSize, simpleui.FontSemiBold, clockColor)
	simpleui.DrawText(fmt.Sprintf("TOTAL  %d", status.DecodeCount), 382, toolY+169, uiCaptionFontSize, colors.text)

	bar := rl.Rectangle{X: 608, Y: toolY + 45, Width: 955, Height: 7}
	rl.DrawRectangleRounded(bar, .5, 6, colors.panelAlt)
	displaySlot, displayProgress := status.Slot, status.Progress
	if !status.Waiting {
		playbackDelay := time.Duration(0)
		if p.screen.audioPlayer != nil {
			playbackDelay = p.screen.audioPlayer.PlaybackDelay()
		}
		displaySlot, displayProgress = ft8UTCCycleProgress(time.Now(), playbackDelay)
	}
	if status.Waiting {
		displayProgress = 0
	}
	fill := bar
	fill.Width *= max(float32(0), min(displayProgress, float32(1)))
	if fill.Width > 0 {
		rl.DrawRectangleRounded(fill, .5, 6, colors.cyan)
	}
	slot := "PRÓXIMO CICLO —"
	if status.Waiting && !status.WaitUntil.IsZero() {
		remaining := max(time.Until(status.WaitUntil), time.Duration(0))
		slot = fmt.Sprintf("ESPERANDO PRÓXIMO CICLO  %s UTC  ·  %.1f s", status.WaitUntil.UTC().Format("15:04:05"), remaining.Seconds())
	} else if !displaySlot.IsZero() {
		slot = "CICLO UTC  " + displaySlot.UTC().Format("15:04:05")
	}
	simpleui.DrawTextStyled(slot, 610, toolY+18, uiCaptionFontSize, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText("UTC        SNR     DT       AUDIO       MENSAJE", 610, toolY+66, uiCaptionFontSize, colors.muted)
	rl.DrawLine(608, int32(toolY+87), 1562, int32(toolY+87), colors.border)

	selection, _ := tableSelectionColors()
	for row := 0; row < 6; row++ {
		y := toolY + 94 + float32(row)*25
		index := p.tableOffset + row
		if index == p.tableSelected {
			rl.DrawRectangle(608, int32(y-3), 955, 24, selection)
		} else if row%2 == 0 {
			rl.DrawRectangle(608, int32(y-3), 955, 24, mixColor(colors.panelAlt, colors.blue, .06))
		}
		if index >= len(status.Decodes) {
			continue
		}
		item := status.Decodes[index]
		simpleui.DrawTextStyled(item.Slot.UTC().Format("15:04:05"), 610, y, uiCaptionFontSize, simpleui.FontMono, colors.muted)
		snrColor := colors.green
		if item.SNR < -18 {
			snrColor = colors.orange
		}
		simpleui.DrawTextStyled(fmt.Sprintf("%+3d", item.SNR), 715, y, uiCaptionFontSize, simpleui.FontMono, snrColor)
		simpleui.DrawTextStyled(fmt.Sprintf("%+4.1f", item.DT), 780, y, uiCaptionFontSize, simpleui.FontMono, colors.text)
		simpleui.DrawTextStyled(fmt.Sprintf("%4.0f Hz", item.Frequency), 850, y, uiCaptionFontSize, simpleui.FontMono, colors.text)
		simpleui.DrawTextStyled(clipFT8(item.Message, 62), 955, y, uiBodyFontSize, simpleui.FontSemiBold, colors.text)
	}
	if len(status.Decodes) > 6 {
		track := rl.Rectangle{X: 1566, Y: toolY + 92, Width: 4, Height: 150}
		rl.DrawRectangleRounded(track, 1, 4, colors.panelAlt)
		thumb := track
		thumb.Height = max(float32(20), track.Height*6/float32(len(status.Decodes)))
		if len(status.Decodes) > 6 {
			thumb.Y += (track.Height - thumb.Height) * float32(p.tableOffset) / float32(len(status.Decodes)-6)
		}
		rl.DrawRectangleRounded(thumb, 1, 4, colors.cyan)
	}
	simpleui.DrawText("Rueda para desplazarte · doble clic para abrir el mapa", 610, toolY+225, 10, colors.muted)
	if status.Dropped > 0 {
		simpleui.DrawText(fmt.Sprintf("Ciclos omitidos: %d", status.Dropped), 1390, toolY+224, uiCaptionFontSize, colors.orange)
	} else if status.ResyncCount > 0 {
		simpleui.DrawText(fmt.Sprintf("Resincronizaciones: %d", status.ResyncCount), 1375, toolY+224, uiCaptionFontSize, colors.orange)
	}
}

func ft8AudibleProgress(slot time.Time, progress float32, delay time.Duration) (time.Time, float32) {
	if slot.IsZero() || delay <= 0 {
		return slot, progress
	}
	progress -= float32(delay.Seconds() / ft8.SlotSeconds)
	for progress < 0 {
		progress++
		slot = slot.Add(-ft8.SlotSeconds * time.Second)
	}
	for progress >= 1 {
		progress--
		slot = slot.Add(ft8.SlotSeconds * time.Second)
	}
	return slot, progress
}

// ft8UTCCycleProgress derives the presentation directly from the UTC clock on
// every frame. It deliberately does not accumulate frame time or decoder
// samples, so a slow frame cannot leave the bar permanently displaced. The
// playback delay keeps the visual cycle aligned with the audio at the speaker.
func ft8UTCCycleProgress(now time.Time, playbackDelay time.Duration) (time.Time, float32) {
	if playbackDelay < 0 {
		playbackDelay = 0
	}
	audibleNow := now.UTC().Add(-playbackDelay)
	slotDuration := ft8.SlotSeconds * time.Second
	slot := audibleNow.Truncate(slotDuration)
	progress := float32(audibleNow.Sub(slot)) / float32(slotDuration)
	return slot, max(float32(0), min(progress, float32(1)))
}

func ft8ClockPresentation(status ft8.Status) (string, rl.Color) {
	if status.ClockSamples < 3 {
		return "RELOJ  CALIBRANDO", colors.muted
	}
	offset := status.ClockOffset
	absolute := offset
	if absolute < 0 {
		absolute = -absolute
	}
	if absolute <= .5 {
		return fmt.Sprintf("RELOJ  CORRECTO  ·  DT %+.1f s", offset), colors.green
	}
	if absolute <= 1 {
		return fmt.Sprintf("RELOJ  ACEPTABLE  ·  DT %+.1f s", offset), colors.orange
	}
	return fmt.Sprintf("RELOJ  REVISAR  ·  DT %+.1f s", offset), colors.red
}

func drawFT8Card(bounds rl.Rectangle, accent rl.Color) {
	background := mixColor(colors.panel, accent, .035)
	rl.DrawRectangleRounded(bounds, .035, 8, background)
	rl.DrawRectangleRoundedLinesEx(bounds, .035, 8, 1, mixColor(colors.border, accent, .35))
}

func drawFT8Wrapped(text string, x, y float32, maxChars, maxLines int, color rl.Color) {
	words, line, lines := strings.Fields(text), "", 0
	for _, word := range words {
		candidate := strings.TrimSpace(line + " " + word)
		if len(candidate) > maxChars && line != "" {
			simpleui.DrawText(line, x, y+float32(lines)*17, uiCaptionFontSize, color)
			lines++
			line = word
			if lines == maxLines {
				return
			}
		} else {
			line = candidate
		}
	}
	if line != "" && lines < maxLines {
		simpleui.DrawText(line, x, y+float32(lines)*17, uiCaptionFontSize, color)
	}
}

func clipFT8(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:max(0, limit-1)] + "…"
}
