package screens

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"go-zero/internal/localtime"
	"go-zero/internal/resources"
	"go-zero/internal/worldclock"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const worldClockToolID = "WORLD_CLOCK"

type worldClockMapRequest struct {
	Clock      int                 `json:"clock"`
	Location   worldclock.Location `json:"location"`
	ResultPath string              `json:"resultPath"`
}

type worldClockMapResult struct {
	Clock    int                 `json:"clock"`
	Location worldclock.Location `json:"location"`
}

type WorldClockPanel struct {
	screen      *MainScreen
	controls    []simpleui.Element
	buttons     [5]*simpleui.Button
	config      worldclock.Config
	configPath  string
	requestPath string
	resultPath  string
	viewer      *exec.Cmd
	viewerDone  chan struct{}
	viewerClock int
	nextPoll    time.Time
	status      string
	statusUntil time.Time
}

func NewWorldClockPanel(screen *MainScreen) *WorldClockPanel {
	p := &WorldClockPanel{
		screen: screen, viewerClock: -1,
		configPath:  resources.WritablePath("config", "world-clocks.json"),
		requestPath: resources.WritablePath("cache", "world-clock-map-request.json"),
		resultPath:  resources.WritablePath("cache", "world-clock-map-result.json"),
	}
	p.config = worldclock.Load(p.configPath)
	for index := range p.buttons {
		i := index
		x := float32(374 + index*238)
		button := simpleui.NewButton(fmt.Sprintf("worldClockLocation%d", index), x, toolY+218, 206, 32, "UBICACIÓN", 12)
		button.SetColors(colors.panelAlt, colors.cyan, rl.Color{R: 235, G: 239, B: 238, A: 255})
		button.OnClick(func() { p.openMap(i) })
		p.buttons[index] = button
		p.controls = append(p.controls, button)
	}
	p.SetVisible(false)
	return p
}

func (p *WorldClockPanel) SetVisible(visible bool) {
	for _, control := range p.controls {
		control.SetVisible(visible)
	}
}

func (p *WorldClockPanel) openMap(index int) {
	if index < 0 || index >= len(p.config.Locations) {
		return
	}
	if p.viewer != nil && p.viewer.Process != nil {
		select {
		case <-p.viewerDone:
			p.viewer, p.viewerDone = nil, nil
		default:
			focusRTL433Viewer(p.viewer.Process.Pid)
			p.status, p.statusUntil = "EL SELECTOR DE UBICACIÓN YA ESTÁ ABIERTO", time.Now().Add(3*time.Second)
			return
		}
	}
	_ = os.Remove(p.resultPath)
	request := worldClockMapRequest{Clock: index, Location: p.config.Locations[index], ResultPath: p.resultPath}
	data, err := json.Marshal(request)
	if err != nil {
		p.status = "NO SE PUDO PREPARAR EL SELECTOR"
		return
	}
	if err = os.MkdirAll(filepath.Dir(p.requestPath), 0o755); err == nil {
		err = replaceLiveSnapshot(p.requestPath, data)
	}
	if err != nil {
		p.status = "NO SE PUDO PREPARAR EL SELECTOR"
		return
	}
	executable, err := os.Executable()
	if err != nil {
		p.status = "NO SE PUDO ABRIR EL MAPA"
		return
	}
	command := exec.Command(executable, "--world-clock-map", p.requestPath)
	command.SysProcAttr = rtl433ViewerProcessAttributes()
	if err = command.Start(); err != nil {
		p.status = "NO SE PUDO ABRIR EL MAPA"
		return
	}
	p.viewer, p.viewerClock = command, index
	p.viewerDone = make(chan struct{})
	done := p.viewerDone
	go func() { _ = command.Wait(); close(done) }()
	p.status, p.statusUntil = "SELECTOR ABIERTO · RELOJ "+fmt.Sprint(index+1), time.Now().Add(3*time.Second)
}

func (p *WorldClockPanel) Tick() {
	if time.Now().Before(p.nextPoll) {
		return
	}
	p.nextPoll = time.Now().Add(200 * time.Millisecond)
	data, err := os.ReadFile(p.resultPath)
	if err == nil {
		var result worldClockMapResult
		if json.Unmarshal(data, &result) == nil && result.Clock >= 0 && result.Clock < len(p.config.Locations) {
			result.Location = worldclock.Normalize(result.Location, time.Now().UTC())
			p.config.Locations[result.Clock] = result.Location
			if saveErr := worldclock.Save(p.configPath, p.config); saveErr == nil {
				p.status, p.statusUntil = "UBICACIÓN ACTUALIZADA · "+result.Location.City, time.Now().Add(4*time.Second)
			} else {
				p.status, p.statusUntil = "NO SE PUDO GUARDAR LA UBICACIÓN", time.Now().Add(4*time.Second)
			}
		}
		_ = os.Remove(p.resultPath)
	}
	if p.viewerDone != nil {
		select {
		case <-p.viewerDone:
			p.viewer, p.viewerDone, p.viewerClock = nil, nil, -1
		default:
		}
	}
}

func (p *WorldClockPanel) Close() {
	if p.viewer != nil && p.viewer.Process != nil {
		_ = p.viewer.Process.Kill()
	}
}

func (p *WorldClockPanel) DrawPanel() {
	x, y, width := float32(360), toolY, float32(1232)
	drawPanel(x, y, width, toolH)
	drawClockText("HORA INTERNACIONAL", x+18, y+10, 20, simpleui.FontSemiBold, colors.cyan)
	now := time.Now()
	drawClockText("UTC  "+now.UTC().Format("15:04:05"), x+280, y+13, 15, simpleui.FontMono, colors.orange)
	drawClockText("LOCAL  "+now.Format("15:04:05"), x+465, y+13, 15, simpleui.FontMono, colors.green)
	if p.status != "" && time.Now().Before(p.statusUntil) {
		simpleui.DrawText(p.status, x+760, y+15, 11, colors.muted)
	}
	for index, location := range p.config.Locations {
		p.drawClock(index, location, now.UTC())
	}
}

func (p *WorldClockPanel) drawClock(index int, location worldclock.Location, now time.Time) {
	card := rl.Rectangle{X: float32(366 + index*238), Y: toolY + 42, Width: 222, Height: 166}
	fill := colors.panelAlt
	if index == 0 {
		fill = mixColor(fill, colors.cyan, .08)
	}
	rl.DrawRectangleRounded(card, .06, 8, fill)
	rl.DrawRectangleRoundedLinesEx(card, .06, 8, 1.4, colors.border)
	info, ok := localtime.At(now, location.Lat, location.Lon)
	local := now
	offset, zone := "UTC", location.Zone
	if ok {
		local, offset, zone = info.Time, info.UTCOffset, info.Zone
	}
	name := location.City
	if name == "" {
		name = zone
	}
	drawClockCityPlaque(card, name)
	// The face and readout follow the corrected first-card geometry: the clock
	// sits farther left and leaves a balanced, readable column for the data.
	center := rl.Vector2{X: card.X + 62, Y: card.Y + 93}
	drawAnalogClock(center, 51, local)
	drawClockText(local.Format("15:04:05"), card.X+119, card.Y+68, 18, simpleui.FontMono, colors.cyan)
	drawClockText(offset, card.X+119, card.Y+96, 13, simpleui.FontSemiBold, colors.orange)
	zoneLabel := zone
	if len(zoneLabel) > 19 {
		zoneLabel = zoneLabel[len(zoneLabel)-19:]
	}
	zoneColor := mixColor(colors.muted, colors.text, .38)
	drawClockText(zoneLabel, card.X+119, card.Y+121, 10, simpleui.FontMono, zoneColor)
}

func drawClockCityPlaque(card rl.Rectangle, name string) {
	plate := rl.Rectangle{X: card.X + 30, Y: card.Y + 5, Width: card.Width - 60, Height: 29}
	shadow := plate
	shadow.X, shadow.Y = shadow.X+1.5, shadow.Y+2.5
	drawClippedClockPlate(shadow, 7, rl.Color{R: 0, G: 0, B: 0, A: 105})

	// A restrained brass frame gives the clock a classical instrument identity
	// while the dark inset keeps it compatible with the SDR interface.
	brassDark := rl.Color{R: 126, G: 79, B: 22, A: 255}
	brass := rl.Color{R: 205, G: 148, B: 56, A: 255}
	brassLight := rl.Color{R: 244, G: 201, B: 111, A: 255}
	inset := rl.Color{R: 18, G: 22, B: 23, A: 255}
	drawClippedClockPlate(plate, 7, brassDark)
	drawClippedClockPlate(rl.Rectangle{X: plate.X + 1, Y: plate.Y + 1, Width: plate.Width - 2, Height: plate.Height - 2}, 6, brass)
	drawClippedClockPlate(rl.Rectangle{X: plate.X + 4, Y: plate.Y + 4, Width: plate.Width - 8, Height: plate.Height - 8}, 4, inset)
	rl.DrawLineEx(rl.Vector2{X: plate.X + 9, Y: plate.Y + 2}, rl.Vector2{X: plate.X + plate.Width - 9, Y: plate.Y + 2}, 1, brassLight)

	for _, x := range []float32{plate.X + 12, plate.X + plate.Width - 12} {
		center := rl.Vector2{X: x, Y: plate.Y + plate.Height/2}
		rl.DrawCircleV(center, 4.2, brassDark)
		rl.DrawCircleV(center, 3.2, brassLight)
		rl.DrawLineEx(rl.Vector2{X: center.X - 2, Y: center.Y + 2}, rl.Vector2{X: center.X + 2, Y: center.Y - 2}, 1.1, rl.Color{R: 82, G: 49, B: 14, A: 255})
	}

	textBounds := rl.Rectangle{X: plate.X + 20, Y: plate.Y + 3, Width: plate.Width - 40, Height: plate.Height - 6}
	fontSize := int32(14)
	for fontSize > 9 && simpleui.MeasureTextStyled(name, fontSize, simpleui.FontSemiBold).X > textBounds.Width-4 {
		fontSize--
	}
	ivory := rl.Color{R: 246, G: 235, B: 205, A: 255}
	drawCenteredStyled(name, rl.Rectangle{X: textBounds.X + 1, Y: textBounds.Y + 1, Width: textBounds.Width, Height: textBounds.Height}, fontSize, simpleui.FontSemiBold, rl.Color{R: 0, G: 0, B: 0, A: 180})
	drawCenteredStyled(name, textBounds, fontSize, simpleui.FontSemiBold, ivory)
}

func drawClippedClockPlate(bounds rl.Rectangle, cut float32, color rl.Color) {
	rl.DrawRectangleRec(rl.Rectangle{X: bounds.X + cut, Y: bounds.Y, Width: bounds.Width - cut*2, Height: bounds.Height}, color)
	rl.DrawRectangleRec(rl.Rectangle{X: bounds.X, Y: bounds.Y + cut, Width: bounds.Width, Height: bounds.Height - cut*2}, color)
	rl.DrawTriangle(rl.Vector2{X: bounds.X + cut, Y: bounds.Y}, rl.Vector2{X: bounds.X + cut, Y: bounds.Y + cut}, rl.Vector2{X: bounds.X, Y: bounds.Y + cut}, color)
	rl.DrawTriangle(rl.Vector2{X: bounds.X + bounds.Width - cut, Y: bounds.Y}, rl.Vector2{X: bounds.X + bounds.Width, Y: bounds.Y + cut}, rl.Vector2{X: bounds.X + bounds.Width - cut, Y: bounds.Y + cut}, color)
	rl.DrawTriangle(rl.Vector2{X: bounds.X, Y: bounds.Y + bounds.Height - cut}, rl.Vector2{X: bounds.X + cut, Y: bounds.Y + bounds.Height - cut}, rl.Vector2{X: bounds.X + cut, Y: bounds.Y + bounds.Height}, color)
	rl.DrawTriangle(rl.Vector2{X: bounds.X + bounds.Width - cut, Y: bounds.Y + bounds.Height - cut}, rl.Vector2{X: bounds.X + bounds.Width, Y: bounds.Y + bounds.Height - cut}, rl.Vector2{X: bounds.X + bounds.Width - cut, Y: bounds.Y + bounds.Height}, color)
}

func drawClockText(text string, x, y float32, size int32, style simpleui.FontStyle, color rl.Color) {
	simpleui.DrawTextStyled(text, x+1, y+1, size, style, rl.Color{R: 0, G: 0, B: 0, A: 145})
	simpleui.DrawTextStyled(text, x, y, size, style, color)
}

func drawAnalogClock(center rl.Vector2, radius float32, value time.Time) {
	face := rl.Color{R: 235, G: 239, B: 238, A: 255}
	rl.DrawCircleV(center, radius+4, rl.Color{R: 48, G: 67, B: 78, A: 255})
	rl.DrawCircleV(center, radius+2, rl.Color{R: 177, G: 188, B: 190, A: 255})
	rl.DrawCircleV(center, radius, face)
	for tick := 0; tick < 60; tick++ {
		angle := float64(tick)*math.Pi/30 - math.Pi/2
		outer := rl.Vector2{X: center.X + float32(math.Cos(angle))*radius*.91, Y: center.Y + float32(math.Sin(angle))*radius*.91}
		length, thickness := float32(4), float32(1)
		if tick%5 == 0 {
			length, thickness = 8, 2.2
		}
		inner := rl.Vector2{X: center.X + float32(math.Cos(angle))*(radius*.91-length), Y: center.Y + float32(math.Sin(angle))*(radius*.91-length)}
		rl.DrawLineEx(inner, outer, thickness, rl.Color{R: 34, G: 44, B: 50, A: 255})
	}
	drawClockHand(center, (float64(value.Hour()%12)+float64(value.Minute())/60)*math.Pi/6-math.Pi/2, radius*.48, 4.4, rl.Color{R: 23, G: 34, B: 40, A: 255})
	drawClockHand(center, (float64(value.Minute())+float64(value.Second())/60)*math.Pi/30-math.Pi/2, radius*.72, 3, rl.Color{R: 23, G: 34, B: 40, A: 255})
	drawClockHand(center, float64(value.Second())*math.Pi/30-math.Pi/2, radius*.78, 1.4, rl.Color{R: 220, G: 57, B: 50, A: 255})
	rl.DrawCircleV(center, 4, rl.Color{R: 220, G: 57, B: 50, A: 255})
}

func drawClockHand(center rl.Vector2, angle float64, length, thickness float32, color rl.Color) {
	end := rl.Vector2{X: center.X + float32(math.Cos(angle))*length, Y: center.Y + float32(math.Sin(angle))*length}
	rl.DrawLineEx(center, end, thickness, color)
}
