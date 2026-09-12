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
	family             string
	targetHz, centerHz int64
	enabled            bool
	pendingAt          time.Time
	feedback           string
}

func NewRadiosondePanel(screen *MainScreen) *RadiosondePanel {
	p := &RadiosondePanel{screen: screen, family: screen.radiosondeFamily, targetHz: screen.radiosondeFrequencyHz}
	if !radiosonde.ValidFamily(p.family) {
		p.family = "AUTO"
	}
	if !validRadiosondeFrequency(p.targetHz) {
		p.targetHz = 403_000_000
	}
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
	p.start = button("sondeStart", "START", 490, 135, func() { p.enabled = !p.enabled; p.apply() })
	jsonButton := button("sondeJSON", "JSON", 640, 105, func() { p.export(false) })
	csvButton := button("sondeCSV", "CSV", 755, 105, func() { p.export(true) })
	jsonButton.SetColors(actionExportFill, colors.green, colors.text)
	csvButton.SetColors(actionExportFill, colors.green, colors.text)
	clearButton := button("sondeClear", T("CLEAR"), 870, 120, func() {
		if screen.receiver != nil {
			screen.receiver.ClearRadiosondeEvents()
		}
		p.feedback = "History cleared"
	})
	clearButton.SetColors(actionClearFill, colors.red, colors.text)
	p.SetVisible(false)
	p.style()
	return p
}
func (p *RadiosondePanel) SetVisible(v bool) {
	for _, c := range p.controls {
		c.SetVisible(v)
	}
}
func (p *RadiosondePanel) Enter() {
	// Model selection and panel entry do not own the receiver tuning. The dial
	// remains exactly where the user left it.
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
	p.screen.radiosondeFrequencyHz = p.targetHz
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
		p.start.SetLabel(T("STOP"))
		p.start.SetColors(actionStopFill, colors.red, colors.text)
	} else {
		p.start.SetLabel(T("START"))
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
	status := radiosonde.Status{State: "NO RECEIVER"}
	var events []radiosonde.Event
	if p.screen.receiver != nil {
		status = p.screen.receiver.RadiosondeStatus()
		events = p.screen.receiver.RadiosondeEvents()
	}
	p.drawTelemetry(status, events)
}

func (p *RadiosondePanel) drawTelemetry(status radiosonde.Status, events []radiosonde.Event) {
	simpleui.DrawText(fmt.Sprintf("RADIOSONDES · %.6f MHz · %s · %d frames · %d dropped blocks", float64(p.targetHz)/1e6, status.State, len(events), status.Dropped), 40, toolY+7, 12, colors.cyan)
	if status.Error != "" {
		simpleui.DrawText(sondeClip(status.Error, 72), 1020, toolY+36, 12, colors.red)
	} else {
		simpleui.DrawText("Tune with the dial / spectrum", 1020, toolY+36, 12, colors.muted)
	}
	columns := []struct {
		x     float32
		label string
	}{{40, "SONDE"}, {240, "RECEIVED UTC"}, {370, "LATITUDE"}, {500, "LONGITUDE"}, {640, "ALT m"}, {750, "Vh m/s"}, {860, "Vv m/s"}, {975, "T °C"}, {1080, "RH %"}, {1180, "P hPa"}, {1300, "REF. ALT"}}
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
		simpleui.DrawText("Select a family, set the frequency and press START. One active frequency; no automatic sweep.", 40, toolY+99, 13, colors.muted)
	}
	footer := p.feedback
	if footer == "" {
		footer = "Latest sondes · -- = data not available · History: last 2000 frames · Altitude per original reference (JSON/CSV)"
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
		p.feedback = "No frames to export"
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
	p.feedback = "Saved: " + path
}
func sondeCSVNumber(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}
