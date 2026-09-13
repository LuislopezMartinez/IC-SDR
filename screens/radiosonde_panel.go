package screens

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"go-zero/internal/radiosonde"
	"go-zero/internal/resources"
	"go-zero/simpleui"
)

type RadiosondePanel struct {
	screen             *MainScreen
	controls           []simpleui.Element
	models             []*simpleui.Button
	start              *simpleui.Button
	tuneButton         *simpleui.Button
	family             string
	targetHz, centerHz int64
	quickTuneHz        int64
	enabled            bool
	pendingAt          time.Time
	feedback           string
}

const radiosondeQuickTuneHz int64 = 403_000_000

func NewRadiosondePanel(screen *MainScreen) *RadiosondePanel {
	p := &RadiosondePanel{screen: screen, family: screen.radiosondeFamily, targetHz: screen.radiosondeFrequencyHz}
	if !radiosonde.ValidFamily(p.family) {
		p.family = "AUTO"
	}
	if !validRadiosondeFrequency(p.targetHz) {
		p.targetHz = radiosondeQuickTuneHz
	}
	p.quickTuneHz = p.targetHz
	button := func(id, label string, x, w float32, action func()) *simpleui.Button {
		b := simpleui.NewButton(id, x, toolY+28, w, 34, label, uiControlFontSize)
		b.SetColors(colors.panelAlt, colors.border, colors.text)
		b.OnClick(action)
		p.controls = append(p.controls, b)
		return b
	}
	for i, family := range radiosonde.Modes {
		f := family
		b := button("sondeModel"+strconv.Itoa(i), f, 40+float32(i)*110, 100, func() { p.family = f; p.apply(); p.save() })
		p.models = append(p.models, b)
	}
	p.start = button("sondeStart", "INICIAR", 490, 135, func() { p.enabled = !p.enabled; p.apply() })
	jsonButton := button("sondeJSON", "JSON", 640, 105, func() { p.export(false) })
	csvButton := button("sondeCSV", "CSV", 755, 105, func() { p.export(true) })
	jsonButton.SetColors(actionExportFill, colors.green, colors.text)
	csvButton.SetColors(actionExportFill, colors.green, colors.text)
	clearButton := button("sondeClear", "LIMPIAR", 870, 120, func() {
		if screen.receiver != nil {
			screen.receiver.ClearRadiosondeEvents()
		}
		p.feedback = "Historial limpiado"
	})
	clearButton.SetColors(actionClearFill, colors.red, colors.text)
	p.tuneButton = button("sondeTune", fmt.Sprintf("SINTONIZAR %.3f MHz", float64(p.quickTuneHz)/1e6), 1010, 300, p.tuneRecommended)
	p.tuneButton.SetColors(colors.blue, colors.border, colors.text)
	p.SetVisible(false)
	p.style()
	return p
}

func (p *RadiosondePanel) tuneRecommended() {
	s := p.screen
	s.draggingSpectrum = false
	s.setFrequencyDigitExponent(-1)
	s.frequencyHz = p.quickTuneHz
	s.centerFrequencyHz = p.quickTuneHz
	s.centerMode = true
	s.updateBandForFrequency(p.quickTuneHz)
	if s.vfoModeSwitch != nil {
		s.vfoModeSwitch.SetActive(false)
	}
	if s.receiver != nil {
		s.receiver.SetCenterFrequency(p.quickTuneHz)
		s.receiver.SetDemodulator(s.receiverDemodMode(), p.quickTuneHz, s.demodBandwidthHz)
	}
	if s.waterfall != nil {
		s.waterfall.Reset()
	}
	p.targetHz = p.quickTuneHz
	p.centerHz = p.quickTuneHz
	p.pendingAt = time.Time{}
	p.apply()
	p.save()
	p.feedback = fmt.Sprintf("Sintonizado en %.3f MHz · busca la portadora de una sonda cercana", float64(p.quickTuneHz)/1e6)
}
func (p *RadiosondePanel) SetVisible(v bool) {
	for _, c := range p.controls {
		c.SetVisible(v)
	}
}
func (p *RadiosondePanel) Enter() {
	// Model selection and panel entry do not own the receiver tuning. The dial
	// remains exactly where the user left it.
	if p.screen.mode != nil && p.screen.mode.SelectedText() != "NFM" {
		for i, mode := range p.screen.mode.Items() {
			if mode == "NFM" {
				p.screen.mode.SetSelected(i)
				p.screen.savedMode = mode
				if p.screen.filterSelector != nil {
					p.screen.selectFilter(p.screen.filterSelector.Current(mode))
				} else if p.screen.receiver != nil {
					p.screen.receiver.SetDemodulator(mode, p.screen.frequencyHz, p.screen.demodBandwidthHz)
				}
				break
			}
		}
	}
	p.targetHz = p.screen.frequencyHz
	p.centerHz = p.screen.centerFrequencyHz
	p.pendingAt = time.Time{}
	p.save()
	p.style()
}

func validRadiosondeFrequency(hz int64) bool { return hz >= 400_000_000 && hz <= 406_000_000 }
func (p *RadiosondePanel) Leave() {
	p.enabled = false
	p.apply()
	p.save()
}
func (p *RadiosondePanel) Close() { p.enabled = false; p.apply() }
func (p *RadiosondePanel) save() {
	p.screen.radiosondeFamily = p.family
	p.screen.radiosondeFrequencyHz = p.quickTuneHz
	p.screen.markSettingsDirty()
}
func (p *RadiosondePanel) apply() {
	if p.screen.receiver != nil {
		p.screen.receiver.ConfigureRadiosonde(p.enabled, p.family, p.targetHz)
		if p.enabled && !p.screen.receiver.RadiosondeStatus().Running {
			p.enabled = false
		}
	}
	p.style()
}
func (p *RadiosondePanel) style() {
	for i, b := range p.models {
		c := colors.panelAlt
		if radiosonde.Modes[i] == p.family {
			c = colors.blue
		}
		b.SetColors(c, colors.border, colors.text)
	}
	if p.enabled {
		p.start.SetLabel("DETENER")
		p.start.SetColors(actionStopFill, colors.red, colors.text)
	} else {
		p.start.SetLabel("INICIAR")
		p.start.SetColors(actionStartFill, colors.green, colors.text)
	}
}
func (p *RadiosondePanel) Tick() {
	if p.screen.activeTool != "RADIOSONDE" {
		return
	}
	if p.targetHz != p.screen.frequencyHz || p.centerHz != p.screen.centerFrequencyHz {
		p.targetHz = p.screen.frequencyHz
		p.centerHz = p.screen.centerFrequencyHz
		if p.screen.receiver != nil {
			p.screen.receiver.ConfigureRadiosonde(false, p.family, p.targetHz)
		}
		p.pendingAt = time.Now().Add(300 * time.Millisecond)
		p.save()
	}
	if !p.pendingAt.IsZero() && time.Now().After(p.pendingAt) {
		p.pendingAt = time.Time{}
		p.apply()
	}
	if p.enabled && p.pendingAt.IsZero() && p.screen.receiver != nil && !p.screen.receiver.RadiosondeStatus().Running {
		p.enabled = false
		p.style()
	}
}
func sondeNumber(v *float64, format string) string {
	if v == nil {
		return "--"
	}
	return fmt.Sprintf(format, *v)
}
func (p *RadiosondePanel) DrawPanel() {
	status := radiosonde.Status{State: "SIN RECEPTOR"}
	var events []radiosonde.Event
	if p.screen.receiver != nil {
		status = p.screen.receiver.RadiosondeStatus()
		events = p.screen.receiver.RadiosondeEvents()
	}
	p.drawTelemetry(status, events)
}

func (p *RadiosondePanel) drawTelemetry(status radiosonde.Status, events []radiosonde.Event) {
	simpleui.DrawText(fmt.Sprintf("RADIOSONDAS · %.6f MHz · %s · %d tramas · %d bloques perdidos", float64(p.targetHz)/1e6, status.State, len(events), status.Dropped), 40, toolY+7, 12, colors.cyan)
	if status.Error != "" {
		simpleui.DrawText(sondeClip(status.Error, 72), 1020, toolY+36, 12, colors.red)
	} else {
		simpleui.DrawText("Sintoniza con el dial / espectro", 1020, toolY+36, 12, colors.muted)
	}
	columns := []struct {
		x     float32
		label string
	}{{40, "SONDA"}, {240, "RECIBIDA UTC"}, {370, "LATITUD"}, {500, "LONGITUD"}, {640, "ALT m"}, {750, "Vh m/s"}, {860, "Vv m/s"}, {975, "T °C"}, {1080, "HR %"}, {1180, "P hPa"}, {1300, "REF. ALT"}}
	for _, c := range columns {
		simpleui.DrawText(c.label, c.x, toolY+72, 12, colors.muted)
	}
	seen := map[string]bool{}
	row := 0
	for i := len(events) - 1; i >= 0 && row < 3; i-- {
		e := events[i]
		key := e.Type + ":" + e.ID
		if seen[key] {
			continue
		}
		seen[key] = true
		values := []string{sondeClip(e.ID, 18), e.Received.Format("15:04:05"), sondeNumber(e.Lat, "%.5f"), sondeNumber(e.Lon, "%.5f"), sondeNumber(e.Alt, "%.0f"), sondeNumber(e.Speed, "%.1f"), sondeNumber(e.Climb, "%.1f"), sondeNumber(e.Temperature, "%.1f"), sondeNumber(e.Humidity, "%.1f"), sondeNumber(e.Pressure, "%.1f"), e.PositionReference}
		for i, v := range values {
			simpleui.DrawText(v, columns[i].x, toolY+94+float32(row)*22, 13, colors.text)
		}
		row++
	}
	if len(events) == 0 {
		simpleui.DrawText("Selecciona familia, ajusta frecuencia y pulsa INICIAR. Una frecuencia activa; sin barrido automático.", 40, toolY+99, 13, colors.muted)
	}
	footer := p.feedback
	if footer == "" {
		footer = "Últimas sondas · -- = dato no disponible · Historial: últimas 2000 tramas · Altitud según referencia original (JSON/CSV)"
	}
	simpleui.DrawText(sondeClip(footer, 180), 40, toolY+172, 12, colors.muted)
}
func sondeClip(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n-3]) + "..."
	}
	return s
}
func (p *RadiosondePanel) export(asCSV bool) {
	if p.screen.receiver == nil {
		return
	}
	events := p.screen.receiver.RadiosondeEvents()
	if len(events) == 0 {
		p.feedback = "No hay tramas para exportar"
		return
	}
	dir := resources.WritablePath("exports", "radiosonde")
	if err := os.MkdirAll(dir, 0755); err != nil {
		p.feedback = err.Error()
		return
	}
	extension := ".json"
	if asCSV {
		extension = ".csv"
	}
	path := filepath.Join(dir, "sondes-"+time.Now().Format("20060102-150405.000000000")+extension)
	f, err := os.Create(path)
	if err != nil {
		p.feedback = err.Error()
		return
	}
	if asCSV {
		w := csv.NewWriter(f)
		_ = w.Write([]string{"received_utc", "type", "id", "frame", "datetime", "ref_datetime", "frequency_hz", "lat", "lon", "alt_m", "ref_position", "vel_h_ms", "vel_v_ms", "temp_c", "humidity_pct", "pressure_hpa"})
		for _, e := range events {
			_ = w.Write([]string{e.Received.Format(time.RFC3339Nano), e.Type, e.ID, strconv.FormatInt(e.Frame, 10), e.Datetime, e.DatetimeReference, strconv.FormatInt(e.FrequencyHz, 10), sondeCSVNumber(e.Lat), sondeCSVNumber(e.Lon), sondeCSVNumber(e.Alt), e.PositionReference, sondeCSVNumber(e.Speed), sondeCSVNumber(e.Climb), sondeCSVNumber(e.Temperature), sondeCSVNumber(e.Humidity), sondeCSVNumber(e.Pressure)})
		}
		w.Flush()
		err = w.Error()
	} else {
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		err = enc.Encode(events)
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		p.feedback = err.Error()
		return
	}
	p.feedback = "Guardado: " + path
}
func sondeCSVNumber(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}
