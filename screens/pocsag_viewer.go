package screens

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go-zero/internal/pocsag"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type pocsagViewer struct {
	path               string
	events, filtered   []pocsag.Event
	selected, scroll   int
	nextRead           time.Time
	search             *simpleui.TextField
	export, copyButton *simpleui.Button
	feedback           string
	feedbackUntil      time.Time
}

func RunPOCSAGViewer(snapshotPath string) {
	simpleui.SetMode(1500, 820, simpleui.Fit)
	simpleui.SetInitialWindowSize(1500, 820)
	simpleui.SetMinimumSize(960, 540)
	simpleui.SetCanvasFilter(rl.FilterPoint)
	simpleui.SetTextScale(1.25)
	simpleui.SetTitle("IC-SDR · TERMINAL POCSAG")
	simpleui.SetMaximizeKey(0)
	v := &pocsagViewer{path: snapshotPath, selected: -1}
	v.search = simpleui.NewTextField("pocsagViewerSearch", 830, 22, 300, 42, "FILTRAR CAPCODE O MENSAJE", 14)
	v.search.OnChange(func(string) { v.applyFilter(); v.selected, v.scroll = -1, 0 })
	v.export = simpleui.NewButton("pocsagViewerExport", 1150, 22, 160, 42, "EXPORTAR CSV", 14)
	v.export.SetColors(colors.green, colors.border, colors.text)
	v.export.OnClick(v.exportCSV)
	v.copyButton = simpleui.NewButton("pocsagViewerCopy", 1320, 22, 150, 42, "COPIAR", 14)
	v.copyButton.SetColors(colors.blue, colors.border, colors.text)
	v.copyButton.OnClick(v.copySelected)
	simpleui.Add(v.search)
	simpleui.Add(v.export)
	simpleui.Add(v.copyButton)
	simpleui.Run(v.draw)
}

func (v *pocsagViewer) read() {
	if time.Now().Before(v.nextRead) {
		return
	}
	v.nextRead = time.Now().Add(200 * time.Millisecond)
	data, err := os.ReadFile(v.path)
	if err != nil {
		return
	}
	var events []pocsag.Event
	if json.Unmarshal(data, &events) == nil {
		v.events = events
		v.applyFilter()
		if v.selected >= len(v.filtered) {
			v.selected = len(v.filtered) - 1
		}
		v.scroll = min(v.scroll, max(0, len(v.filtered)-24))
	}
}

func (v *pocsagViewer) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(v.search.Text()))
	v.filtered = v.filtered[:0]
	for _, event := range v.events {
		frequency := fmt.Sprintf("%.6f", float64(event.FrequencyHz)/1e6)
		if query == "" || strings.Contains(strings.ToLower(event.Message), query) || strings.Contains(strconv.FormatUint(event.Address, 10), query) || strings.Contains(strings.ToLower(event.Kind), query) || strings.Contains(strconv.Itoa(event.Function), query) || strings.Contains(strconv.Itoa(event.Baud), query) || strings.Contains(frequency, query) {
			v.filtered = append(v.filtered, event)
		}
	}
}

func (v *pocsagViewer) draw() {
	v.read()
	if controlCopyPressed() {
		v.copySelected()
	}
	rl.DrawRectangle(0, 0, 1500, 820, colors.background)
	simpleui.DrawTextStyled("TERMINAL POCSAG", 28, 22, 22, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText(fmt.Sprintf("%d mensajes · rueda/cursores para desplazarse", len(v.filtered)), 28, 52, 13, colors.muted)
	if time.Now().Before(v.feedbackUntil) {
		simpleui.DrawTextStyled(v.feedback, 500, 35, 13, simpleui.FontSemiBold, colors.green)
	}

	headers := []struct {
		x    float32
		text string
	}{{28, "FECHA/HORA"}, {210, "MHz"}, {320, "CAPCODE"}, {440, "FUNCIÓN"}, {525, "BAUD"}, {600, "TIPO"}, {680, "MENSAJE"}}
	rl.DrawRectangle(20, 82, 1460, 34, colors.panelAlt)
	for _, header := range headers {
		simpleui.DrawTextStyled(header.text, header.x, 89, 14, simpleui.FontSemiBold, colors.cyan)
	}
	if wheel := rl.GetMouseWheelMove(); wheel != 0 {
		v.scroll = min(max(v.scroll-int(wheel)*3, 0), max(len(v.filtered)-24, 0))
	}
	if rl.IsKeyPressed(rl.KeyDown) && len(v.filtered) > 0 {
		v.selected = min(v.selected+1, len(v.filtered)-1)
		if v.selected >= v.scroll+24 {
			v.scroll = v.selected - 23
		}
	}
	if rl.IsKeyPressed(rl.KeyUp) && len(v.filtered) > 0 {
		v.selected = max(v.selected-1, 0)
		if v.selected < v.scroll {
			v.scroll = v.selected
		}
	}
	mouse := simpleui.MousePosition()
	for row := 0; row < 24 && v.scroll+row < len(v.filtered); row++ {
		index := v.scroll + row
		e := v.filtered[index]
		y := 123 + float32(row)*24
		if index == v.selected {
			rl.DrawRectangle(20, int32(y-3), 1460, 23, rl.Color{R: 25, G: 83, B: 116, A: 230})
		} else if row%2 == 0 {
			rl.DrawRectangle(20, int32(y-3), 1460, 23, colors.panel)
		}
		bounds := rl.Rectangle{X: 20, Y: y - 3, Width: 1460, Height: 23}
		if rl.IsMouseButtonReleased(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(mouse, bounds) {
			v.selected = index
		}
		simpleui.DrawTextStyled(e.Received.Format("2006-01-02 15:04:05"), 28, y, 13, simpleui.FontMono, colors.text)
		simpleui.DrawTextStyled(fmt.Sprintf("%.6f", float64(e.FrequencyHz)/1e6), 210, y, 13, simpleui.FontMono, colors.text)
		simpleui.DrawTextStyled(fmt.Sprintf("%07d", e.Address), 320, y, 13, simpleui.FontMono, colors.text)
		simpleui.DrawText(strconv.Itoa(e.Function), 465, y, 13, colors.text)
		simpleui.DrawText(strconv.Itoa(e.Baud), 525, y, 13, colors.text)
		simpleui.DrawText(e.Kind, 600, y, 13, colors.text)
		simpleui.DrawText(short(e.Message, 95), 680, y, 13, colors.text)
	}
	drawPanel(20, 710, 1460, 90)
	simpleui.DrawTextStyled("MENSAJE COMPLETO", 32, 718, 13, simpleui.FontSemiBold, colors.cyan)
	if v.selected >= 0 && v.selected < len(v.filtered) {
		drawWrapped(v.filtered[v.selected].Message, 32, 744, 1430, 14, colors.text)
	} else {
		simpleui.DrawText("Seleccione una fila para ver y copiar el mensaje completo.", 32, 746, 13, colors.muted)
	}
}

func (v *pocsagViewer) copySelected() {
	if v.selected < 0 || v.selected >= len(v.filtered) {
		return
	}
	e := v.filtered[v.selected]
	rl.SetClipboardText(fmt.Sprintf("%s · %.6f MHz · CAPCODE %07d · F%d · %d baud · %s", e.Received.Format("2006-01-02 15:04:05"), float64(e.FrequencyHz)/1e6, e.Address, e.Function, e.Baud, e.Message))
	v.feedback, v.feedbackUntil = "MENSAJE COPIADO", time.Now().Add(2*time.Second)
}

func (v *pocsagViewer) exportCSV() {
	path, err := ExportPOCSAGCSV(v.filtered)
	if err != nil {
		v.feedback = "ERROR AL EXPORTAR"
	} else {
		v.feedback = "EXPORTADO: " + filepath.Base(path)
	}
	v.feedbackUntil = time.Now().Add(3 * time.Second)
}
