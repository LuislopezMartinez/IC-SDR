package screens

import (
	"go-zero/internal/i18n"

	"strings"

	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	themeDark  = "DARK"
	themeLight = "LIGHT"
	themeBlue  = "BLUE"
)

var lightPalette = uiPalette{
	background: rl.Color{R: 232, G: 242, B: 250, A: 255},
	panel:      rl.Color{R: 248, G: 252, B: 255, A: 255},
	panelAlt:   rl.Color{R: 237, G: 246, B: 253, A: 255},
	border:     rl.Color{R: 180, G: 207, B: 228, A: 255},
	grid:       rl.Color{R: 194, G: 216, B: 232, A: 255},
	cyan:       rl.Color{R: 0, G: 153, B: 211, A: 255},
	blue:       rl.Color{R: 31, G: 126, B: 214, A: 255},
	green:      rl.Color{R: 32, G: 180, B: 95, A: 255},
	orange:     rl.Color{R: 232, G: 137, B: 0, A: 255},
	red:        rl.Color{R: 214, G: 48, B: 55, A: 255},
	text:       rl.Color{R: 22, G: 48, B: 82, A: 255},
	muted:      rl.Color{R: 84, G: 111, B: 140, A: 255},
}

var bluePalette = uiPalette{
	background: rl.Color{R: 3, G: 49, B: 82, A: 255},
	panel:      rl.Color{R: 5, G: 67, B: 111, A: 255},
	panelAlt:   rl.Color{R: 7, G: 78, B: 128, A: 255},
	border:     rl.Color{R: 27, G: 143, B: 204, A: 255},
	grid:       rl.Color{R: 22, G: 105, B: 161, A: 255},
	cyan:       rl.Color{R: 29, G: 218, B: 241, A: 255},
	blue:       rl.Color{R: 39, G: 151, B: 235, A: 255},
	green:      rl.Color{R: 48, G: 215, B: 132, A: 255},
	orange:     rl.Color{R: 255, G: 177, B: 25, A: 255},
	red:        rl.Color{R: 245, G: 72, B: 78, A: 255},
	text:       rl.Color{R: 237, G: 248, B: 255, A: 255},
	muted:      rl.Color{R: 164, G: 204, B: 229, A: 255},
}

func validTheme(name string) bool {
	switch strings.ToUpper(name) {
	case themeDark, themeLight, themeBlue:
		return true
	}
	return false
}

func themeDisplayName(name string) string {
	switch strings.ToUpper(name) {
	case themeLight:
		return i18n.Source("text.33f29205b18a")
	case themeBlue:
		return i18n.Source("text.dcaa3316dde8")
	default:
		return i18n.Source("text.4f88622f704e")
	}
}

func paletteForTheme(name string) uiPalette {
	switch strings.ToUpper(name) {
	case themeLight:
		return lightPalette
	case themeBlue:
		return bluePalette
	default:
		return darkPalette
	}
}

func (screen *MainScreen) cycleTheme() {
	switch screen.themeName {
	case themeDark:
		screen.applyTheme(themeLight)
	case themeLight:
		screen.applyTheme(themeBlue)
	default:
		screen.applyTheme(themeDark)
	}
	screen.markSettingsDirty()
}

func (screen *MainScreen) applyTheme(name string) {
	name = strings.ToUpper(name)
	if !validTheme(name) {
		name = themeDark
	}
	screen.themeName = name
	colors = paletteForTheme(name)
	simpleui.SetTheme(simpleUITheme(colors, name == themeLight))
	simpleui.SetColorTransform(func(color rl.Color) rl.Color {
		return remapThemeColor(color, colors)
	})
	if screen.themeButton != nil {
		screen.themeButton.SetLabel(i18n.Source("text.193f56324ce6"))
	}
}

func simpleUITheme(p uiPalette, light bool) simpleui.Theme {
	controlHover := mixColor(p.panelAlt, p.cyan, 0.13)
	controlPressed := mixColor(p.panelAlt, p.blue, 0.32)
	disabled := mixColor(p.panelAlt, p.muted, 0.20)
	track := mixColor(p.panelAlt, p.muted, 0.34)
	input := mixColor(p.panel, p.background, 0.50)
	if light {
		input = rl.Color{R: 255, G: 255, B: 255, A: 255}
	}
	return simpleui.Theme{
		Text: p.text, TextMuted: p.muted, Accent: p.cyan,
		Control: p.panelAlt, ControlHover: controlHover, ControlPressed: controlPressed,
		ControlDisabled: disabled, Border: p.border, IndicatorOff: track,
		SliderTrack: track, SliderSelection: p.cyan, SliderHandle: mixColor(p.text, p.panel, 0.15),
		DisabledTrack: disabled, DisabledHandle: p.muted, DisabledPattern: withAlpha(p.muted, 160),
		SwitchOff: track, SwitchOn: p.green,
		InputBackground: input, InputSelection: withAlpha(p.blue, 180), InputCaret: p.text,
		PopupBackground: p.panel, PopupHover: controlHover,
		CornerRadius: 0.12, BorderWidth: 1,
	}
}

func remapThemeColor(color rl.Color, target uiPalette) rl.Color {
	pairs := [][2]rl.Color{
		{darkPalette.background, target.background}, {darkPalette.panel, target.panel},
		{darkPalette.panelAlt, target.panelAlt}, {darkPalette.border, target.border},
		{darkPalette.grid, target.grid}, {darkPalette.cyan, target.cyan},
		{darkPalette.blue, target.blue}, {darkPalette.green, target.green},
		{darkPalette.orange, target.orange}, {darkPalette.red, target.red},
		{darkPalette.text, target.text}, {darkPalette.muted, target.muted},
	}
	for _, pair := range pairs {
		if sameRGB(color, pair[0]) {
			result := pair[1]
			result.A = color.A
			return result
		}
	}
	return color
}

func sameRGB(a, b rl.Color) bool { return a.R == b.R && a.G == b.G && a.B == b.B }

func withAlpha(color rl.Color, alpha uint8) rl.Color { color.A = alpha; return color }

func mixColor(a, b rl.Color, amount float32) rl.Color {
	mix := func(x, y uint8) uint8 { return uint8(float32(x) + (float32(y)-float32(x))*amount) }
	return rl.Color{R: mix(a.R, b.R), G: mix(a.G, b.G), B: mix(a.B, b.B), A: 255}
}

func tableSelectionColors() (rl.Color, rl.Color) {
	background := mixColor(colors.blue, rl.Black, .35)
	return background, simpleui.EnsureTextContrast(colors.text, background)
}
