package screens

import (
	"testing"

	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestValidTheme(t *testing.T) {
	for _, name := range []string{themeDark, themeLight, themeBlue, "dark", "Light"} {
		if !validTheme(name) {
			t.Fatalf("expected %q to be a valid theme", name)
		}
	}
	if validTheme("PURPLE") || validTheme("") {
		t.Fatal("unexpected theme accepted")
	}
}

func TestSemanticActionColorsFollowEveryPalette(t *testing.T) {
	palettes := []uiPalette{darkPalette, lightPalette, bluePalette}
	for _, palette := range palettes {
		actions := semanticActionColors(palette)
		for name, pair := range map[string][2]rl.Color{
			"start":  {actions.start, palette.text},
			"stop":   {actions.stop, palette.text},
			"clear":  {actions.clear, palette.text},
			"export": {actions.export, palette.text},
		} {
			if got := simpleui.EnsureTextContrast(pair[1], pair[0]); got.A == 0 {
				t.Fatalf("%s action has no readable text color", name)
			}
		}
		if actions.start == actions.stop || actions.stop == actions.clear {
			t.Fatal("semantic actions must remain visually distinct")
		}
	}
	lightActions := semanticActionColors(lightPalette)
	if got := remapThemeColor(actionStartFill, lightPalette); got != lightActions.start {
		t.Fatalf("start action was not remapped for light theme: got %#v want %#v", got, lightActions.start)
	}
	if got := remapThemeColor(actionClearFill, bluePalette); got != semanticActionColors(bluePalette).clear {
		t.Fatalf("clear action was not remapped for blue theme: got %#v", got)
	}
}

func TestTypographyScaleIsOrderedAndReadable(t *testing.T) {
	if !(uiCaptionFontSize < uiBodyFontSize &&
		uiBodyFontSize < uiMinimumFontSize &&
		uiMinimumFontSize < uiControlFontSize &&
		uiControlFontSize < uiSectionFontSize &&
		uiSectionFontSize < uiTitleFontSize) {
		t.Fatal("typography tokens must form a strictly increasing scale")
	}
	if uiCaptionFontSize < 11 {
		t.Fatal("caption text is below the readable baseline")
	}
}

func TestApplyThemeUpdatesPaletteAndControls(t *testing.T) {
	screen := &MainScreen{themeName: themeDark}
	t.Cleanup(func() { screen.applyTheme(themeDark) })

	screen.applyTheme(themeLight)
	if screen.themeName != themeLight || colors != lightPalette {
		t.Fatal("light palette was not applied")
	}
	if simpleui.CurrentTheme().Text != lightPalette.text {
		t.Fatal("SimpleUI did not receive the light palette")
	}

	screen.applyTheme(themeBlue)
	if screen.themeName != themeBlue || colors != bluePalette {
		t.Fatal("blue palette was not applied")
	}
}

func TestThemeCycleOrder(t *testing.T) {
	screen := &MainScreen{themeName: themeDark}
	t.Cleanup(func() { screen.applyTheme(themeDark) })

	screen.cycleTheme()
	if screen.themeName != themeLight {
		t.Fatalf("dark should advance to light, got %q", screen.themeName)
	}
	screen.cycleTheme()
	if screen.themeName != themeBlue {
		t.Fatalf("light should advance to blue, got %q", screen.themeName)
	}
	screen.cycleTheme()
	if screen.themeName != themeDark {
		t.Fatalf("blue should advance to dark, got %q", screen.themeName)
	}
}
