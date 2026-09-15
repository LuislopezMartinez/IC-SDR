package screens

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/internal/i18n"
	"go-zero/simpleui"
)

func languageButtonBounds() rl.Rectangle { return rl.Rectangle{X: 820, Y: 63, Width: 250, Height: 42} }
func languageRowBounds(i int) rl.Rectangle {
	return rl.Rectangle{X: 820, Y: 115 + float32(i)*48, Width: 250, Height: 42}
}
func (menu *ToolMenu) languageInput(input simpleui.Input) bool {
	if menu.languageOpen && rl.IsKeyPressed(rl.KeyEscape) {
		menu.languageOpen = false
		return true
	}
	if rl.CheckCollisionPointRec(input.Pointer, languageButtonBounds()) {
		if input.Pressed {
			menu.languagePressed = true
		}
		if input.Released {
			if menu.languagePressed {
				menu.languageOpen = !menu.languageOpen
			}
			menu.languagePressed = false
		}
		return true
	}
	menu.languagePressed = false
	if !menu.languageOpen {
		return false
	}
	langs := i18n.Languages()
	menu.languageOffset = min(max(menu.languageOffset-int(rl.GetMouseWheelMove()), 0), max(len(langs)-12, 0))
	for i, l := range langs {
		if i < menu.languageOffset || i >= menu.languageOffset+12 {
			continue
		}
		if input.Released && rl.CheckCollisionPointRec(input.Pointer, languageRowBounds(i-menu.languageOffset)) {
			_ = i18n.Select(l.ID)
			menu.languageOpen = false
			simpleui.PlayActivationFeedback()
		}
	}
	if input.Pressed && !rl.CheckCollisionPointRec(input.Pointer, rl.Rectangle{X: 810, Y: 110, Width: 270, Height: float32(min(len(langs), 12))*48 + 10}) {
		menu.languageOpen = false
	}
	return true
}
func (menu *ToolMenu) drawLanguages() {
	b := languageButtonBounds()
	rl.DrawRectangleRounded(b, .1, 4, colors.blue)
	langs := i18n.Languages()
	for _, l := range langs {
		if l.ID == i18n.Current() {
			drawLanguageFlag(l.Flag, rl.Rectangle{X: b.X + 10, Y: b.Y + 10, Width: 34, Height: 22})
			simpleui.DrawText(l.Name, b.X+55, b.Y+11, 15, colors.text)
		}
	}
	simpleui.DrawText("v", b.X+b.Width-25, b.Y+11, 15, colors.text)
	if !menu.languageOpen {
		return
	}
	drawPanel(810, 110, 270, float32(min(len(langs), 12))*48+10)
	for i, l := range langs {
		if i < menu.languageOffset || i >= menu.languageOffset+12 {
			continue
		}
		b := languageRowBounds(i - menu.languageOffset)
		fill := colors.panelAlt
		if l.ID == i18n.Current() {
			fill = colors.blue
		}
		rl.DrawRectangleRounded(b, .1, 4, fill)
		drawLanguageFlag(l.Flag, rl.Rectangle{X: b.X + 10, Y: b.Y + 10, Width: 34, Height: 22})
		simpleui.DrawText(l.Name, b.X+55, b.Y+10, 15, colors.text)
	}
}
func drawLanguageFlag(flag string, b rl.Rectangle) {
	switch flag {
	case "ES":
		rl.DrawRectangleRec(b, rl.Color{R: 180, G: 20, B: 35, A: 255})
		rl.DrawRectangleRec(rl.Rectangle{X: b.X, Y: b.Y + b.Height/4, Width: b.Width, Height: b.Height / 2}, rl.Gold)
	case "GB":
		rl.DrawRectangleRec(b, rl.Color{R: 20, G: 35, B: 100, A: 255})
		rl.DrawLineEx(rl.Vector2{X: b.X, Y: b.Y}, rl.Vector2{X: b.X + b.Width, Y: b.Y + b.Height}, 5, rl.White)
		rl.DrawLineEx(rl.Vector2{X: b.X + b.Width, Y: b.Y}, rl.Vector2{X: b.X, Y: b.Y + b.Height}, 5, rl.White)
		rl.DrawRectangleRec(rl.Rectangle{X: b.X, Y: b.Y + 8, Width: b.Width, Height: 6}, rl.White)
		rl.DrawRectangleRec(rl.Rectangle{X: b.X + 13, Y: b.Y, Width: 8, Height: b.Height}, rl.White)
		rl.DrawRectangleRec(rl.Rectangle{X: b.X, Y: b.Y + 10, Width: b.Width, Height: 3}, rl.Red)
		rl.DrawRectangleRec(rl.Rectangle{X: b.X + 15, Y: b.Y, Width: 4, Height: b.Height}, rl.Red)
	default:
		rl.DrawRectangleLinesEx(b, 1, colors.text)
		simpleui.DrawText(flag, b.X, b.Y, 10, colors.text)
	}
}
