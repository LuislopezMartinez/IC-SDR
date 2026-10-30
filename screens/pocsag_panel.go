package screens

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go-zero/internal/i18n"
	"go-zero/internal/pocsag"
	"go-zero/internal/resources"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const pocsagToolID = "POCSAG"

type POCSAGPanel struct {
	simpleui.BaseElement
	screen                           *MainScreen
	controls                         []simpleui.Element
	enable                           *simpleui.Switch
	pause, edit, open, export, clear *simpleui.Button
	presets                          []*simpleui.Button
	protocolButtons                  []*simpleui.Button
	presetFrequencies                []int64
	presetDefinitions                []pocsagPreset
	activePreset                     int
	editorFields                     []*simpleui.TextField
	editorOpen                       bool
	editorPressed                    int
	editorError                      string
	status                           pocsag.Status
	selected, scroll                 int
	nextRefresh                      time.Time
	snapshotPath, lastSnapshot       string
	viewer                           *exec.Cmd
	viewerDone                       chan struct{}
	feedback                         string
	feedbackUntil                    time.Time
	protocol                         pocsag.Protocol
}

type pocsagPreset struct {
	Label       string `json:"label"`
	Description string `json:"description"`
	FrequencyHz int64  `json:"frequencyHz"`
}

func defaultPagingPresets(protocol pocsag.Protocol) []pocsagPreset {
	if protocol == pocsag.ProtocolFLEX {
		return []pocsagPreset{
			{"PAÍSES BAJOS", "Red FLEX observada en 169.650 MHz", 169_650_000},
			{"FLEX 2", "Configure aquí una red FLEX local", 0},
			{"FLEX 3", "Configure aquí una red FLEX local", 0},
			{"FLEX 4", "Configure aquí una red FLEX local", 0},
			{"CUSTOM", "Configure aquí su frecuencia", 0},
		}
	}
	defaults := []pocsagPreset{
		{"BÉLGICA", "ASTRID — red de radiomensajería POCSAG 2400", 169_625_000},
		{"SUIZA", "Swiss Paging — canal nacional F5", 169_700_000},
		{"ALEMANIA", "Digitalalarm — alertas de bomberos y emergencias", 169_810_000},
		{"ALEMANIA", "Cityruf / Euromessage — radiomensajería POCSAG", 466_075_000},
		{"CUSTOM", "Configure aquí su frecuencia", 0},
	}
	return defaults
}

func pagingPresetsPath(protocol pocsag.Protocol) string {
	name := "pocsag-presets.json"
	if protocol == pocsag.ProtocolFLEX {
		name = "flex-presets.json"
	}
	return resources.WritablePath("config", name)
}

func loadPagingPresets(protocol pocsag.Protocol) []pocsagPreset {
	defaults := defaultPagingPresets(protocol)
	path := pagingPresetsPath(protocol)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		_ = os.MkdirAll(filepath.Dir(path), 0755)
		if encoded, encodeErr := json.MarshalIndent(defaults, "", "  "); encodeErr == nil {
			_ = os.WriteFile(path, encoded, 0644)
		}
		return defaults
	}
	var configured []pocsagPreset
	if err != nil || json.Unmarshal(data, &configured) != nil || len(configured) == 0 {
		return defaults
	}
	valid := configured[:0]
	for index, preset := range configured {
		if preset.FrequencyHz >= 0 && preset.Label != "" {
			legacyDescriptions := map[int64]string{169_625_000: "ASTRID · POCSAG 2400", 169_700_000: "Swiss Paging · canal F5", 169_810_000: "Digitalalarm · emergencias", 466_075_000: "Cityruf / Euromessage"}
			if preset.Description == "" && index < len(defaults) {
				preset.Description = defaults[index].Description
				preset.Label = defaults[index].Label
			} else if legacyDescriptions[preset.FrequencyHz] == preset.Description && index < len(defaults) {
				preset.Description = defaults[index].Description
			}
			valid = append(valid, preset)
		}
	}
	if len(valid) == 0 {
		return defaults
	}
	for len(valid) < len(defaults) {
		valid = append(valid, defaults[len(valid)])
	}
	if len(valid) > 5 {
		valid = valid[:5]
	}
	return valid
}

type pagingPreference struct {
	Protocol pocsag.Protocol `json:"protocol"`
}

func pagingPreferencePath() string { return resources.WritablePath("config", "paging.json") }

func loadPagingProtocol() pocsag.Protocol {
	data, err := os.ReadFile(pagingPreferencePath())
	var preference pagingPreference
	if err == nil && json.Unmarshal(data, &preference) == nil && preference.Protocol == pocsag.ProtocolFLEX {
		return pocsag.ProtocolFLEX
	}
	return pocsag.ProtocolPOCSAG
}

func savePagingProtocol(protocol pocsag.Protocol) {
	data, err := json.MarshalIndent(pagingPreference{Protocol: protocol}, "", "  ")
	if err == nil {
		err = os.MkdirAll(filepath.Dir(pagingPreferencePath()), 0755)
	}
	if err == nil {
		_ = os.WriteFile(pagingPreferencePath(), data, 0644)
	}
}

func NewPOCSAGPanel(screen *MainScreen) *POCSAGPanel {
	p := &POCSAGPanel{BaseElement: simpleui.NewBaseElement("pocsagTable", 0, 0, designWidth, designHeight), screen: screen, selected: -1, activePreset: -1, snapshotPath: resources.WritablePath("cache", "pocsag-messages.json"), protocol: loadPagingProtocol()}
	p.enable = simpleui.NewSwitch("pocsagEnabled", 42, toolY+42, 180, 30, "DECODER", false, uiMinimumFontSize)
	p.enable.SetTrackColors(colors.panelAlt, colors.green)
	p.enable.OnChange(func(active bool) {
		if screen.receiver != nil {
			screen.receiver.ConfigurePOCSAG(active)
		}
	})
	button := func(id string, x, y, w, h float32, label string, color rl.Color, action func()) *simpleui.Button {
		b := simpleui.NewButton(id, x, y, w, h, label, uiCaptionFontSize)
		b.SetColors(color, colors.border, colors.text)
		b.OnClick(action)
		p.controls = append(p.controls, b)
		return b
	}
	for index, protocol := range []pocsag.Protocol{pocsag.ProtocolPOCSAG, pocsag.ProtocolFLEX} {
		selected := protocol
		p.protocolButtons = append(p.protocolButtons, button("pagingProtocol"+string(protocol), 42+float32(index)*102, toolY+10, 96, 28, string(protocol), colors.panelAlt, func() { p.selectProtocol(selected) }))
	}
	definitions := loadPagingPresets(p.protocol)
	p.presetDefinitions = append([]pocsagPreset(nil), definitions...)
	for index, definition := range definitions {
		hz := definition.FrequencyHz
		label := fmt.Sprintf("%.3f MHz", float64(hz)/1e6)
		if hz < 1_000 {
			label = "CUSTOM"
		}
		preset := button("pocsagPreset"+strconv.Itoa(index), 42+float32(index)*170, toolY+96, 160, 44, label, colors.panelAlt, func() {
			currentHz := p.presetFrequencies[index]
			if currentHz < 1_000 {
				p.openPresetEditor()
				return
			}
			p.selectFrequency(currentHz)
		})
		p.presets = append(p.presets, preset)
		p.presetFrequencies = append(p.presetFrequencies, hz)
	}
	p.pause = button("pocsagPause", 42, toolY+218, 112, 28, "PAUSAR", colors.panelAlt, p.togglePause)
	p.edit = button("pocsagEdit", 160, toolY+218, 154, 28, "EDITAR FREC.", colors.panelAlt, p.openPresetEditor)
	p.open = button("pocsagOpen", 320, toolY+218, 126, 28, "TABLA", colors.blue, p.openViewer)
	p.export = button("pocsagExport", 452, toolY+218, 132, 28, "EXPORTAR", colors.green, p.exportCSV)
	p.clear = button("pocsagClear", 590, toolY+218, 112, 28, "LIMPIAR", actionClearFill, p.clearEvents)
	p.controls = append([]simpleui.Element{p.enable}, p.controls...)
	p.styleProtocolButtons()
	p.stylePresets()
	p.SetVisible(false)
	return p
}

func (p *POCSAGPanel) Enter() {
	nfm := i18n.Source("text.0896d612d497")
	if p.screen.mode != nil {
		for index, item := range p.screen.mode.Items() {
			if item == nfm {
				p.screen.mode.SetSelected(index)
				break
			}
		}
	}
	p.screen.savedMode = nfm
	p.screen.demodBandwidthHz = 12_500
	if p.screen.filter != nil {
		p.screen.filter.SetLabel("FIL 12.5 kHz")
	}
	if p.screen.receiver != nil {
		p.screen.receiver.SetPOCSAGProtocol(p.protocol)
		p.screen.receiver.SetDemodulator(nfm, p.screen.frequencyHz, 12_500)
		p.screen.receiver.SetPOCSAGFrequency(p.screen.frequencyHz)
		p.screen.receiver.ConfigurePOCSAG(true)
	}
	p.enable.SetActive(true)
	p.stylePresets()
}

func (p *POCSAGPanel) Leave() {
	p.editorOpen = false
	p.enable.SetActive(false)
	if p.screen.receiver != nil {
		p.screen.receiver.ConfigurePOCSAG(false)
	}
}

func (p *POCSAGPanel) Close() {
	p.Leave()
	if p.viewer != nil && p.viewer.Process != nil {
		_ = p.viewer.Process.Kill()
	}
}

func (p *POCSAGPanel) SetVisible(visible bool) {
	p.BaseElement.SetVisible(visible)
	for _, control := range p.controls {
		control.SetVisible(visible)
	}
}

func (p *POCSAGPanel) Draw() {}

func (p *POCSAGPanel) Tick() {
	if time.Now().Before(p.nextRefresh) {
		return
	}
	p.nextRefresh = time.Now().Add(100 * time.Millisecond)
	if p.screen.receiver != nil {
		p.status = p.screen.receiver.POCSAGStatus()
		filtered := p.status.Events[:0]
		for _, event := range p.status.Events {
			if event.Protocol == p.protocol {
				filtered = append(filtered, event)
			}
		}
		p.status.Events = filtered
	}
	p.stylePresets()
	p.scroll = min(p.scroll, max(0, len(p.status.Events)-6))
	if p.selected >= len(p.status.Events) {
		p.selected = -1
	}
	p.writeSnapshot()
	if p.viewerDone != nil {
		select {
		case <-p.viewerDone:
			p.viewer, p.viewerDone, p.lastSnapshot = nil, nil, ""
		default:
		}
	}
}

func (p *POCSAGPanel) Update(input simpleui.Input) bool {
	// DrawPanel uses the shared legacy workspace transform. Convert pointer
	// coordinates back to that same space before hit-testing table rows.
	pointer := toolPointerForLayout(input.Pointer, p.screen.utilitiesCollapsed)
	pointer.X = legacyToolX + (pointer.X-toolContentX)/toolContentScaleX
	input.Pointer = pointer
	bounds := rl.Rectangle{X: 936, Y: toolY + 52, Width: 624, Height: 188}
	if input.Over(bounds) && input.Wheel != 0 {
		p.scroll = min(max(p.scroll-int(input.Wheel)*3, 0), max(0, len(p.status.Events)-6))
		return true
	}
	if input.Released && input.Over(bounds) && input.Pointer.Y >= bounds.Y+28 {
		row := int((input.Pointer.Y - bounds.Y - 28) / 25)
		index := p.scroll + row
		if row >= 0 && row < 6 && index < len(p.status.Events) {
			p.selected = index
			return true
		}
	}
	return false
}

func (p *POCSAGPanel) DrawPanel() {
	stateColor := colors.orange
	if p.status.State == "ESCUCHANDO" || p.status.State == "DECODIFICANDO" {
		stateColor = colors.green
	} else if p.status.State == "ERROR" || p.status.State == "NO DISPONIBLE" {
		stateColor = colors.red
	}
	drawPanel(24, toolY+8, 900, 248)
	drawPanel(936, toolY+8, 640, 248)
	simpleui.DrawTextStyled(p.status.State, 250, toolY+20, uiMinimumFontSize, simpleui.FontSemiBold, stateColor)
	detail := "Runtime multimon-ng · PCM 22.050 Hz"
	if p.status.Error != "" {
		detail = short(p.status.Error, 48)
	}
	simpleui.DrawText(detail, 370, toolY+22, uiCaptionFontSize, colors.muted)
	baud := "AUTO 512 / 1200 / 2400"
	if p.protocol == pocsag.ProtocolFLEX {
		baud = "AUTO FLEX / FLEX_NEXT"
	}
	if p.status.LastBaud > 0 {
		baud = fmt.Sprintf("AUTO · ÚLTIMO %d BAUD", p.status.LastBaud)
	}
	simpleui.DrawTextStyled("NFM 12.5 kHz", 250, toolY+54, uiBodyFontSize, simpleui.FontSemiBold, colors.text)
	simpleui.DrawTextStyled(baud, 390, toolY+54, uiBodyFontSize, simpleui.FontSemiBold, colors.text)
	// Keep live telemetry inside the left panel. Two short, stable lines avoid
	// variable counters growing into the terminal header on the right.
	simpleui.DrawText(fmt.Sprintf("MSG %d  ·  COLA %d  ·  DROP %d", len(p.status.Events), p.status.Queued, p.status.Dropped), 650, toolY+45, uiCaptionFontSize, colors.muted)
	diagnostic := fmt.Sprintf("NIVEL %.0f dBFS  ·  PID %d", p.status.InputPeakDBFS, p.status.ProcessID)
	if p.protocol == pocsag.ProtocolFLEX && p.status.LastBaud > 0 {
		diagnostic = fmt.Sprintf("SYNC L%d · FASE %s · C%02d F%03d", p.status.LastSyncLevel, p.status.LastPhase, p.status.LastCycle, p.status.LastFrame)
	}
	simpleui.DrawText(diagnostic, 650, toolY+61, uiCaptionFontSize, colors.muted)
	for index, preset := range p.presetDefinitions {
		x := 42 + float32(index)*170
		simpleui.DrawTextStyled(preset.Label, x+8, toolY+79, uiBodyFontSize, simpleui.FontSemiBold, colors.cyan)
		if index == p.activePreset {
			rl.DrawCircle(int32(x+148), int32(toolY+86), 5, colors.cyan)
		}
	}
	p.drawPresetDetail()

	simpleui.DrawTextStyled("TERMINAL DE MENSAJES", 952, toolY+17, uiSectionFontSize, simpleui.FontSemiBold, colors.cyan)
	headers := []struct {
		x    float32
		text string
	}{{952, "HORA"}, {1035, "CAPCODE"}, {1137, map[bool]string{true: "FASE", false: "FUN"}[p.protocol == pocsag.ProtocolFLEX]}, {1182, "BAUD"}, {1242, "TIPO"}, {1300, "MENSAJE"}}
	for _, h := range headers {
		simpleui.DrawTextStyled(h.text, h.x, toolY+50, uiBodyFontSize, simpleui.FontSemiBold, colors.muted)
	}
	rl.DrawLine(950, int32(toolY+72), 1560, int32(toolY+72), colors.border)
	for row := 0; row < 6 && p.scroll+row < len(p.status.Events); row++ {
		index := p.scroll + row
		e := p.status.Events[index]
		y := toolY + 79 + float32(row)*25
		if index == p.selected {
			rl.DrawRectangle(946, int32(y-3), 618, 23, rl.Color{R: 25, G: 83, B: 116, A: 220})
		} else if row%2 == 0 {
			rl.DrawRectangle(946, int32(y-3), 618, 23, colors.panelAlt)
		}
		simpleui.DrawTextStyled(e.Received.Format("15:04:05"), 952, y-1, uiBodyFontSize, simpleui.FontMono, colors.text)
		simpleui.DrawTextStyled(fmt.Sprintf("%07d", e.Address), 1035, y-1, uiBodyFontSize, simpleui.FontMono, colors.text)
		function := strconv.Itoa(e.Function)
		if e.Protocol == pocsag.ProtocolFLEX {
			function = e.Phase
		}
		simpleui.DrawText(function, 1147, y-1, uiBodyFontSize, colors.text)
		simpleui.DrawText(strconv.Itoa(e.Baud), 1182, y-1, uiBodyFontSize, colors.text)
		simpleui.DrawText(e.Kind, 1242, y-1, uiBodyFontSize, colors.text)
		simpleui.DrawText(short(e.Message, 31), 1300, y-1, uiBodyFontSize, colors.text)
	}
	if time.Now().Before(p.feedbackUntil) {
		simpleui.DrawTextStyled(p.feedback, 1190, toolY+222, uiCaptionFontSize, simpleui.FontSemiBold, colors.green)
	}
}

func (p *POCSAGPanel) selectProtocol(protocol pocsag.Protocol) {
	if protocol != pocsag.ProtocolPOCSAG && protocol != pocsag.ProtocolFLEX {
		return
	}
	p.protocol = protocol
	p.selected, p.scroll = -1, 0
	p.applyProtocolPresets()
	if p.screen.settingsPath != "" {
		savePagingProtocol(protocol)
	}
	if p.screen.receiver != nil {
		p.screen.receiver.SetPOCSAGProtocol(protocol)
	}
	p.styleProtocolButtons()
}

func (p *POCSAGPanel) applyProtocolPresets() {
	definitions := loadPagingPresets(p.protocol)
	p.presetDefinitions = append(p.presetDefinitions[:0], definitions...)
	p.presetFrequencies = p.presetFrequencies[:0]
	for index, preset := range definitions {
		p.presetFrequencies = append(p.presetFrequencies, preset.FrequencyHz)
		if index < len(p.presets) {
			label := "CUSTOM"
			if preset.FrequencyHz >= 1_000 {
				label = fmt.Sprintf("%.3f MHz", float64(preset.FrequencyHz)/1e6)
			}
			p.presets[index].SetLabel(label)
		}
	}
	p.stylePresets()
}

func (p *POCSAGPanel) styleProtocolButtons() {
	protocols := []pocsag.Protocol{pocsag.ProtocolPOCSAG, pocsag.ProtocolFLEX}
	for index, button := range p.protocolButtons {
		if index < len(protocols) && protocols[index] == p.protocol {
			button.SetColors(mixColor(colors.panelAlt, colors.green, .30), colors.green, colors.text)
		} else {
			button.SetColors(colors.panelAlt, colors.border, colors.text)
		}
	}
}

func (p *POCSAGPanel) selectFrequency(hz int64) {
	if hz < 1_000 {
		return
	}
	p.screen.frequencyHz = hz
	p.screen.centerFrequencyHz = hz
	p.screen.demodBandwidthHz = 12_500
	p.screen.spanHz = max(p.screen.spanHz, int64(500_000))
	if p.screen.filter != nil {
		p.screen.filter.SetLabel("FIL 12.5 kHz")
	}
	if p.screen.receiver != nil {
		p.screen.receiver.SetCenterFrequency(hz)
		p.screen.receiver.SetDemodulator(i18n.Source("text.0896d612d497"), hz, 12_500)
		p.screen.receiver.SetPOCSAGFrequency(hz)
	}
	p.screen.markSettingsDirty()
	p.stylePresets()
}

func (p *POCSAGPanel) stylePresets() {
	selected := false
	p.activePreset = -1
	for index, button := range p.presets {
		active := index < len(p.presetFrequencies) && p.presetFrequencies[index] == p.screen.frequencyHz
		if active {
			selected = true
			p.activePreset = index
			button.SetColors(mixColor(colors.panelAlt, colors.cyan, .28), colors.cyan, colors.text)
		} else {
			button.SetColors(colors.panelAlt, colors.border, colors.text)
		}
	}
	if p.edit != nil {
		if selected {
			p.edit.SetColors(mixColor(colors.panelAlt, colors.cyan, .32), colors.cyan, colors.text)
		} else {
			p.edit.SetColors(colors.panelAlt, colors.border, colors.text)
		}
	}
}

func (p *POCSAGPanel) drawPresetDetail() {
	bounds := rl.Rectangle{X: 42, Y: toolY + 148, Width: 840, Height: 62}
	rl.DrawRectangleRounded(bounds, .08, 6, colors.panelAlt)
	rl.DrawRectangleRoundedLinesEx(bounds, .08, 6, 1, colors.border)
	rl.DrawRectangle(int32(bounds.X), int32(bounds.Y+5), 5, int32(bounds.Height-10), colors.cyan)
	// Compact radio-tower glyph drawn with primitives so it follows theme scaling.
	cx, cy := float32(78), toolY+178
	rl.DrawLineEx(rl.Vector2{X: cx, Y: cy - 15}, rl.Vector2{X: cx - 11, Y: cy + 19}, 2, colors.cyan)
	rl.DrawLineEx(rl.Vector2{X: cx, Y: cy - 15}, rl.Vector2{X: cx + 11, Y: cy + 19}, 2, colors.cyan)
	rl.DrawLineEx(rl.Vector2{X: cx - 7, Y: cy + 7}, rl.Vector2{X: cx + 7, Y: cy + 7}, 2, colors.cyan)
	rl.DrawCircle(int32(cx), int32(cy-16), 3, colors.cyan)
	if p.activePreset < 0 || p.activePreset >= len(p.presetDefinitions) {
		simpleui.DrawTextStyled("SELECCIONE UN PRESET", 112, toolY+157, uiSectionFontSize, simpleui.FontSemiBold, colors.text)
		simpleui.DrawTextStyled("La descripción y frecuencia aparecerán aquí.", 112, toolY+183, uiMinimumFontSize, simpleui.FontRegular, colors.muted)
		return
	}
	preset := p.presetDefinitions[p.activePreset]
	frequency := "SIN CONFIGURAR"
	if preset.FrequencyHz >= 1_000 {
		frequency = fmt.Sprintf("%.3f MHz", float64(preset.FrequencyHz)/1e6)
	}
	simpleui.DrawTextStyled(preset.Label+"  ·  "+frequency, 112, toolY+156, uiSectionFontSize, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawTextStyled(preset.Description, 112, toolY+183, uiMinimumFontSize, simpleui.FontRegular, colors.text)
}

func (p *POCSAGPanel) openPresetEditor() {
	p.editorOpen, p.editorError, p.editorPressed = true, "", 0
	p.editorFields = nil
	for index, preset := range p.presetDefinitions {
		y := float32(242 + index*82)
		label := simpleui.NewTextField("pocsagEditLabel"+strconv.Itoa(index), 350, y, 190, 40, "PAÍS / NOMBRE", 14)
		description := simpleui.NewTextField("pocsagEditDescription"+strconv.Itoa(index), 555, y, 480, 40, "DESCRIPCIÓN", 14)
		frequency := simpleui.NewTextField("pocsagEditFrequency"+strconv.Itoa(index), 1050, y, 200, 40, "MHz", 14)
		label.SetText(preset.Label)
		description.SetText(preset.Description)
		frequency.SetText(fmt.Sprintf("%.6f", float64(preset.FrequencyHz)/1e6))
		frequency.SetFilter(func(r rune) bool { return r >= '0' && r <= '9' || r == '.' || r == ',' })
		p.editorFields = append(p.editorFields, label, description, frequency)
	}
}

func (p *POCSAGPanel) OverlayOpen() bool { return p.editorOpen }

func (p *POCSAGPanel) editorCancelBounds() rl.Rectangle {
	return rl.Rectangle{X: 530, Y: 700, Width: 220, Height: 48}
}

func (p *POCSAGPanel) editorSaveBounds() rl.Rectangle {
	return rl.Rectangle{X: 850, Y: 700, Width: 220, Height: 48}
}

func (p *POCSAGPanel) UpdateOverlay(input simpleui.Input) bool {
	if !p.editorOpen {
		return false
	}
	if rl.IsKeyPressed(rl.KeyEscape) {
		p.editorOpen = false
		return true
	}
	if input.Pressed {
		for _, field := range p.editorFields {
			field.SetFocused(input.Over(field.Bounds()))
		}
	}
	for _, field := range p.editorFields {
		field.Update(input)
	}
	if input.Pressed {
		p.editorPressed = 0
		if input.Over(p.editorCancelBounds()) {
			p.editorPressed = 1
		}
		if input.Over(p.editorSaveBounds()) {
			p.editorPressed = 2
		}
	}
	if input.Released {
		if p.editorPressed == 1 && input.Over(p.editorCancelBounds()) {
			p.editorOpen = false
		}
		if p.editorPressed == 2 && input.Over(p.editorSaveBounds()) {
			p.savePresetEditor()
		}
		p.editorPressed = 0
	}
	return true
}

func (p *POCSAGPanel) savePresetEditor() {
	updated := make([]pocsagPreset, 0, len(p.presetDefinitions))
	for index := range p.presetDefinitions {
		label := strings.TrimSpace(p.editorFields[index*3].Text())
		description := strings.TrimSpace(p.editorFields[index*3+1].Text())
		mhz, err := strconv.ParseFloat(strings.ReplaceAll(p.editorFields[index*3+2].Text(), ",", "."), 64)
		if err != nil || mhz < 0 || mhz > 6000 || label == "" {
			p.editorError = fmt.Sprintf("Revise la fila %d: nombre y frecuencia entre 0 y 6000 MHz.", index+1)
			return
		}
		updated = append(updated, pocsagPreset{Label: label, Description: description, FrequencyHz: int64(math.Round(mhz * 1e6))})
	}
	data, err := json.MarshalIndent(updated, "", "  ")
	if err == nil {
		err = os.MkdirAll(filepath.Dir(pagingPresetsPath(p.protocol)), 0755)
	}
	if err == nil {
		err = os.WriteFile(pagingPresetsPath(p.protocol), data, 0644)
	}
	if err != nil {
		p.editorError = "No se pudo guardar: " + err.Error()
		return
	}
	p.presetDefinitions = updated
	for index, preset := range updated {
		p.presetFrequencies[index] = preset.FrequencyHz
		buttonLabel := "CUSTOM"
		if preset.FrequencyHz >= 1_000 {
			buttonLabel = fmt.Sprintf("%.3f MHz", float64(preset.FrequencyHz)/1e6)
		}
		p.presets[index].SetLabel(buttonLabel)
	}
	p.editorOpen = false
	p.stylePresets()
}

func (p *POCSAGPanel) DrawOverlay() {
	if !p.editorOpen {
		return
	}
	rl.DrawRectangle(0, 0, int32(designWidth), int32(designHeight), rl.Color{A: 220})
	modal := rl.Rectangle{X: 300, Y: 90, Width: 1000, Height: 690}
	rl.DrawRectangleRounded(modal, .025, 8, colors.panel)
	rl.DrawRectangleRoundedLinesEx(modal, .025, 8, 2, colors.cyan)
	simpleui.DrawTextStyled("EDITAR FRECUENCIAS "+string(p.protocol), 330, 118, 22, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText("Los cambios se guardan como presets locales. Frecuencia expresada en MHz.", 330, 152, 14, colors.muted)
	for index := range p.presetDefinitions {
		y := float32(224 + index*82)
		simpleui.DrawTextStyled(fmt.Sprintf("PRESET %d", index+1), 330, y, 12, simpleui.FontSemiBold, colors.muted)
		simpleui.DrawText("NOMBRE / ZONA", 350, y, 11, colors.muted)
		simpleui.DrawText("DESCRIPCIÓN", 555, y, 11, colors.muted)
		simpleui.DrawText("FRECUENCIA MHz", 1050, y, 11, colors.muted)
	}
	for _, field := range p.editorFields {
		field.Draw()
	}
	if p.editorError != "" {
		simpleui.DrawTextStyled(p.editorError, 350, 670, 13, simpleui.FontSemiBold, colors.red)
	}
	drawModalAction(p.editorCancelBounds(), "CANCELAR", colors.border, p.editorPressed == 1)
	drawModalAction(p.editorSaveBounds(), "GUARDAR", colors.green, p.editorPressed == 2)
}

func (p *POCSAGPanel) togglePause() {
	paused := !p.status.Paused
	if p.screen.receiver != nil {
		p.screen.receiver.PausePOCSAG(paused)
	}
	p.pause.SetLabel(map[bool]string{true: "REANUDAR", false: "PAUSAR"}[paused])
}

func (p *POCSAGPanel) clearEvents() {
	if p.screen.receiver != nil {
		p.screen.receiver.ClearPOCSAGProtocol(p.protocol)
	}
	p.selected, p.scroll = -1, 0
}

func (p *POCSAGPanel) writeSnapshot() {
	data, err := json.Marshal(p.status.Events)
	if err != nil || string(data) == p.lastSnapshot {
		return
	}
	_ = os.MkdirAll(filepath.Dir(p.snapshotPath), 0755)
	if replaceLiveSnapshot(p.snapshotPath, data) == nil {
		p.lastSnapshot = string(data)
	}
}

func (p *POCSAGPanel) openViewer() {
	p.writeSnapshot()
	if p.viewer != nil && p.viewer.Process != nil {
		focusRTL433Viewer(p.viewer.Process.Pid)
		return
	}
	executable, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(executable, "--pocsag-viewer", p.snapshotPath)
	cmd.SysProcAttr = rtl433ViewerProcessAttributes()
	if cmd.Start() != nil {
		return
	}
	p.viewer, p.viewerDone = cmd, make(chan struct{})
	done := p.viewerDone
	go func() { _ = cmd.Wait(); close(done) }()
}

func (p *POCSAGPanel) exportCSV() {
	path, err := ExportPOCSAGCSV(p.status.Events)
	if err != nil {
		p.feedback = "ERROR AL EXPORTAR"
	} else {
		p.feedback = "EXPORTADO: " + filepath.Base(path)
	}
	p.feedbackUntil = time.Now().Add(3 * time.Second)
}

func ExportPOCSAGCSV(events []pocsag.Event) (string, error) {
	dir := resources.WritablePath("exports", "pocsag")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "pocsag-"+time.Now().Format("20060102-150405")+".csv")
	file, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	_, _ = file.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(file)
	w.Comma = ';'
	_ = w.Write([]string{"fecha", "hora", "protocolo", "frecuencia_hz", "capcode", "funcion", "baud", "fase", "ciclo", "trama", "tipo", "mensaje", "raw"})
	for _, e := range events {
		_ = w.Write([]string{e.Received.Format("2006-01-02"), e.Received.Format("15:04:05"), string(e.Protocol), strconv.FormatInt(e.FrequencyHz, 10), strconv.FormatUint(e.Address, 10), strconv.Itoa(e.Function), strconv.Itoa(e.Baud), e.Phase, strconv.Itoa(e.Cycle), strconv.Itoa(e.Frame), e.Kind, e.Message, e.Raw})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	jsonPath := strings.TrimSuffix(path, ".csv") + ".json"
	data, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return "", err
	}
	if err = os.WriteFile(jsonPath, data, 0644); err != nil {
		return "", err
	}
	return path, nil
}
