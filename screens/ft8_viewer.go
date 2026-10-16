package screens

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"go-zero/internal/ft8"
	"go-zero/internal/resources"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type ft8ViewerSnapshot struct {
	Status      ft8.Status `json:"status"`
	SelectedKey string     `json:"selectedKey,omitempty"`
	HomeLat     float64    `json:"homeLat"`
	HomeLon     float64    `json:"homeLon"`
	Theme       string     `json:"theme"`
}

type ft8ViewerProcess struct {
	cmd  *exec.Cmd
	done chan struct{}
}

func ft8ViewerSnapshotPath() string {
	return resources.WritablePath("cache", "ft8-activity.json")
}

func (p *FT8Panel) writeViewerSnapshot() {
	if p.snapshotPath == "" {
		return
	}
	homeLat, homeLon := p.ft8HomePosition()
	status := p.status
	// The map does not render the 15-second capture progress. Excluding this
	// rapidly changing value avoids rewriting a large session snapshot at 10 Hz.
	status.Progress = 0
	snapshot := ft8ViewerSnapshot{Status: status, SelectedKey: p.viewerSelectedKey, HomeLat: homeLat, HomeLon: homeLon, Theme: p.screen.themeName}
	data, err := json.Marshal(snapshot)
	if err != nil || string(data) == p.lastSnapshot {
		return
	}
	_ = os.MkdirAll(filepath.Dir(p.snapshotPath), 0755)
	if replaceLiveSnapshot(p.snapshotPath, data) == nil {
		p.lastSnapshot = string(data)
	}
}

func (p *FT8Panel) openExternalViewer(preferred int) {
	if preferred >= 0 && preferred < len(p.status.Decodes) {
		p.viewerSelectedKey = ft8DecodeKey(p.status.Decodes[preferred])
	} else {
		p.viewerSelectedKey = ""
	}
	if p.screen.receiver != nil {
		p.status = p.screen.receiver.FT8Status()
	}
	p.writeViewerSnapshot()
	if p.viewer != nil && p.viewer.cmd != nil && p.viewer.cmd.Process != nil {
		focusRTL433Viewer(p.viewer.cmd.Process.Pid)
		return
	}
	executable, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(executable, "--ft8-viewer", p.snapshotPath)
	cmd.SysProcAttr = rtl433ViewerProcessAttributes()
	if cmd.Start() != nil {
		return
	}
	done := make(chan struct{})
	p.viewer = &ft8ViewerProcess{cmd: cmd, done: done}
	go func() { _ = cmd.Wait(); close(done) }()
}

func (p *FT8Panel) updateExternalViewer() {
	if p.viewer == nil {
		return
	}
	select {
	case <-p.viewer.done:
		p.viewer = nil
		p.lastSnapshot = ""
	default:
		p.writeViewerSnapshot()
	}
}

func (p *FT8Panel) closeExternalViewer() {
	if p.viewer != nil && p.viewer.cmd != nil && p.viewer.cmd.Process != nil {
		_ = p.viewer.cmd.Process.Kill()
	}
	p.viewer = nil
}

type ft8ActivityViewer struct {
	path, selectedKey string
	panel             *FT8Panel
	nextRead          time.Time
}

// RunFT8Viewer runs in a separate process because raylib owns one native
// window per process. The main receiver publishes a live read-only snapshot.
func RunFT8Viewer(path string) {
	// This is a data-dense dashboard: consume the complete native client area
	// even when its aspect ratio differs from the design canvas. Bilinear
	// sampling keeps glyph edges smooth at non-integer Windows DPI scales.
	simpleui.SetMode(int32(designWidth), int32(designHeight), simpleui.Stretch)
	simpleui.SetInitialWindowSize(int32(designWidth), int32(designHeight))
	simpleui.SetMinimumSize(960, 540)
	simpleui.SetCanvasFilter(rl.FilterBilinear)
	simpleui.SetTextScale(1.25)
	simpleui.SetTitle("IC-SDR · ACTIVIDAD FT8 GLOBAL")
	simpleui.SetMaximizeKey(0)
	colors = lightPalette
	simpleui.SetTheme(simpleUITheme(colors, true))
	screen := &MainScreen{themeName: themeLight, ft8Depth: ft8.DepthBalanced}
	panel := NewFT8Panel(screen)
	panel.mapOpen, panel.viewerMode = true, true
	panel.mapWindow, panel.mapLat, panel.mapLon, panel.mapSpan = time.Hour, 10, 0, 360
	viewer := &ft8ActivityViewer{path: path, panel: panel}
	simpleui.Run(viewer.draw)
}

func (v *ft8ActivityViewer) read() {
	if time.Now().Before(v.nextRead) {
		return
	}
	v.nextRead = time.Now().Add(100 * time.Millisecond)
	data, err := os.ReadFile(v.path)
	if err != nil {
		return
	}
	var snapshot ft8ViewerSnapshot
	if json.Unmarshal(data, &snapshot) != nil {
		return
	}
	if validTheme(snapshot.Theme) && snapshot.Theme != v.panel.screen.themeName {
		v.panel.screen.themeName = snapshot.Theme
		colors = paletteForTheme(snapshot.Theme)
		simpleui.SetTheme(simpleUITheme(colors, snapshot.Theme == themeLight))
	}
	v.panel.status = snapshot.Status
	v.panel.mapDecodes = append(v.panel.mapDecodes[:0], snapshot.Status.Decodes...)
	v.panel.mapHomeLat, v.panel.mapHomeLon, v.panel.mapHomeSet = snapshot.HomeLat, snapshot.HomeLon, true
	if snapshot.SelectedKey != v.selectedKey {
		v.selectedKey = snapshot.SelectedKey
		v.panel.mapSelected = -1
		for index, decode := range v.panel.mapDecodes {
			if ft8DecodeKey(decode) == snapshot.SelectedKey {
				v.panel.mapSelected = index
				break
			}
		}
	}
}

func (v *ft8ActivityViewer) draw() {
	v.read()
	input := simpleui.Input{Pointer: simpleui.MousePosition(), PointerInCanvas: simpleui.MouseInViewport(), Pressed: rl.IsMouseButtonPressed(rl.MouseButtonLeft), Down: rl.IsMouseButtonDown(rl.MouseButtonLeft), Released: rl.IsMouseButtonReleased(rl.MouseButtonLeft), Wheel: rl.GetMouseWheelMove()}
	v.panel.UpdateOverlay(input)
	if !v.panel.mapOpen {
		simpleui.RequestClose()
		return
	}
	v.panel.DrawOverlay()
}
