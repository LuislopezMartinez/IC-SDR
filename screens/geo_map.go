package screens

import (
	"fmt"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/simpleui"
)

const mercatorMaxLat = 85.05112878

func wrapLongitude(lon float64) float64 {
	return lon - math.Floor((lon+180)/360)*360
}

func mapZoom(lonSpan float64, width float32) float64 {
	if lonSpan <= 0 || width <= 0 {
		return 2
	}
	return math.Max(2, math.Min(18.5, math.Log2(360/lonSpan*float64(width)/256)))
}

func mercatorX(lon, zoom float64) float64 {
	return (wrapLongitude(lon) + 180) / 360 * math.Exp2(zoom)
}

func mercatorY(lat, zoom float64) float64 {
	lat = math.Max(-mercatorMaxLat, math.Min(mercatorMaxLat, lat))
	s := math.Sin(lat * math.Pi / 180)
	return (1 - math.Log((1+s)/(1-s))/(2*math.Pi)) / 2 * math.Exp2(zoom)
}

func mercatorLon(x, zoom float64) float64 {
	n := math.Exp2(zoom)
	return wrapLongitude(x/n*360 - 180)
}

func mercatorLat(y, zoom float64) float64 {
	n := math.Exp2(zoom)
	return math.Atan(math.Sinh(math.Pi*(1-2*y/n))) * 180 / math.Pi
}

func projectMercator(centerLat, centerLon, lonSpan, lat, lon float64, b rl.Rectangle) rl.Vector2 {
	z := mapZoom(lonSpan, b.Width)
	cx, cy := mercatorX(centerLon, z), mercatorY(centerLat, z)
	px, py := mercatorX(lon, z), mercatorY(lat, z)
	dx := px - cx
	n := math.Exp2(z)
	if dx > n/2 {
		dx -= n
	} else if dx < -n/2 {
		dx += n
	}
	return rl.Vector2{
		X: b.X + b.Width/2 + float32(dx*256),
		Y: b.Y + b.Height/2 + float32((py-cy)*256),
	}
}

func unprojectMercator(centerLat, centerLon, lonSpan float64, p rl.Vector2, b rl.Rectangle) (float64, float64) {
	z := mapZoom(lonSpan, b.Width)
	cx, cy := mercatorX(centerLon, z), mercatorY(centerLat, z)
	x := cx + float64(p.X-b.X-b.Width/2)/256
	y := cy + float64(p.Y-b.Y-b.Height/2)/256
	return mercatorLat(y, z), mercatorLon(x, z)
}

func mercatorLatSpan(centerLat, lonSpan float64, b rl.Rectangle) float64 {
	top, _ := unprojectMercator(centerLat, 0, lonSpan, rl.Vector2{X: b.X, Y: b.Y}, b)
	bottom, _ := unprojectMercator(centerLat, 0, lonSpan, rl.Vector2{X: b.X, Y: b.Y + b.Height}, b)
	span := top - bottom
	if span < 0.01 {
		return 0.01
	}
	return span
}

func clampMapView(centerLat, centerLon, lonSpan float64, b rl.Rectangle) (float64, float64, float64) {
	lonSpan = math.Max(0.002, math.Min(360, lonSpan))
	lat := math.Max(-mercatorMaxLat, math.Min(mercatorMaxLat, centerLat))
	top, _ := unprojectMercator(lat, 0, lonSpan, rl.Vector2{X: b.X, Y: b.Y}, b)
	bottom, _ := unprojectMercator(lat, 0, lonSpan, rl.Vector2{X: b.X, Y: b.Y + b.Height}, b)
	if top > mercatorMaxLat {
		lat -= top - mercatorMaxLat
	}
	if bottom < -mercatorMaxLat {
		lat += -mercatorMaxLat - bottom
	}
	lat = math.Max(-mercatorMaxLat, math.Min(mercatorMaxLat, lat))
	return lat, wrapLongitude(centerLon), lonSpan
}

func panMap(centerLat, centerLon, lonSpan float64, delta rl.Vector2, b rl.Rectangle) (float64, float64) {
	z := mapZoom(lonSpan, b.Width)
	lon := wrapLongitude(centerLon - float64(delta.X)/float64(b.Width)*lonSpan)
	lat := mercatorLat(mercatorY(centerLat, z)-float64(delta.Y)/256, z)
	return lat, lon
}

func zoomMap(centerLat, centerLon, lonSpan float64, anchor rl.Vector2, b rl.Rectangle, wheel float32) (float64, float64, float64) {
	lat, lon := unprojectMercator(centerLat, centerLon, lonSpan, anchor, b)
	lonSpan *= math.Pow(.78, float64(wheel))
	centerLat, centerLon, lonSpan = clampMapView(centerLat, centerLon, lonSpan, b)
	lat2, lon2 := unprojectMercator(centerLat, centerLon, lonSpan, anchor, b)
	z := mapZoom(lonSpan, b.Width)
	centerLat = mercatorLat(mercatorY(centerLat, z)+mercatorY(lat, z)-mercatorY(lat2, z), z)
	centerLon = wrapLongitude(centerLon + lon - lon2)
	return clampMapView(centerLat, centerLon, lonSpan, b)
}

func drawGeoMap(centerLat, centerLon, lonSpan float64, b rl.Rectangle) {
	rl.BeginScissorMode(int32(b.X), int32(b.Y), int32(b.Width), int32(b.Height))
	defer func() {
		_ = recover()
		rl.EndScissorMode()
	}()
	rl.DrawRectangleRec(b, rl.Color{R: 164, G: 200, B: 224, A: 255})
	drawWorldFallback(centerLat, centerLon, lonSpan, b)
	_ = drawMapTiles(centerLat, centerLon, lonSpan, b)
}

func drawWorldFallback(centerLat, centerLon, lonSpan float64, b rl.Rectangle) {
	if lonSpan < 24 {
		return
	}
	tex := worldMapTexture()
	if tex.ID == 0 || tex.Width <= 0 || tex.Height <= 0 {
		return
	}
	top, leftLon := unprojectMercator(centerLat, centerLon, lonSpan, rl.Vector2{X: b.X, Y: b.Y}, b)
	bottom, _ := unprojectMercator(centerLat, centerLon, lonSpan, rl.Vector2{X: b.X, Y: b.Y + b.Height}, b)
	src := rl.Rectangle{
		X:      float32((leftLon + 180) / 360 * float64(tex.Width)),
		Y:      float32((90 - top) / 180 * float64(tex.Height)),
		Width:  float32(lonSpan / 360 * float64(tex.Width)),
		Height: float32((top - bottom) / 180 * float64(tex.Height)),
	}
	src = clampTextureSrc(src, tex)
	if src.Width < 1 || src.Height < 1 {
		return
	}
	rl.DrawTexturePro(tex, src, b, rl.Vector2{}, 0, rl.White)
}

func clampTextureSrc(src rl.Rectangle, tex rl.Texture2D) rl.Rectangle {
	if src.Width < 0 {
		src.X += src.Width
		src.Width = -src.Width
	}
	if src.Height < 0 {
		src.Y += src.Height
		src.Height = -src.Height
	}
	if src.X < 0 {
		src.Width += src.X
		src.X = 0
	}
	if src.Y < 0 {
		src.Height += src.Y
		src.Y = 0
	}
	if src.X+src.Width > float32(tex.Width) {
		src.Width = float32(tex.Width) - src.X
	}
	if src.Y+src.Height > float32(tex.Height) {
		src.Height = float32(tex.Height) - src.Y
	}
	if src.Width < 1 || src.Height < 1 {
		return rl.Rectangle{}
	}
	return src
}

func drawMapGrid(centerLat, centerLon, lonSpan float64, b rl.Rectangle) {
	for i := 0; i <= 10; i++ {
		x := b.X + b.Width*float32(i)/10
		_, lon := unprojectMercator(centerLat, centerLon, lonSpan, rl.Vector2{X: x, Y: b.Y + b.Height/2}, b)
		rl.DrawLine(int32(x), int32(b.Y), int32(x), int32(b.Y+b.Height), rl.Color{R: 40, G: 70, B: 90, A: 50})
		simpleui.DrawText(fmt.Sprintf("%.2f°", lon), x+2, b.Y+b.Height-18, 9, colors.muted)
	}
	for i := 0; i <= 8; i++ {
		y := b.Y + b.Height*float32(i)/8
		lat, _ := unprojectMercator(centerLat, centerLon, lonSpan, rl.Vector2{X: b.X + b.Width/2, Y: y}, b)
		rl.DrawLine(int32(b.X), int32(y), int32(b.X+b.Width), int32(y), rl.Color{R: 40, G: 70, B: 90, A: 50})
		simpleui.DrawText(fmt.Sprintf("%.2f°", lat), b.X+3, y+2, 9, colors.muted)
	}
}
