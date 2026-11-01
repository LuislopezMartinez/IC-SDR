package screens

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"go-zero/internal/localtime"
	"go-zero/internal/worldclock"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type worldClockMap struct {
	request            worldClockMapRequest
	location           worldclock.Location
	lat, lon, span     float64
	manager            *simpleui.Manager
	name               *simpleui.TextField
	confirm, cancel    *simpleui.Button
	press, last        rl.Vector2
	mapGesture, moved  bool
	resolvedDistanceKM float64
	message            string
}

func RunWorldClockMap(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var request worldClockMapRequest
	if json.Unmarshal(data, &request) != nil || request.ResultPath == "" {
		return
	}
	simpleui.SetMode(1440, 900, simpleui.Stretch)
	simpleui.SetMinimumSize(960, 600)
	simpleui.SetTitle(fmt.Sprintf("HORA INTERNACIONAL · UBICACIÓN DEL RELOJ %d", request.Clock+1))
	simpleui.SetTextScale(1.15)
	v := newWorldClockMap(request)
	simpleui.Run(v.draw)
}

func newWorldClockMap(request worldClockMapRequest) *worldClockMap {
	location := worldclock.Normalize(request.Location, time.Now().UTC())
	v := &worldClockMap{
		request: request, location: location,
		lat: location.Lat, lon: location.Lon, span: 24,
		manager: simpleui.NewManager(), resolvedDistanceKM: -1,
	}
	v.name = simpleui.NewTextField("worldClockMapName", 1060, 253, 350, 44, "CIUDAD / ETIQUETA", 16)
	v.name.SetMaxLength(48)
	v.name.SetText(location.City)
	v.confirm = simpleui.NewButton("worldClockMapConfirm", 1060, 734, 350, 52, "USAR UBICACIÓN", 15)
	v.confirm.SetColors(colors.blue, colors.cyan, colors.text)
	v.confirm.OnClick(v.save)
	v.cancel = simpleui.NewButton("worldClockMapCancel", 1060, 801, 350, 44, "CANCELAR", 14)
	v.cancel.SetColors(colors.panelAlt, colors.border, colors.text)
	v.cancel.OnClick(simpleui.RequestClose)
	v.manager.Add(v.name)
	v.manager.Add(v.confirm)
	v.manager.Add(v.cancel)
	v.resolvePin(false)
	return v
}

func (v *worldClockMap) mapBounds() rl.Rectangle {
	return rl.Rectangle{X: 0, Y: 0, Width: 1028, Height: 900}
}

func (v *worldClockMap) resolvePin(updateName bool) {
	if info, ok := localtime.At(time.Now().UTC(), v.location.Lat, v.location.Lon); ok {
		v.location.Zone = info.Zone
		city, km := worldclock.Resolve(v.location.Lat, v.location.Lon, info.Zone)
		v.resolvedDistanceKM = km
		if city.Name != "" && km <= worldclock.NearestCityMaxKM {
			v.location.City, v.location.Country = city.Name, city.Country
		} else {
			v.location.City = fmt.Sprintf("%.2f°, %.2f°", v.location.Lat, v.location.Lon)
			v.location.Country = ""
		}
		if updateName {
			v.name.SetText(v.location.City)
		}
	}
}

func (v *worldClockMap) save() {
	label := strings.TrimSpace(v.name.Text())
	if label != "" {
		v.location.City = label
	}
	v.location = worldclock.Normalize(v.location, time.Now().UTC())
	data, err := json.Marshal(worldClockMapResult{Clock: v.request.Clock, Location: v.location})
	if err != nil || replaceLiveSnapshot(v.request.ResultPath, data) != nil {
		v.message = "NO SE PUDO GUARDAR LA UBICACIÓN"
		return
	}
	simpleui.RequestClose()
}

func (v *worldClockMap) input() {
	mouse := simpleui.MousePosition()
	bounds := v.mapBounds()
	v.manager.Update(simpleui.Input{
		Pointer: mouse, PointerInCanvas: true,
		Pressed: rl.IsMouseButtonPressed(rl.MouseButtonLeft), Down: rl.IsMouseButtonDown(rl.MouseButtonLeft),
		Released: rl.IsMouseButtonReleased(rl.MouseButtonLeft), Wheel: rl.GetMouseWheelMove(),
	})
	inside := rl.CheckCollisionPointRec(mouse, bounds)
	if rl.IsMouseButtonPressed(rl.MouseButtonLeft) && inside {
		v.mapGesture, v.moved = true, false
		v.press, v.last = mouse, mouse
	}
	if v.mapGesture && rl.IsMouseButtonDown(rl.MouseButtonLeft) {
		delta := rl.Vector2Subtract(mouse, v.last)
		if rl.Vector2Distance(mouse, v.press) > 4 {
			v.moved = true
		}
		if v.moved {
			v.lat, v.lon = panMap(v.lat, v.lon, v.span, delta, bounds)
		}
		v.last = mouse
	}
	if v.mapGesture && rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
		if !v.moved && inside {
			lat, lon := unprojectMercator(v.lat, v.lon, v.span, mouse, bounds)
			v.location.Lat = math.Max(-mercatorMaxLat, math.Min(mercatorMaxLat, lat))
			v.location.Lon = lon
			v.resolvePin(true)
		}
		v.mapGesture = false
	}
	if wheel := rl.GetMouseWheelMove(); wheel != 0 && inside {
		v.lat, v.lon, v.span = zoomMap(v.lat, v.lon, v.span, mouse, bounds, wheel)
	}
	v.lat, v.lon, v.span = clampMapView(v.lat, v.lon, v.span, bounds)
}

func (v *worldClockMap) draw() {
	v.input()
	bounds := v.mapBounds()
	drawGeoMap(v.lat, v.lon, v.span, bounds)
	drawMapGrid(v.lat, v.lon, v.span, bounds)
	pin := projectMercator(v.lat, v.lon, v.span, v.location.Lat, v.location.Lon, bounds)
	drawWorldClockPin(pin)
	v.drawPanel()
	v.manager.Draw()
}

func drawWorldClockPin(point rl.Vector2) {
	shadow := rl.Vector2{X: point.X + 2, Y: point.Y + 3}
	rl.DrawCircleV(shadow, 10, rl.Color{A: 95})
	rl.DrawTriangle(rl.Vector2{X: shadow.X - 7, Y: shadow.Y + 5}, rl.Vector2{X: shadow.X + 7, Y: shadow.Y + 5}, rl.Vector2{X: shadow.X, Y: shadow.Y + 25}, rl.Color{A: 95})
	rl.DrawCircleV(point, 11, colors.orange)
	rl.DrawTriangle(rl.Vector2{X: point.X - 8, Y: point.Y + 5}, rl.Vector2{X: point.X + 8, Y: point.Y + 5}, rl.Vector2{X: point.X, Y: point.Y + 25}, colors.orange)
	rl.DrawCircleV(point, 4, colors.panel)
}

func (v *worldClockMap) drawPanel() {
	panel := rl.Rectangle{X: 1028, Y: 0, Width: 412, Height: 900}
	rl.DrawRectangleRec(panel, colors.background)
	rl.DrawLineEx(rl.Vector2{X: panel.X, Y: 0}, rl.Vector2{X: panel.X, Y: 900}, 2, colors.cyan)
	simpleui.DrawTextStyled("SELECCIONAR UBICACIÓN", 1060, 35, 22, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText(fmt.Sprintf("RELOJ %d", v.request.Clock+1), 1060, 70, 13, colors.orange)
	simpleui.DrawText("HAGA CLIC EN EL MAPA PARA COLOCAR EL PIN", 1060, 112, 11, colors.text)
	simpleui.DrawText("ARRASTRE PARA MOVER · RUEDA PARA ZOOM", 1060, 136, 10, colors.muted)

	info, ok := localtime.At(time.Now().UTC(), v.location.Lat, v.location.Lon)
	if ok {
		simpleui.DrawTextStyled(info.Time.Format("15:04:05"), 1060, 181, 30, simpleui.FontMono, colors.green)
		simpleui.DrawTextStyled(info.UTCOffset, 1260, 188, 15, simpleui.FontSemiBold, colors.orange)
	}
	simpleui.DrawText("ETIQUETA MOSTRADA EN EL RELOJ", 1060, 232, 10, colors.muted)

	detailY := float32(338)
	details := []struct{ label, value string }{
		{"PAÍS / REGIÓN", v.location.Country},
		{"COORDENADAS", fmt.Sprintf("%.5f, %.5f", v.location.Lat, v.location.Lon)},
		{"ZONA HORARIA", v.location.Zone},
	}
	for _, detail := range details {
		simpleui.DrawText(detail.label, 1060, detailY, 10, colors.muted)
		simpleui.DrawTextStyled(detail.value, 1060, detailY+19, 14, simpleui.FontMono, colors.text)
		detailY += 68
	}
	if v.resolvedDistanceKM >= 0 {
		quality := colors.green
		text := fmt.Sprintf("CIUDAD CALCULADA · %.0f km DEL PIN", v.resolvedDistanceKM)
		if v.resolvedDistanceKM > worldclock.NearestCityMaxKM {
			quality = colors.orange
			text = "SIN CIUDAD CONOCIDA CERCANA · PUEDE EDITAR LA ETIQUETA"
		} else if v.resolvedDistanceKM > 150 {
			quality = colors.orange
			text = fmt.Sprintf("CIUDAD DE REFERENCIA · %.0f km · EDITE SI ES NECESARIO", v.resolvedDistanceKM)
		}
		simpleui.DrawText(text, 1060, 561, 10, quality)
	}
	simpleui.DrawText("La zona horaria procede siempre de la posición", 1060, 614, 11, colors.text)
	simpleui.DrawText("exacta del pin, no de la ciudad sugerida.", 1060, 635, 11, colors.text)
	simpleui.DrawText("Funciona sin servicios de geolocalización externos.", 1060, 671, 10, colors.muted)
	if v.message != "" {
		simpleui.DrawText(v.message, 1060, 704, 11, colors.red)
	}
}
