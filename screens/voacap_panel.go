package screens

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"go-zero/internal/i18n"
	"go-zero/internal/resources"
	"go-zero/internal/voacap"
	"go-zero/simpleui"
)

const voacapToolID = "VOACAP_DX"

type VOACAPPanel struct {
	screen               *MainScreen
	controls             []simpleui.Element
	qth, latitude        *simpleui.TextField
	longitude            *simpleui.TextField
	band, mode, power    *simpleui.Dropdown
	antenna, noise, hour *simpleui.Dropdown
	ssn                  *simpleui.Dropdown
	autoSolar            *simpleui.Switch
	settings             voacap.Settings
	settingsPath         string
	snapshotPath         string
	solarCachePath       string
	solar                voacap.SolarData
	solarSource          string
	solarResults         chan solarUpdateResult
	solarUpdating        bool
	nextSolarUpdate      time.Time
	viewer               *exec.Cmd
	viewerDone           chan struct{}
	feedback             string
	prediction           voacap.Prediction
	calculateAt          time.Time
	calculationCancel    context.CancelFunc
	calculationID        uint64
	calculationResults   chan voacapCalculationResult
}

type voacapCalculationResult struct {
	id         uint64
	prediction voacap.Prediction
}

type solarUpdateResult struct {
	data voacap.SolarData
	err  error
}

func NewVOACAPPanel(screen *MainScreen) *VOACAPPanel {
	p := &VOACAPPanel{
		screen: screen, calculationResults: make(chan voacapCalculationResult, 1),
		settingsPath:   resources.WritablePath("config", "voacap.json"),
		snapshotPath:   resources.WritablePath("cache", "voacap-map.json"),
		solarCachePath: resources.WritablePath("cache", "voacap-solar.json"),
		solarResults:   make(chan solarUpdateResult, 1),
	}
	p.settings = voacap.Load(p.settingsPath)
	if cached, err := voacap.LoadSolarData(p.solarCachePath); err == nil {
		p.solar = cached
		p.solarSource = "CACHÉ NOAA"
		if p.settings.AutoSolar {
			p.settings.SSN = solarSSN(cached.SSN)
		}
	}
	// Predictions are time-sensitive: every new application session starts at
	// the current UTC hour instead of silently reusing yesterday's saved hour.
	p.settings.UTCHour = time.Now().UTC().Hour()
	p.qth = simpleui.NewTextField("voacapQTH", 390, toolY+64, 220, 42, "QTH / LOCATOR", 14)
	p.latitude = simpleui.NewTextField("voacapLatitude", 622, toolY+64, 160, 42, "LATITUD", 14)
	p.longitude = simpleui.NewTextField("voacapLongitude", 794, toolY+64, 160, 42, "LONGITUD", 14)
	p.qth.SetText(p.settings.QTH)
	p.latitude.SetText(strconv.FormatFloat(p.settings.Latitude, 'f', 5, 64))
	p.longitude.SetText(strconv.FormatFloat(p.settings.Longitude, 'f', 5, 64))
	p.qth.SetMaxLength(32)
	p.latitude.SetMaxLength(12)
	p.longitude.SetMaxLength(13)
	p.qth.OnChange(func(value string) { p.settings.QTH = value; p.changed() })
	p.latitude.OnChange(func(string) { p.readCoordinates() })
	p.longitude.OnChange(func(string) { p.readCoordinates() })
	p.controls = append(p.controls, p.qth, p.latitude, p.longitude)

	selectedBand := voacap.AmateurBands[0]
	for _, band := range voacap.AmateurBands {
		if p.settings.Bands[band] {
			selectedBand = band
			break
		}
	}
	for _, band := range voacap.AmateurBands {
		p.settings.Bands[band] = band == selectedBand
	}
	p.band = p.dropdown("voacapBand", 966, toolY+64, 135, 42, "BANDA", voacap.AmateurBands, selectedBand)
	p.mode = p.dropdown("voacapMode", 1113, toolY+64, 135, 42, "MODO", []string{"SSB", "CW", "DIGITAL"}, p.settings.Mode)
	p.power = p.dropdown("voacapPower", 1260, toolY+64, 145, 42, "POTENCIA · W", []string{"5 W", "10 W", "50 W", "100 W", "500 W", "1000 W"}, fmt.Sprintf("%d W", p.settings.PowerW))
	p.antenna = p.dropdown("voacapAntenna", 390, toolY+151, 220, 42, "ANTENA", []string{"DIPOLO", "VERTICAL", "YAGI 3 EL.", "YAGI 5 EL."}, p.settings.Antenna)
	p.noise = p.dropdown("voacapNoise", 622, toolY+151, 160, 42, "RUIDO", []string{"RURAL", "RESIDENCIAL", "URBANO"}, p.settings.Noise)
	hours := make([]string, 24)
	for index := range hours {
		hours[index] = fmt.Sprintf("%02d:00 UTC", index)
	}
	p.hour = p.dropdown("voacapHour", 1417, toolY+64, 145, 42, "HORA UTC", hours, hours[p.settings.UTCHour])
	p.band.OnChange(func(_ int, value string) {
		for _, band := range voacap.AmateurBands {
			p.settings.Bands[band] = band == value
		}
		p.changed()
	})
	p.mode.OnChange(func(_ int, value string) { p.settings.Mode = value; p.changed() })
	p.power.OnChange(func(_ int, value string) { fmt.Sscanf(value, "%d W", &p.settings.PowerW); p.changed() })
	p.antenna.OnChange(func(_ int, value string) { p.settings.Antenna = value; p.changed() })
	p.noise.OnChange(func(_ int, value string) { p.settings.Noise = value; p.changed() })
	p.hour.OnChange(func(index int, _ string) { p.settings.UTCHour = index; p.changed() })
	ssnValues := []string{"0", "25", "50", "75", "100", "125", "150", "175", "200", "250", "300"}
	p.ssn = simpleui.NewDropdown("voacapSSN", 794, toolY+151, 130, 42, "SSN", ssnValues, 14)
	closestSSN := 0
	for index, value := range ssnValues {
		parsed, _ := strconv.Atoi(value)
		if absInt(parsed-p.settings.SSN) < absInt(closestSSN-p.settings.SSN) {
			closestSSN = parsed
			p.ssn.SetSelected(index)
		}
	}
	p.ssn.OnChange(func(_ int, value string) { p.settings.SSN, _ = strconv.Atoi(value); p.changed() })
	p.controls = append(p.controls, p.ssn)
	p.autoSolar = simpleui.NewSwitch("voacapAutoSolar", 1110, toolY+151, 220, 42, "SOLAR NOAA AUTO", p.settings.AutoSolar, 14)
	p.autoSolar.OnChange(func(active bool) {
		p.settings.AutoSolar = active
		p.ssn.SetEnabled(!active)
		if active {
			p.nextSolarUpdate = time.Now()
			if p.solar.SSN > 0 {
				p.applySolarData(p.solar, p.solarSource)
			}
		} else {
			p.solarSource = "MANUAL"
		}
		p.changed()
	})
	p.ssn.SetEnabled(!p.settings.AutoSolar)
	p.controls = append(p.controls, p.autoSolar)
	if p.settings.AutoSolar {
		p.nextSolarUpdate = time.Now()
	}

	now := simpleui.NewButton("voacapNow", 936, toolY+151, 160, 42, "AHORA UTC", 14)
	now.OnClick(func() {
		p.settings.UTCHour = time.Now().UTC().Hour()
		p.hour.SetSelected(p.settings.UTCHour)
		p.changed()
	})
	openMap := simpleui.NewButton("voacapOpenMap", 1350, toolY+145, 212, 54, "ABRIR MAPA DX", 15)
	openMap.SetColors(colors.blue, colors.cyan, colors.text)
	openMap.OnClick(p.openMap)
	p.controls = append(p.controls, now, openMap)
	p.SetVisible(false)
	p.changed()
	return p
}

func (p *VOACAPPanel) dropdown(id string, x, y, width, height float32, placeholder string, items []string, selected string) *simpleui.Dropdown {
	control := simpleui.NewDropdown(id, x, y, width, height, placeholder, items, 14)
	for index, item := range items {
		if item == selected {
			control.SetSelected(index)
			break
		}
	}
	p.controls = append(p.controls, control)
	return control
}

func (p *VOACAPPanel) readCoordinates() {
	lat, latErr := strconv.ParseFloat(p.latitude.Text(), 64)
	lon, lonErr := strconv.ParseFloat(p.longitude.Text(), 64)
	if latErr != nil || lonErr != nil || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		p.feedback = "COORDENADAS NO VÁLIDAS"
		return
	}
	p.settings.Latitude, p.settings.Longitude = lat, lon
	p.feedback = ""
	p.changed()
}

func (p *VOACAPPanel) changed() {
	p.calculationID++
	if p.calculationCancel != nil {
		p.calculationCancel()
		p.calculationCancel = nil
	}
	// Never present a previous band's coverage as if it belonged to the new
	// selection while VOACAP is recalculating the map.
	p.prediction = voacap.Prediction{}
	p.settings.Normalize()
	data, _ := json.MarshalIndent(p.settings, "", "  ")
	_ = os.MkdirAll(filepath.Dir(p.settingsPath), 0o755)
	_ = os.WriteFile(p.settingsPath, append(data, '\n'), 0o644)
	p.writeSnapshot()
	p.calculateAt = time.Now().Add(450 * time.Millisecond)
}

func (p *VOACAPPanel) writeSnapshot() {
	engine := "CALCULANDO"
	if p.prediction.Error != "" {
		engine = p.prediction.Error
	} else if len(p.prediction.Cells) > 0 {
		engine = p.prediction.Engine
	}
	snapshot := voacap.Snapshot{Settings: p.settings, Theme: p.screen.themeName, Updated: time.Now().UTC(), Engine: engine, Prediction: p.prediction, Solar: p.solar}
	data, err := json.Marshal(snapshot)
	if err == nil {
		_ = replaceLiveSnapshot(p.snapshotPath, data)
	}
}

func (p *VOACAPPanel) openMap() {
	p.writeSnapshot()
	if p.viewer != nil && p.viewer.Process != nil {
		focusRTL433Viewer(p.viewer.Process.Pid)
		return
	}
	executable, err := os.Executable()
	if err != nil {
		p.feedback = "NO SE PUDO ABRIR EL MAPA"
		return
	}
	command := exec.Command(executable, "--voacap-map", p.snapshotPath)
	command.SysProcAttr = rtl433ViewerProcessAttributes()
	if err = command.Start(); err != nil {
		p.feedback = "NO SE PUDO ABRIR EL MAPA"
		return
	}
	p.viewer = command
	p.viewerDone = make(chan struct{})
	done := p.viewerDone
	go func() { _ = command.Wait(); close(done) }()
	p.feedback = "MAPA DX ABIERTO"
}

func (p *VOACAPPanel) Tick() {
	select {
	case result := <-p.solarResults:
		p.solarUpdating = false
		p.nextSolarUpdate = time.Now().Add(30 * time.Minute)
		if result.err != nil {
			if p.solar.SSN > 0 {
				p.solarSource = "CACHÉ NOAA · SIN CONEXIÓN"
			} else {
				p.solarSource = "MANUAL · NOAA NO DISPONIBLE"
			}
		} else {
			p.solar = result.data
			p.solarSource = "NOAA EN LÍNEA"
			_ = voacap.SaveSolarData(p.solarCachePath, result.data)
			if p.settings.AutoSolar {
				if p.applySolarData(result.data, p.solarSource) {
					p.changed()
				}
			}
		}
	default:
	}
	if p.settings.AutoSolar && !p.solarUpdating && !p.nextSolarUpdate.IsZero() && time.Now().After(p.nextSolarUpdate) {
		p.solarUpdating = true
		p.nextSolarUpdate = time.Time{}
		go func() {
			data, err := voacap.FetchSolarData(context.Background())
			p.solarResults <- solarUpdateResult{data: data, err: err}
		}()
	}
	select {
	case result := <-p.calculationResults:
		if result.id == p.calculationID {
			p.calculationCancel = nil
			p.prediction = result.prediction
			p.writeSnapshot()
		}
	default:
	}
	if !p.calculateAt.IsZero() && time.Now().After(p.calculateAt) && p.calculationCancel == nil {
		p.calculateAt = time.Time{}
		p.calculationID++
		id, settings := p.calculationID, p.settings
		ctx, cancel := context.WithCancel(context.Background())
		p.calculationCancel = cancel
		go func() {
			prediction := voacap.Calculate(ctx, resources.Path("tools", "voacap", "runtime"), settings)
			p.calculationResults <- voacapCalculationResult{id: id, prediction: prediction}
		}()
	}
	if p.viewerDone != nil {
		select {
		case <-p.viewerDone:
			p.viewer, p.viewerDone = nil, nil
		default:
		}
	}
}

func solarSSN(value float64) int {
	return min(max(int(value/5+.5)*5, 0), 300)
}

func (p *VOACAPPanel) applySolarData(data voacap.SolarData, source string) bool {
	ssn := solarSSN(data.SSN)
	p.solarSource = source
	if p.settings.SSN == ssn {
		return false
	}
	p.settings.SSN = ssn
	for index, value := range p.ssn.Items() {
		if parsed, _ := strconv.Atoi(value); parsed == ssn {
			p.ssn.SetSelected(index)
			break
		}
	}
	return true
}

func (p *VOACAPPanel) Close() {
	if p.calculationCancel != nil {
		p.calculationCancel()
	}
	if p.viewer != nil && p.viewer.Process != nil {
		_ = p.viewer.Process.Kill()
	}
}

func (p *VOACAPPanel) SetVisible(visible bool) {
	for _, control := range p.controls {
		control.SetVisible(visible)
	}
}

// Enter synchronizes the propagation model with the current hour whenever the
// user opens the tool. The hour remains selectable afterwards for planning.
func (p *VOACAPPanel) Enter() {
	hour := time.Now().UTC().Hour()
	if p.settings.UTCHour == hour {
		return
	}
	p.settings.UTCHour = hour
	p.hour.SetSelected(hour)
	p.changed()
}

func (p *VOACAPPanel) OverlayOpen() bool {
	return p.band.OverlayOpen() || p.mode.OverlayOpen() || p.power.OverlayOpen() || p.antenna.OverlayOpen() || p.noise.OverlayOpen() || p.hour.OverlayOpen() || p.ssn.OverlayOpen()
}

func (p *VOACAPPanel) DrawPanel() {
	x, y, width := float32(360), toolY, float32(1232)
	drawPanel(x, y, width, toolH)
	simpleui.DrawTextStyled("PROPAGACIÓN DX · VOACAP", x+18, y+12, 20, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText("CONFIGURACIÓN LOCAL · HF 3–30 MHz", x+340, y+18, 13, colors.muted)
	labels := []struct {
		text string
		x    float32
		y    float32
	}{{"ESTACIÓN / QTH", 390, y + 48}, {"LATITUD", 622, y + 48}, {"LONGITUD", 794, y + 48}, {"BANDA", 966, y + 48}, {"MODO", 1113, y + 48}, {"POTENCIA · W", 1260, y + 48}, {"HORA UTC", 1417, y + 48}, {"ANTENA", 390, y + 135}, {"RUIDO", 622, y + 135}, {"CICLO SOLAR · SSN", 794, y + 135}, {"ACTUALIZACIÓN SOLAR", 1110, y + 135}}
	for _, label := range labels {
		simpleui.DrawText(label.text, label.x, label.y, 11, colors.muted)
	}
	status := "VOACAP LOCAL · PREPARANDO CÁLCULO"
	statusColor := colors.orange
	if p.calculationCancel != nil {
		status = "VOACAP LOCAL · CALCULANDO COBERTURA…"
	}
	if p.prediction.Error != "" {
		status = p.prediction.Error
	}
	if len(p.prediction.Cells) > 0 {
		status = fmt.Sprintf("VOACAP 08.0121W · %d PUNTOS · SSN %d", len(p.prediction.Cells), p.settings.SSN)
		statusColor = colors.green
	}
	simpleui.DrawText(status, 900, y+246, 12, statusColor)
	solarStatus := i18n.Source(p.solarSource)
	if solarStatus == "" {
		solarStatus = i18n.Source("MANUAL")
	}
	if p.solarUpdating {
		solarStatus = i18n.Source("CONECTANDO CON NOAA…")
	}
	if p.solar.SSN > 0 {
		solarStatus += fmt.Sprintf(" · SSN %.0f · SFI %.0f · KP %.1f · AP %d · R%d/S%d/G%d · X %s", p.solar.SSN, p.solar.SFI, p.solar.Kp, p.solar.Ap, p.solar.RScale, p.solar.SScale, p.solar.GScale, p.solar.XRayClass)
		if !p.solar.DRAPTime.IsZero() {
			solarStatus += " · D-RAP " + p.solar.DRAPTime.UTC().Format("15:04Z")
		}
	}
	simpleui.DrawText(solarStatus, 390, y+218, 12, colors.muted)
	if p.feedback != "" {
		simpleui.DrawText(p.feedback, 1160, y+16, 10, colors.green)
	}
}
