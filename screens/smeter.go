package screens

import (
	"fmt"
	"math"

	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type SMeter struct {
	displayed, peak       float32
	initialized           bool
	lastUpdate, peakUntil float64
}

func calibrateSMeter(rawSpectrumDB float32) float32 {
	rawDBm := rawSpectrumDB - 53
	weakCorrection := min(max(float32(-73)-rawDBm, 0), 24)
	return min(max(rawDBm-weakCorrection, -130), -13)
}

func (meter *SMeter) Update(rawSpectrumDB float32) {
	now := rl.GetTime()
	target := calibrateSMeter(rawSpectrumDB)
	if !meter.initialized {
		meter.displayed, meter.peak = target, target
		meter.initialized = true
		meter.lastUpdate = now
		return
	}
	delta := max(now-meter.lastUpdate, 0)
	tau := .38
	if target > meter.displayed {
		tau = .055
	}
	alpha := float32(1 - math.Exp(-delta/tau))
	meter.displayed += alpha * (target - meter.displayed)
	if meter.displayed >= meter.peak {
		meter.peak = meter.displayed
		meter.peakUntil = now + .75
	} else if now > meter.peakUntil {
		meter.peak -= float32(delta * 18)
		if meter.peak < meter.displayed {
			meter.peak = meter.displayed
		}
	}
	meter.lastUpdate = now
}

func (meter *SMeter) Draw(x, y, w, h float32) {
	db := int(math.Round(float64(min(max(meter.displayed, -130), -53))))
	drawPanel(x, y, w, h)
	left, right := x+17, x+w-15
	scaleY, barY := y+35, y+58
	labels := []struct {
		text  string
		db    int
		color rl.Color
	}{{"S1", -121, colors.text}, {"S3", -109, colors.text}, {"S5", -97, colors.text}, {"S7", -85, colors.text}, {"S9", -73, colors.text}, {"+10", -63, colors.orange}, {"+20", -53, colors.red}}
	for _, label := range labels {
		px := meterX(label.db, left, right)
		tw := simpleui.MeasureText(label.text, 10).X
		simpleui.DrawText(label.text, px-tw/2, y+8, 10, label.color)
	}
	for s := 1; s <= 9; s++ {
		px := meterX(-121+(s-1)*6, left, right)
		length := float32(6)
		if s == 1 || s == 5 || s == 9 {
			length = 10
		}
		rl.DrawLineEx(rl.Vector2{X: px, Y: scaleY}, rl.Vector2{X: px, Y: scaleY + length}, 2, colors.text)
	}
	for plus := 10; plus <= 20; plus += 10 {
		px := meterX(-73+plus, left, right)
		length := float32(6)
		if plus == 20 {
			length = 10
		}
		rl.DrawLineEx(rl.Vector2{X: px, Y: scaleY}, rl.Vector2{X: px, Y: scaleY + length}, 2, colors.red)
	}
	rl.DrawRectangleRounded(rl.Rectangle{X: left, Y: barY, Width: right - left, Height: 13}, .15, 3, meterTrackColor())
	level := min(max(float32(db+121)/68, 0), 1)
	segments := 32
	gap := float32(2)
	segmentW := (right - left - gap*float32(segments-1)) / float32(segments)
	lit := int(math.Round(float64(level * float32(segments))))
	for i := 0; i < lit; i++ {
		fraction := float32(i) / 31
		color := colors.cyan
		if fraction >= 48.0/68 {
			color = colors.orange
		}
		if fraction >= 58.0/68 {
			color = colors.red
		}
		sx := left + float32(i)*(segmentW+gap)
		rl.DrawRectangleRounded(rl.Rectangle{X: sx, Y: barY, Width: segmentW, Height: 13}, .15, 2, color)
	}
	peakX := meterX(int(math.Round(float64(meter.peak))), left, right)
	peakColor := colors.text
	peakColor.A = 190
	rl.DrawLineEx(rl.Vector2{X: peakX, Y: barY - 2}, rl.Vector2{X: peakX, Y: barY + 15}, 1, peakColor)
	text := fmt.Sprintf("%s   %d dBm", sMeterLabel(db), db)
	tw := simpleui.MeasureTextStyled(text, 12, simpleui.FontMono).X
	simpleui.DrawTextStyled(text, right-tw, y+h-22, 12, simpleui.FontMono, colors.text)
}

func meterX(db int, left, right float32) float32 {
	return left + min(max(float32(db+121)/68, 0), 1)*(right-left)
}
func sMeterLabel(db int) string {
	if db < -121 {
		return "S0"
	}
	if db <= -73 {
		return fmt.Sprintf("S%d", min(max((db+121)/6+1, 1), 9))
	}
	return fmt.Sprintf("S9+%d", db+73)
}
