package screens

import (
	"fmt"
	"strings"
	"time"

	"go-zero/internal/tetrapolruntime"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const tetrapolNetworkRows = 14

var (
	tetrapolNetworkModal = rl.Rectangle{X: 70, Y: 55, Width: 1460, Height: 790}
	tetrapolNetworkTable = rl.Rectangle{X: 95, Y: 220, Width: 1410, Height: 500}
)

func (panel *TETRAPOLPanel) openNetwork() {
	panel.networkOpen = true
	panel.networkLive = true
	panel.networkTab = 0
	panel.networkOffset = 0
	panel.networkSnapshot = cloneTETRAPOLTelemetry(panel.latestTelemetry)
}

func cloneTETRAPOLTelemetry(value tetrapolruntime.Telemetry) tetrapolruntime.Telemetry {
	value.RecentEvents = append([]tetrapolruntime.Event(nil), value.RecentEvents...)
	value.Channels = append([]tetrapolruntime.ChannelObservation(nil), value.Channels...)
	value.Groups = append([]tetrapolruntime.GroupObservation(nil), value.Groups...)
	return value
}

func (panel *TETRAPOLPanel) networkTelemetry() tetrapolruntime.Telemetry {
	if panel.networkLive {
		return panel.latestTelemetry
	}
	return panel.networkSnapshot
}

func (panel *TETRAPOLPanel) OverlayOpen() bool          { return panel.networkOpen }
func (panel *TETRAPOLPanel) Update(simpleui.Input) bool { return false }
func (panel *TETRAPOLPanel) Draw()                      {}

func tetrapolNetworkActionBounds(action int) rl.Rectangle {
	switch action {
	case 1:
		return rl.Rectangle{X: 1370, Y: 78, Width: 130, Height: 36}
	case 2:
		return rl.Rectangle{X: 1195, Y: 78, Width: 160, Height: 36}
	case 3, 4, 5, 6:
		return rl.Rectangle{X: 95 + float32(action-3)*185, Y: 160, Width: 170, Height: 40}
	}
	return rl.Rectangle{}
}

func (panel *TETRAPOLPanel) networkItemCount() int {
	telemetry := panel.networkTelemetry()
	switch panel.networkTab {
	case 1:
		return len(telemetry.Channels)
	case 2:
		return len(telemetry.Groups)
	case 3:
		return len(telemetry.RecentEvents)
	default:
		return 0
	}
}

func (panel *TETRAPOLPanel) UpdateOverlay(input simpleui.Input) bool {
	if !panel.networkOpen {
		return false
	}
	if rl.IsKeyPressed(rl.KeyEscape) {
		panel.networkOpen = false
		return true
	}
	maximum := max(0, panel.networkItemCount()-tetrapolNetworkRows)
	if input.Over(tetrapolNetworkTable) && input.Wheel != 0 && panel.networkTab != 0 {
		panel.networkLive = false
		panel.networkSnapshot = cloneTETRAPOLTelemetry(panel.latestTelemetry)
		panel.networkOffset = min(max(panel.networkOffset-int(input.Wheel)*3, 0), maximum)
	}
	if input.Pressed {
		panel.networkPressed = 0
		for action := 1; action <= 6; action++ {
			if input.Over(tetrapolNetworkActionBounds(action)) {
				panel.networkPressed = action
				return true
			}
		}
	}
	if input.Released {
		pressed := panel.networkPressed
		panel.networkPressed = 0
		if pressed != 0 && input.Over(tetrapolNetworkActionBounds(pressed)) {
			switch pressed {
			case 1:
				panel.networkOpen = false
			case 2:
				panel.networkLive = !panel.networkLive
				if !panel.networkLive {
					panel.networkSnapshot = cloneTETRAPOLTelemetry(panel.latestTelemetry)
				}
			case 3, 4, 5, 6:
				panel.networkTab = pressed - 3
				panel.networkOffset = 0
			}
			return true
		}
	}
	panel.networkOffset = min(max(panel.networkOffset, 0), maximum)
	return true
}

func (panel *TETRAPOLPanel) DrawOverlay() {
	if !panel.networkOpen {
		return
	}
	telemetry := panel.networkTelemetry()
	rl.DrawRectangle(0, 0, int32(designWidth), int32(designHeight), rl.Color{R: 0, G: 0, B: 0, A: 215})
	rl.DrawRectangleRounded(tetrapolNetworkModal, .025, 8, colors.panel)
	rl.DrawRectangleRoundedLinesEx(tetrapolNetworkModal, .025, 8, 2, colors.cyan)
	simpleui.DrawTextStyled("RED TETRAPOL", 95, 78, 22, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText("Vista avanzada de señalización, recursos y actividad observada", 95, 107, 12, colors.muted)

	state := "EN VIVO"
	stateColor := colors.green
	if !panel.networkLive {
		state, stateColor = "PAUSADO", colors.orange
	}
	drawModalAction(tetrapolNetworkActionBounds(1), "CERRAR", colors.border, panel.networkPressed == 1)
	drawModalAction(tetrapolNetworkActionBounds(2), map[bool]string{true: "PAUSAR", false: "REANUDAR"}[panel.networkLive], stateColor, panel.networkPressed == 2)
	tabs := []string{"RESUMEN", "CANALES", "GRUPOS", "TRAZAS TSDU"}
	for index, label := range tabs {
		accent := colors.border
		if panel.networkTab == index {
			accent = colors.cyan
		}
		drawModalAction(tetrapolNetworkActionBounds(index+3), label, accent, panel.networkPressed == index+3)
	}
	simpleui.DrawText(state, 1095, 88, 12, stateColor)

	switch panel.networkTab {
	case 0:
		panel.drawTETRAPOLNetworkSummary(telemetry)
	case 1:
		panel.drawTETRAPOLChannels(telemetry)
	case 2:
		panel.drawTETRAPOLGroups(telemetry)
	case 3:
		panel.drawTETRAPOLTraces(telemetry)
	}
	simpleui.DrawText("La información se obtiene de mensajes recibidos: un campo vacío no implica que la red no lo utilice.", 95, 800, 11, colors.muted)
}

func networkCard(bounds rl.Rectangle, title, value, detail string, accent rl.Color) {
	rl.DrawRectangleRounded(bounds, .05, 7, colors.panelAlt)
	rl.DrawRectangleRoundedLinesEx(bounds, .05, 7, 1, accent)
	rl.DrawRectangle(int32(bounds.X), int32(bounds.Y), 4, int32(bounds.Height), accent)
	simpleui.DrawTextStyled(title, bounds.X+16, bounds.Y+13, 11, simpleui.FontSemiBold, accent)
	simpleui.DrawTextStyled(value, bounds.X+16, bounds.Y+39, 19, simpleui.FontMono, colors.text)
	simpleui.DrawText(trimMemory(detail, 43), bounds.X+16, bounds.Y+70, 11, colors.muted)
}

func (panel *TETRAPOLPanel) drawTETRAPOLNetworkSummary(t tetrapolruntime.Telemetry) {
	sync, syncColor := "BUSCANDO", colors.orange
	if t.Synchronized {
		sync, syncColor = "BLOQUEADO", colors.green
	}
	quality := "SIN MUESTRA"
	if t.QualityKnown {
		quality = fmt.Sprintf("%.0f %%", t.QualityPercent)
	}
	cipher := strings.ToUpper(valueOrDash(t.CipherState))
	cipherColor := colors.orange
	if t.CipherState == "clear" {
		cipherColor = colors.green
	} else if t.CipherState == "encrypted" {
		cipherColor = colors.red
	}
	cards := []struct {
		title, value, detail string
		color                rl.Color
	}{
		{"SINCRONISMO", sync, fmt.Sprintf("%d tramas válidas · %.1f CRC/s", t.ValidFrames, t.CRCPerSecond), syncColor},
		{"INTEGRIDAD CRC", quality, fmt.Sprintf("%d errores · %d bits corregidos", t.CRCFailures, t.BitsFixed), colors.cyan},
		{"AUDIO", fmt.Sprintf("%d / PLC %d", t.AudioDecoded, t.AudioConcealed), fmt.Sprintf("%d huecos largos · voz %s", t.AudioLongGaps, map[bool]string{true: "activa", false: "inactiva"}[t.VoiceActive]), colors.green},
		{"PROTECCIÓN", cipher, fmt.Sprintf("tipo %d · índice %d", t.CipherKeyType, t.CipherKeyIndex), cipherColor},
		{"RECURSOS", fmt.Sprintf("%d CH · %d GR", len(t.Channels), len(t.Groups)), "canales y grupos descubiertos", colors.cyan},
		{"SEÑALIZACIÓN", fmt.Sprintf("%d TSDU", t.TSDUCount), fmt.Sprintf("último CODOP %s", tetrapolCODOP(t)), colors.orange},
		{"CANAL LÓGICO", valueOrDash(t.LogicalChannel), fmt.Sprintf("acción %s · canal %d", valueOrDash(t.ChannelAction), t.ChannelID), colors.cyan},
		{"ÚLTIMO REGISTRO", tetrapolRecordAge(t.LastRecord), fmt.Sprintf("trama %s · SCR %s", valueOrDash(t.FrameType), tetrapolSCR(t)), colors.green},
	}
	for index, card := range cards {
		column, row := index%4, index/4
		bounds := rl.Rectangle{X: 95 + float32(column)*352, Y: 230 + float32(row)*160, Width: 335, Height: 135}
		networkCard(bounds, card.title, card.value, card.detail, card.color)
	}
	drawPanel(95, 565, 1393, 120)
	simpleui.DrawTextStyled("LECTURA RÁPIDA", 115, 582, 12, simpleui.FontSemiBold, colors.cyan)
	message := "Esperando señalización válida. Mantén el receptor sobre el canal de control para construir el mapa de red."
	if t.Synchronized {
		message = fmt.Sprintf("Red sincronizada. Se han observado %d canales y %d grupos en %d mensajes TSDU. La lista conserva hasta 500 eventos para análisis.", len(t.Channels), len(t.Groups), t.TSDUCount)
	}
	drawWrapped(message, 115, 610, 1350, 18, colors.text)
}

func drawTETRAPOLTableFrame(headers []struct {
	label string
	x     float32
}) {
	rl.DrawRectangleRec(tetrapolNetworkTable, colors.background)
	rl.DrawRectangleLinesEx(tetrapolNetworkTable, 1, colors.border)
	rl.DrawRectangle(int32(tetrapolNetworkTable.X), int32(tetrapolNetworkTable.Y), int32(tetrapolNetworkTable.Width), 34, colors.panelAlt)
	for _, header := range headers {
		simpleui.DrawText(header.label, header.x, tetrapolNetworkTable.Y+11, 11, colors.cyan)
	}
}

func drawTETRAPOLRow(row int) float32 {
	y := tetrapolNetworkTable.Y + 34 + float32(row)*32
	if row%2 == 0 {
		rl.DrawRectangle(int32(tetrapolNetworkTable.X+1), int32(y), int32(tetrapolNetworkTable.Width-2), 31, colors.panelAlt)
	}
	return y + 9
}

func (panel *TETRAPOLPanel) drawTETRAPOLChannels(t tetrapolruntime.Telemetry) {
	drawTETRAPOLTableFrame([]struct {
		label string
		x     float32
	}{{"ESTADO", 110}, {"TCH", 205}, {"FRECUENCIA", 285}, {"TIPO", 425}, {"GRUPO", 550}, {"COB.", 635}, {"DIRECCIÓN", 710}, {"CIFRADO", 875}, {"VISTAS", 1010}, {"PRIMERA / ÚLTIMA", 1110}})
	for row := 0; row < tetrapolNetworkRows && panel.networkOffset+row < len(t.Channels); row++ {
		channel := t.Channels[panel.networkOffset+row]
		y := drawTETRAPOLRow(row)
		state, stateColor := "REGISTRADO", colors.muted
		if channel.Active {
			state, stateColor = "ACTIVO", colors.green
		}
		frequency := "—"
		if hz, err := tetrapolChannelFrequencyForPlan(panel.planType, panel.controlFrequencyHz, channel.ChannelID); err == nil {
			frequency = fmt.Sprintf("%.5f", float64(hz)/1e6)
		}
		simpleui.DrawText(state, 110, y, 11, stateColor)
		simpleui.DrawTextStyled(fmt.Sprintf("%d", channel.ChannelID), 205, y, 11, simpleui.FontMono, colors.text)
		simpleui.DrawTextStyled(frequency, 285, y, 11, simpleui.FontMono, colors.text)
		simpleui.DrawText(valueOrDash(channel.Kind), 425, y, 11, colors.text)
		simpleui.DrawText(tetrapolOptionalID(channel.GroupID), 550, y, 11, colors.text)
		simpleui.DrawText(tetrapolOptionalID(channel.CoverageID), 635, y, 11, colors.text)
		simpleui.DrawText(valueOrDash(channel.Address), 710, y, 11, colors.text)
		simpleui.DrawText(valueOrDash(channel.CipherState), 875, y, 11, tetrapolCipherColor(channel.CipherState))
		simpleui.DrawText(fmt.Sprintf("%d", channel.Observations), 1010, y, 11, colors.text)
		simpleui.DrawText(tetrapolTimeRange(channel.FirstSeen, channel.LastSeen), 1110, y, 11, colors.muted)
	}
}

func (panel *TETRAPOLPanel) drawTETRAPOLGroups(t tetrapolruntime.Telemetry) {
	drawTETRAPOLTableFrame([]struct {
		label string
		x     float32
	}{{"GRUPO", 110}, {"COBERTURA", 260}, {"CANAL TCH", 430}, {"CIFRADO", 610}, {"OBSERVACIONES", 790}, {"PRIMERA / ÚLTIMA VEZ", 1030}})
	for row := 0; row < tetrapolNetworkRows && panel.networkOffset+row < len(t.Groups); row++ {
		group := t.Groups[panel.networkOffset+row]
		y := drawTETRAPOLRow(row)
		simpleui.DrawTextStyled(fmt.Sprintf("%d", group.GroupID), 110, y, 11, simpleui.FontMono, colors.text)
		simpleui.DrawText(tetrapolOptionalID(group.CoverageID), 260, y, 11, colors.text)
		simpleui.DrawText(tetrapolOptionalID(group.ChannelID), 430, y, 11, colors.text)
		simpleui.DrawText(valueOrDash(group.CipherState), 610, y, 11, tetrapolCipherColor(group.CipherState))
		simpleui.DrawText(fmt.Sprintf("%d", group.Observations), 790, y, 11, colors.text)
		simpleui.DrawText(tetrapolTimeRange(group.FirstSeen, group.LastSeen), 1030, y, 11, colors.muted)
	}
}

func (panel *TETRAPOLPanel) drawTETRAPOLTraces(t tetrapolruntime.Telemetry) {
	drawTETRAPOLTableFrame([]struct {
		label string
		x     float32
	}{{"HORA", 110}, {"EVENTO", 245}, {"DETALLE DECODIFICADO", 420}})
	for row := 0; row < tetrapolNetworkRows && panel.networkOffset+row < len(t.RecentEvents); row++ {
		index := len(t.RecentEvents) - 1 - (panel.networkOffset + row)
		event := t.RecentEvents[index]
		y := drawTETRAPOLRow(row)
		color := colors.text
		if event.Kind == "CRC" || event.Kind == "CIPHER" {
			color = colors.orange
		} else if event.Kind == "SYNC" || event.Kind == "VOICE" || event.Kind == "FOLLOW" {
			color = colors.green
		}
		simpleui.DrawTextStyled(event.Time.Format("15:04:05.000"), 110, y, 11, simpleui.FontMono, colors.muted)
		simpleui.DrawText(event.Kind, 245, y, 11, color)
		simpleui.DrawText(trimMemory(event.Summary, 145), 420, y, 11, colors.text)
	}
}

func tetrapolOptionalID(value int) string {
	if value < 0 {
		return "—"
	}
	return fmt.Sprintf("%d", value)
}

func tetrapolCipherColor(value string) rl.Color {
	if value == "clear" {
		return colors.green
	}
	if value == "encrypted" {
		return colors.red
	}
	return colors.muted
}

func tetrapolTimeRange(first, last time.Time) string {
	if first.IsZero() {
		return "—"
	}
	return first.Format("15:04:05") + " / " + last.Format("15:04:05")
}

func tetrapolRecordAge(value time.Time) string {
	if value.IsZero() {
		return "—"
	}
	return strings.ToUpper(formatTETRAPOLAge(time.Since(value)))
}

func tetrapolCODOP(t tetrapolruntime.Telemetry) string {
	if !t.CODOPKnown {
		return "—"
	}
	return fmt.Sprintf("0x%02X", t.LastCODOP)
}

func tetrapolSCR(t tetrapolruntime.Telemetry) string {
	if !t.SCRKnown {
		return "—"
	}
	return fmt.Sprintf("%d", t.SCR)
}
