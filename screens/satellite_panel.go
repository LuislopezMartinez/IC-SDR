package screens

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/internal/resources"
	"go-zero/internal/satellite"
	"go-zero/simpleui"
)

type SatellitePanel struct {
	screen                        *MainScreen
	controls                      []simpleui.Element
	groupSelect                   *simpleui.Dropdown
	satelliteSelect, signalSelect *simpleui.Dropdown
	searchField                   *simpleui.TextField
	searchMatches                 []satellite.Satellite
	currentGroup                  string
	stationName                   *simpleui.TextField
	stationLat, stationLon        *simpleui.TextField
	stationAlt                    *simpleui.TextField
	tracker                       *satellite.Tracker
	snapshotPath                  string
	stationPath                   string
	viewer                        *exec.Cmd
	viewerDone                    chan struct{}
	feedback                      string
	updating                      bool
	updateDone                    chan error
	lastSnapshot                  string
	nextSnapshot                  time.Time
	lastMapSelection              int
}

func NewSatellitePanel(screen *MainScreen) *SatellitePanel {
	cache := resources.WritablePath("cache", "satellites-celestrak.json")
	p := &SatellitePanel{screen: screen, tracker: satellite.NewTracker(cache), snapshotPath: resources.WritablePath("cache", "satellites-live.json"), stationPath: resources.WritablePath("config", "satellite-station.json")}
	p.loadStation()
	p.groupSelect = simpleui.NewDropdown("satelliteGroup", 470, toolY+30, 250, 34, "GROUP", nil, uiControlFontSize)
	p.satelliteSelect = simpleui.NewDropdown("satelliteObject", 730, toolY+30, 260, 34, "SATELLITE", nil, uiControlFontSize)
	p.signalSelect = simpleui.NewDropdown("satelliteSignal", 1000, toolY+30, 280, 34, "SIGNAL", nil, uiControlFontSize)
	p.groupSelect.SetMaxVisibleItems(8)
	p.satelliteSelect.SetMaxVisibleItems(8)
	p.signalSelect.SetMaxVisibleItems(6)
	p.groupSelect.OnChange(func(_ int, group string) {
		p.currentGroup = group
		p.populateSatellites(0)
	})
	p.satelliteSelect.OnChange(func(index int, _ string) {
		sats := p.groupSatellites(p.currentGroup)
		if index >= 0 && index < len(sats) {
			p.selectSatellite(sats[index])
		}
	})
	p.signalSelect.OnChange(func(int, string) {})
	p.controls = append(p.controls, p.groupSelect, p.satelliteSelect, p.signalSelect)
	button := func(id, label string, x, w float32, action func()) *simpleui.Button {
		b := simpleui.NewButton(id, x, toolY+30, w, 34, label, uiControlFontSize)
		b.SetColors(colors.panelAlt, colors.border, colors.text)
		b.OnClick(action)
		p.controls = append(p.controls, b)
		return b
	}
	tune := button("satelliteTune", "TUNE", 1290, 135, p.tune)
	tune.SetColors(actionStartFill, colors.green, colors.text)
	update := button("satelliteUpdate", "UPDATE", 1435, 140, p.update)
	update.SetColors(actionExportFill, colors.blue, colors.text)
	mapButton := button("satelliteMap", "▦  MAPA", 1290, 285, p.openMap)
	mapButton.SetBounds(rl.Rectangle{X: 1290, Y: toolY + 78, Width: 285, Height: 30})
	mapButton.SetColors(colors.blue, colors.border, colors.text)
	station := p.tracker.Station()
	p.searchField = simpleui.NewTextField("satelliteSearch", 40, toolY+30, 390, 34, "SEARCH NAME, NORAD OR GROUP", 13)
	p.searchField.SetMaxLength(64)
	p.searchField.OnChange(p.updateSearch)
	p.stationName = simpleui.NewTextField("satelliteStationName", 470, toolY+78, 140, 30, "STATION", 12)
	p.stationLat = simpleui.NewTextField("satelliteStationLat", 620, toolY+78, 110, 30, "LATITUDE", 12)
	p.stationLon = simpleui.NewTextField("satelliteStationLon", 740, toolY+78, 110, 30, "LONGITUDE", 12)
	p.stationAlt = simpleui.NewTextField("satelliteStationAlt", 860, toolY+78, 80, 30, "ALT m", 12)
	p.stationName.SetText(station.Name)
	p.stationLat.SetText(strconv.FormatFloat(station.Latitude, 'f', 5, 64))
	p.stationLon.SetText(strconv.FormatFloat(station.Longitude, 'f', 5, 64))
	p.stationAlt.SetText(strconv.FormatFloat(station.AltitudeMeters, 'f', 0, 64))
	p.controls = append(p.controls, p.searchField, p.stationName, p.stationLat, p.stationLon, p.stationAlt)
	applyStation := button("satelliteStationApply", "APPLY", 950, 115, p.applyStation)
	applyStation.SetBounds(rl.Rectangle{X: 950, Y: toolY + 78, Width: 115, Height: 30})
	p.populate()
	p.SetVisible(false)
	p.writeSnapshot(true)
	return p
}

func (p *SatellitePanel) sortedSatellites() []satellite.Satellite {
	s := p.tracker.Satellites()
	sort.SliceStable(s, func(i, j int) bool {
		if s[i].NORAD == 25544 {
			return true
		}
		if s[j].NORAD == 25544 {
			return false
		}
		if s[i].NORAD == 43700 {
			return true
		}
		if s[j].NORAD == 43700 {
			return false
		}
		return s[i].Name < s[j].Name
	})
	return s
}
func (p *SatellitePanel) populate() {
	sats := p.sortedSatellites()
	groups := make([]string, 0)
	seen := map[string]bool{}
	selectedGroup := ""
	for _, sat := range sats {
		if !seen[sat.Group] {
			seen[sat.Group] = true
			groups = append(groups, sat.Group)
		}
		if sat.NORAD == p.tracker.Selected() {
			selectedGroup = sat.Group
		}
	}
	if selectedGroup == "" && len(groups) > 0 {
		selectedGroup = groups[0]
	}
	p.groupSelect.SetItems(groups)
	for i, group := range groups {
		if group == selectedGroup {
			p.groupSelect.SetSelected(i)
			break
		}
	}
	p.currentGroup = selectedGroup
	p.populateSatellites(p.tracker.Selected())
	p.updateSearch(p.searchField.Text())
}

func (p *SatellitePanel) groupSatellites(group string) []satellite.Satellite {
	var result []satellite.Satellite
	for _, sat := range p.sortedSatellites() {
		if sat.Group == group {
			result = append(result, sat)
		}
	}
	return result
}

func (p *SatellitePanel) populateSatellites(preferredNORAD int) {
	sats := p.groupSatellites(p.currentGroup)
	names := make([]string, len(sats))
	selected := 0
	for i, sat := range sats {
		names[i] = sat.Name
		if sat.NORAD == preferredNORAD {
			selected = i
		}
	}
	p.satelliteSelect.SetItems(names)
	if len(sats) == 0 {
		p.satelliteSelect.SetSelected(-1)
		return
	}
	p.satelliteSelect.SetSelected(selected)
	p.selectSatellite(sats[selected])
}

func (p *SatellitePanel) selectSatellite(sat satellite.Satellite) {
	p.tracker.Select(sat.NORAD)
	p.refreshSignals(sat)
	p.writeSnapshot(true)
}

func (p *SatellitePanel) updateSearch(query string) {
	query = strings.ToLower(strings.TrimSpace(query))
	p.searchMatches = p.searchMatches[:0]
	if query == "" {
		return
	}
	for _, sat := range p.sortedSatellites() {
		haystack := strings.ToLower(fmt.Sprintf("%s %s %d", sat.Name, sat.Group, sat.NORAD))
		if strings.Contains(haystack, query) {
			p.searchMatches = append(p.searchMatches, sat)
			if len(p.searchMatches) == 5 {
				break
			}
		}
	}
}

func (p *SatellitePanel) selectSearchMatch(sat satellite.Satellite) {
	p.currentGroup = sat.Group
	for i, group := range p.groupSelect.Items() {
		if group == sat.Group {
			p.groupSelect.SetSelected(i)
			break
		}
	}
	p.populateSatellites(sat.NORAD)
	p.feedback = "SEARCH: " + sat.Name
}
func (p *SatellitePanel) refreshSignals(s satellite.Satellite) {
	items := make([]string, len(s.Signals))
	for i, x := range s.Signals {
		items[i] = x.Name + " · " + x.Mode
	}
	if len(items) == 0 {
		items = []string{"No catalogued frequency"}
	}
	p.signalSelect.SetItems(items)
	p.signalSelect.SetSelected(0)
}

func (p *SatellitePanel) loadStation() {
	data, err := os.ReadFile(p.stationPath)
	if err != nil {
		return
	}
	var station satellite.Station
	if json.Unmarshal(data, &station) == nil && station.Name != "" && station.Latitude >= -90 && station.Latitude <= 90 && station.Longitude >= -180 && station.Longitude <= 180 {
		p.tracker.SetStation(station)
	}
}

func (p *SatellitePanel) applyStation() {
	lat, latErr := strconv.ParseFloat(strings.TrimSpace(p.stationLat.Text()), 64)
	lon, lonErr := strconv.ParseFloat(strings.TrimSpace(p.stationLon.Text()), 64)
	alt, altErr := strconv.ParseFloat(strings.TrimSpace(p.stationAlt.Text()), 64)
	name := strings.TrimSpace(p.stationName.Text())
	if name == "" || latErr != nil || lonErr != nil || altErr != nil || lat < -90 || lat > 90 || lon < -180 || lon > 180 || alt < -500 || alt > 9000 {
		p.feedback = "INVALID STATION · CHECK NAME, LATITUDE, LONGITUDE AND ALTITUDE"
		return
	}
	station := satellite.Station{Name: name, Latitude: lat, Longitude: lon, AltitudeMeters: alt}
	p.tracker.SetStation(station)
	data, _ := json.MarshalIndent(station, "", "  ")
	_ = os.MkdirAll(filepath.Dir(p.stationPath), 0755)
	if err := os.WriteFile(p.stationPath, data, 0644); err != nil {
		p.feedback = "COULD NOT SAVE STATION: " + err.Error()
		return
	}
	p.feedback = "STATION APPLIED: " + name
	p.writeSnapshot(true)
}
func (p *SatellitePanel) SetVisible(v bool) {
	for _, c := range p.controls {
		c.SetVisible(v)
	}
}
func (p *SatellitePanel) Enter() {
	p.writeSnapshot(true)
	if len(p.tracker.Satellites()) <= 2 {
		p.update()
	}
}
func (p *SatellitePanel) Leave() {}
func (p *SatellitePanel) Tick() {
	p.readMapSelection()
	p.handleSearchResultInput()
	if p.updateDone != nil {
		select {
		case err := <-p.updateDone:
			p.updateDone = nil
			p.updating = false
			if err != nil {
				p.feedback = "ERROR: " + err.Error()
			} else {
				p.feedback = "CATALOG UPDATED"
				p.populate()
				p.writeSnapshot(true)
			}
		default:
		}
	}
	if p.viewerDone != nil {
		select {
		case <-p.viewerDone:
			p.viewer = nil
			p.viewerDone = nil
		default:
		}
	}
	p.writeSnapshot(false)
}
func (p *SatellitePanel) Close() {
	if p.viewer != nil && p.viewer.Process != nil {
		_ = p.viewer.Process.Kill()
	}
}

func (p *SatellitePanel) update() {
	if p.updating {
		return
	}
	p.updating = true
	p.feedback = "UPDATING CATALOG…"
	p.updateDone = make(chan error, 1)
	done := p.updateDone
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		done <- p.tracker.Refresh(ctx)
	}()
}
func (p *SatellitePanel) tune() {
	sats := p.groupSatellites(p.currentGroup)
	i := p.satelliteSelect.SelectedIndex()
	if i < 0 || i >= len(sats) {
		return
	}
	signals := sats[i].Signals
	j := p.signalSelect.SelectedIndex()
	if j < 0 || j >= len(signals) || signals[j].DownlinkHz <= 0 {
		p.feedback = "THIS OBJECT HAS NO CATALOGUED SDR FREQUENCY"
		return
	}
	hz := signals[j].DownlinkHz
	p.screen.frequencyHz = hz
	p.screen.centerFrequencyHz = hz
	p.screen.centerMode = true
	p.screen.updateBandForFrequency(hz)
	if p.screen.receiver != nil {
		p.screen.receiver.SetCenterFrequency(hz)
		mode := "NFM"
		if signals[j].Mode == "SSB/CW" {
			mode = "USB"
		}
		p.screen.receiver.SetDemodulator(mode, hz, p.screen.demodBandwidthHz)
	}
	p.feedback = fmt.Sprintf("TUNED TO %.6f MHz", float64(hz)/1e6)
	p.screen.markSettingsDirty()
}
func (p *SatellitePanel) writeSnapshot(force bool) {
	if !force && time.Now().Before(p.nextSnapshot) {
		return
	}
	p.nextSnapshot = time.Now().Add(time.Second)
	snap := p.tracker.Snapshot(time.Now().UTC())
	snap.Theme = p.screen.themeName
	data, _ := json.Marshal(snap)
	current := string(data)
	if current == p.lastSnapshot {
		return
	}
	p.lastSnapshot = current
	_ = os.MkdirAll(filepath.Dir(p.snapshotPath), 0755)
	if replaceLiveSnapshot(p.snapshotPath, data) != nil {
		p.lastSnapshot = ""
	}
}

func (p *SatellitePanel) readMapSelection() {
	data, err := os.ReadFile(p.snapshotPath + ".select")
	if err != nil {
		return
	}
	norad, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || norad <= 0 || norad == p.lastMapSelection {
		return
	}
	p.lastMapSelection = norad
	for _, sat := range p.sortedSatellites() {
		if sat.NORAD == norad {
			p.selectSearchMatch(sat)
			p.feedback = "SELECTED ON MAP: " + sat.Name
			break
		}
	}
}

func satelliteSearchRowBounds(index int) rl.Rectangle {
	legacy := rl.Rectangle{X: 40, Y: toolY + 89 + float32(index)*18, Width: 390, Height: 18}
	return rl.Rectangle{
		X: toolContentX + (legacy.X-legacyToolX)*toolContentScaleX,
		Y: legacy.Y, Width: legacy.Width * toolContentScaleX, Height: legacy.Height,
	}
}

func (p *SatellitePanel) handleSearchResultInput() {
	if p.screen.activeTool != "SATELLITES" || p.screen.viewMode != 1 || simpleui.PointerInputBlocked() || !rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
		return
	}
	mouse := simpleui.MousePosition()
	for i, sat := range p.searchMatches {
		if rl.CheckCollisionPointRec(mouse, satelliteSearchRowBounds(i)) {
			p.selectSearchMatch(sat)
			return
		}
	}
}
func (p *SatellitePanel) openMap() {
	p.writeSnapshot(true)
	if p.viewer != nil && p.viewer.Process != nil {
		focusRTL433Viewer(p.viewer.Process.Pid)
		p.feedback = "MAP ALREADY OPEN"
		return
	}
	exe, err := os.Executable()
	if err != nil {
		p.feedback = "ERROR OPENING MAP"
		return
	}
	cmd := exec.Command(exe, "--satellite-map", p.snapshotPath)
	cmd.SysProcAttr = rtl433ViewerProcessAttributes()
	if err = cmd.Start(); err != nil {
		p.feedback = "ERROR OPENING MAP"
		return
	}
	p.viewer = cmd
	p.viewerDone = make(chan struct{})
	done := p.viewerDone
	go func() { _ = cmd.Wait(); close(done) }()
	p.feedback = "MAP OPENED"
}

func (p *SatellitePanel) DrawPanel() {
	snap := p.tracker.Snapshot(time.Now().UTC())
	var selected satellite.State
	for _, s := range snap.Satellites {
		if s.NORAD == snap.SelectedNORAD {
			selected = s
			break
		}
	}
	eye := "BELOW THE HORIZON"
	c := colors.muted
	if selected.Visible {
		eye = "VISIBLE FROM " + snap.Station.Name
		c = colors.green
	}
	simpleui.DrawText("SATELLITES · "+selected.Name+" · "+eye, 40, toolY+7, 12, c)
	if p.feedback != "" {
		simpleui.DrawText(sondeClip(p.feedback, 72), 1035, toolY+7, 11, colors.muted)
	}
	freq := "--"
	if selected.DownlinkHz > 0 {
		freq = fmt.Sprintf("%.6f MHz", float64(selected.DownlinkHz)/1e6)
	}

	simpleui.DrawText("SATELLITE", 40, toolY+73, 10, colors.muted)
	simpleui.DrawText("GROUP", 235, toolY+73, 10, colors.muted)
	simpleui.DrawText("NORAD", 365, toolY+73, 10, colors.muted)
	if p.searchField.Text() == "" {
		simpleui.DrawText("Type to search the whole catalog", 40, toolY+96, 12, colors.muted)
	} else if len(p.searchMatches) == 0 {
		simpleui.DrawText("No matches", 40, toolY+96, 12, colors.muted)
	} else {
		mouse := simpleui.MousePosition()
		for i, sat := range p.searchMatches {
			y := toolY + 92 + float32(i)*18
			legacy := rl.Rectangle{X: 40, Y: y - 3, Width: 390, Height: 18}
			actual := satelliteSearchRowBounds(i)
			if rl.CheckCollisionPointRec(mouse, actual) {
				rl.DrawRectangleRec(legacy, mixColor(colors.panelAlt, colors.cyan, .12))
			}
			simpleui.DrawText(sondeClip(sat.Name, 23), 40, y, 11, colors.text)
			simpleui.DrawText(sondeClip(sat.Group, 16), 235, y, 11, colors.text)
			simpleui.DrawText(strconv.Itoa(sat.NORAD), 365, y, 11, colors.text)
		}
	}

	simpleui.DrawText("CURRENT SELECTION", 470, toolY+119, 10, colors.muted)
	simpleui.DrawText(fmt.Sprintf("AZ %.1f°   EL %.1f°   ALT %.0f km   DIST %.0f km", selected.Azimuth, selected.Elevation, selected.AltitudeKM, selected.RangeKM), 495, toolY+142, 12, colors.text)
	simpleui.DrawText(fmt.Sprintf("%s · %s · %d objetos · %s", freq, selected.Mode, len(snap.Satellites), snap.Source), 495, toolY+168, 11, colors.muted)
}
