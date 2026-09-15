package simpleui

import (
	"go-zero/internal/i18n"

	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type Orientation uint8

const (
	Horizontal Orientation = iota
	Vertical
)

func normalizeRange(minimum, maximum, step float32) (float32, float32, float32) {
	if maximum <= minimum {
		panic(i18n.Source("text.644767ec8f1f"))
	}
	if step <= 0 {
		panic(i18n.Source("text.8ad45dc83078"))
	}
	return minimum, maximum, step
}

func quantize(value, minimum, maximum, step float32) float32 {
	value = max(minimum, min(maximum, value))
	steps := math.Round(float64((value - minimum) / step))
	return max(minimum, min(maximum, minimum+float32(steps)*step))
}

func sliderGeometry(bounds rl.Rectangle, orientation Orientation) (start, end rl.Vector2, radius float32) {
	if orientation == Vertical {
		radius = min(bounds.Width*0.42, 12)
		start = rl.Vector2{X: bounds.X + bounds.Width*0.5, Y: bounds.Y + bounds.Height - radius}
		end = rl.Vector2{X: start.X, Y: bounds.Y + radius}
		return
	}
	radius = min(bounds.Height*0.42, 12)
	start = rl.Vector2{X: bounds.X + radius, Y: bounds.Y + bounds.Height*0.5}
	end = rl.Vector2{X: bounds.X + bounds.Width - radius, Y: start.Y}
	return
}

func sliderPosition(bounds rl.Rectangle, orientation Orientation, value, minimum, maximum float32) rl.Vector2 {
	start, end, _ := sliderGeometry(bounds, orientation)
	ratio := (value - minimum) / (maximum - minimum)
	return rl.Vector2{X: start.X + (end.X-start.X)*ratio, Y: start.Y + (end.Y-start.Y)*ratio}
}

func sliderValue(bounds rl.Rectangle, orientation Orientation, pointer rl.Vector2, minimum, maximum float32) float32 {
	start, end, _ := sliderGeometry(bounds, orientation)
	var ratio float32
	if orientation == Vertical {
		ratio = (pointer.Y - start.Y) / (end.Y - start.Y)
	} else {
		ratio = (pointer.X - start.X) / (end.X - start.X)
	}
	return minimum + max(0, min(1, ratio))*(maximum-minimum)
}

func drawSliderTrack(bounds rl.Rectangle, orientation Orientation, selectedStart, selectedEnd rl.Vector2, enabled bool) {
	start, end, _ := sliderGeometry(bounds, orientation)
	theme := currentTheme
	trackColor := theme.SliderTrack
	selectionColor := theme.SliderSelection
	if !enabled {
		trackColor = theme.DisabledTrack
		selectionColor = theme.DisabledTrack
	}
	rl.DrawLineEx(start, end, 7, trackColor)
	rl.DrawLineEx(selectedStart, selectedEnd, 7, selectionColor)
	if !enabled {
		drawDisabledPattern(bounds, orientation)
	}
}

func drawSliderHandle(position rl.Vector2, radius float32, active, enabled bool) {
	color := currentTheme.SliderHandle
	if !enabled {
		color = currentTheme.DisabledHandle
	} else if active {
		color = currentTheme.Accent
	}
	rl.DrawCircleV(position, radius, color)
	rl.DrawCircleLines(int32(position.X), int32(position.Y), radius, currentTheme.Border)
}

func drawDisabledPattern(bounds rl.Rectangle, orientation Orientation) {
	start, end, _ := sliderGeometry(bounds, orientation)
	color := currentTheme.DisabledPattern
	phase := float32(math.Mod(rl.GetTime()*12, 9))
	if orientation == Vertical {
		for y := end.Y - phase; y <= start.Y; y += 9 {
			rl.DrawLineEx(rl.Vector2{X: start.X - 4, Y: y + 4}, rl.Vector2{X: start.X + 4, Y: y - 4}, 2, color)
		}
		return
	}
	for x := start.X - phase; x <= end.X; x += 9 {
		rl.DrawLineEx(rl.Vector2{X: x - 4, Y: start.Y + 4}, rl.Vector2{X: x + 4, Y: start.Y - 4}, 2, color)
	}
}
