package screens

import (
	"go-zero/internal/i18n"
	"go-zero/simpleui"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func drawGuidedControlCard(bounds rl.Rectangle, title, description string, accent rl.Color) {
	fill := mixColor(colors.panelAlt, accent, .09)
	border := mixColor(colors.border, accent, .42)
	rl.DrawRectangleRounded(bounds, .045, 7, fill)
	rl.DrawRectangleRoundedLinesEx(bounds, .045, 7, 1, border)
	rl.DrawRectangleRounded(rl.Rectangle{X: bounds.X + 10, Y: bounds.Y + 10, Width: 5, Height: 28}, .7, 5, accent)
	simpleui.DrawTextStyled(title, bounds.X+25, bounds.Y+8, 12, simpleui.FontSemiBold, accent)
	drawGuidedHelp(description, bounds.X+25, bounds.Y+29, bounds.Width-42)
}

// drawGuidedHelp keeps the explanatory copy inside a card even though the
// complete tool is horizontally compacted when the utilities sidebar opens.
// Two larger lines are more useful here than several tiny lines.
func drawGuidedHelp(value string, x, y, logicalWidth float32) {
	const size int32 = 11
	maxWidth := logicalWidth * simpleui.HorizontalDrawScale()
	words := strings.Fields(i18n.Display(value))
	lines := make([]string, 0, 2)
	for len(words) > 0 && len(lines) < 2 {
		line := words[0]
		words = words[1:]
		for len(words) > 0 {
			candidate := line + " " + words[0]
			if simpleui.MeasureText(candidate, size).X > maxWidth {
				break
			}
			line = candidate
			words = words[1:]
		}
		lines = append(lines, line)
	}
	if len(words) > 0 && len(lines) > 0 {
		last := lines[len(lines)-1]
		for simpleui.MeasureText(last+"…", size).X > maxWidth && len(last) > 1 {
			last = strings.TrimSpace(last[:len(last)-1])
		}
		lines[len(lines)-1] = last + "…"
	}
	color := mixColor(colors.muted, colors.text, .38)
	for index, line := range lines {
		simpleui.DrawText(line, x, y+float32(index*14), size, color)
	}
}

func drawGuidedHeader(title, introduction string) {
	const (
		x         = float32(40)
		titleSize = int32(16)
		introSize = int32(12)
		gap       = float32(22)
	)
	scale := simpleui.HorizontalDrawScale()
	introX := x + (simpleui.MeasureTextStyled(title, titleSize, simpleui.FontSemiBold).X+gap)/scale
	simpleui.DrawTextStyled(title, x, toolY+9, titleSize, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText(introduction, introX, toolY+12, introSize, mixColor(colors.muted, colors.text, .42))
}

func (screen *MainScreen) drawWaterfallAdjustPanel() {
	drawGuidedHeader(i18n.Source("text.waterfallGuideTitle"), i18n.Source("text.waterfallGuideIntro"))
	drawGuidedControlCard(rl.Rectangle{X: 40, Y: 665, Width: 350, Height: 210}, i18n.Source("text.waterfallBrightnessTitle"), i18n.Source("text.waterfallBrightnessHelp"), colors.cyan)
	drawGuidedControlCard(rl.Rectangle{X: 405, Y: 665, Width: 350, Height: 210}, i18n.Source("text.waterfallContrastTitle"), i18n.Source("text.waterfallContrastHelp"), colors.orange)
	drawGuidedControlCard(rl.Rectangle{X: 770, Y: 665, Width: 400, Height: 210}, i18n.Source("text.waterfallRangeTitle"), i18n.Source("text.waterfallRangeHelp"), colors.green)
	drawGuidedControlCard(rl.Rectangle{X: 1185, Y: 665, Width: 350, Height: 210}, i18n.Source("text.waterfallMotionTitle"), i18n.Source("text.waterfallMotionHelp"), colors.blue)

	// A subdued preview makes the meaning of the range control visible before
	// the user needs to understand dBm values.
	for index := 0; index < 24; index++ {
		fraction := float32(index) / 23
		color := mixColor(colors.background, colors.green, fraction*.75)
		rl.DrawRectangle(805+int32(index)*13, 812, 13, 8, color)
	}
	simpleui.DrawText(i18n.Source("text.waterfallRangeWeak"), 805, 827, 9, colors.muted)
	simpleui.DrawText(i18n.Source("text.waterfallRangeStrong"), 1075, 827, 9, colors.muted)
}
