package screens

import (
	"go-zero/internal/bandplan"
	"go-zero/internal/i18n"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type LocaleSettings struct {
	simpleui.BaseElement
	screen                         *MainScreen
	open                           bool
	language, region, country      string
	pressedLang, pressedRegion     int
	pressedCountry                 int
	apply, cancel                  *simpleui.Button
}

func NewLocaleSettings(screen *MainScreen) *LocaleSettings {
	overlay := &LocaleSettings{
		BaseElement:   simpleui.NewBaseElement("localeSettingsOverlay", 0, 0, designWidth, designHeight),
		screen:        screen,
		pressedLang:   -1,
		pressedRegion: -1,
		pressedCountry: -1,
	}
	overlay.cancel = simpleui.NewButton("localeCancel", 560, 812, 220, 46, T("CANCEL"), 16)
	overlay.apply = simpleui.NewButton("localeApply", 820, 812, 220, 46, T("APPLY"), 16)
	overlay.cancel.OnClick(overlay.Close)
	overlay.apply.OnClick(func() {
		overlay.screen.setLocale(overlay.language, overlay.region, overlay.country)
		overlay.Close()
	})
	return overlay
}

func (overlay *LocaleSettings) Open() {
	overlay.language = i18n.Normalize(overlay.screen.language)
	overlay.region = bandplan.Normalize(overlay.screen.ituRegion)
	overlay.country = overlay.screen.country
	if overlay.country == "" {
		overlay.country = "auto"
	}
	overlay.open = true
}

func (overlay *LocaleSettings) Close() {
	overlay.open = false
	overlay.pressedLang, overlay.pressedRegion, overlay.pressedCountry = -1, -1, -1
}

func (overlay *LocaleSettings) OverlayOpen() bool          { return overlay.open }
func (overlay *LocaleSettings) Update(simpleui.Input) bool { return false }
func (overlay *LocaleSettings) Draw()                      {}

func (overlay *LocaleSettings) UpdateOverlay(input simpleui.Input) bool {
	if !overlay.open {
		return false
	}
	if rl.IsKeyPressed(rl.KeyEscape) {
		overlay.Close()
		return true
	}
	overlay.cancel.SetLabel(T("CANCEL"))
	overlay.apply.SetLabel(T("APPLY"))
	if overlay.cancel.Update(input) || overlay.apply.Update(input) {
		return true
	}
	if input.Pressed {
		overlay.pressedLang = overlay.languageAt(input.Pointer)
		overlay.pressedRegion = overlay.regionAt(input.Pointer)
		overlay.pressedCountry = overlay.countryAt(input.Pointer)
	}
	if input.Released {
		if index := overlay.languageAt(input.Pointer); index >= 0 && index == overlay.pressedLang {
			overlay.language = i18n.Languages()[index].Code
		}
		if index := overlay.regionAt(input.Pointer); index >= 0 && index == overlay.pressedRegion {
			overlay.region = localeRegions[index].id
			if !countryFitsRegion(overlay.country, overlay.region) {
				overlay.country = "auto"
			}
		}
		if index := overlay.countryAt(input.Pointer); index >= 0 && index == overlay.pressedCountry {
			overlay.country = overlay.countryIDs()[index]
		}
		overlay.pressedLang, overlay.pressedRegion, overlay.pressedCountry = -1, -1, -1
	}
	return true
}

func (overlay *LocaleSettings) DrawOverlay() {
	if !overlay.open {
		return
	}
	rl.DrawRectangle(0, 0, int32(designWidth), int32(designHeight), rl.Color{A: 218})
	panel := rl.Rectangle{X: 70, Y: 28, Width: 1460, Height: 844}
	rl.DrawRectangleRounded(panel, .018, 8, colors.panel)
	rl.DrawRectangleRoundedLinesEx(panel, .018, 8, 2, colors.border)
	rl.DrawRectangle(72, 30, 1456, 52, colors.panelAlt)
	simpleui.DrawTextStyled(T("LANGUAGE AND REGION"), 92, 42, 24, simpleui.FontSemiBold, colors.text)
	simpleui.DrawText(T("Region is applied every time the app starts."), 720, 48, 14, colors.muted)

	simpleui.DrawTextStyled(T("LANGUAGE"), 92, 98, 14, simpleui.FontSemiBold, colors.cyan)
	for index, language := range i18n.Languages() {
		bounds := overlay.languageBounds(index)
		fill := colors.panelAlt
		if language.Code == overlay.language {
			fill = colors.blue
		} else if index == overlay.pressedLang {
			fill = mixColor(colors.panelAlt, colors.cyan, .18)
		}
		rl.DrawRectangleRounded(bounds, .12, 8, fill)
		rl.DrawRectangleRoundedLinesEx(bounds, .12, 8, 1.5, colors.border)
		label := language.Native
		if language.Native != language.Name {
			label = language.Native + "  ·  " + language.Name
		}
		drawCentered(label, bounds, 13, simpleui.EnsureTextContrast(colors.text, fill))
	}

	simpleui.DrawTextStyled(T("ITU REGION"), 92, 500, 14, simpleui.FontSemiBold, colors.cyan)
	for index, region := range localeRegions {
		bounds := overlay.regionBounds(index)
		fill := colors.panelAlt
		if region.id == overlay.region {
			fill = colors.blue
		} else if index == overlay.pressedRegion {
			fill = mixColor(colors.panelAlt, colors.cyan, .18)
		}
		rl.DrawRectangleRounded(bounds, .12, 8, fill)
		rl.DrawRectangleRoundedLinesEx(bounds, .12, 8, 1.5, colors.border)
		drawCentered(T(region.label), bounds, 14, simpleui.EnsureTextContrast(colors.text, fill))
	}

	simpleui.DrawTextStyled(T("COUNTRY"), 92, 580, 14, simpleui.FontSemiBold, colors.orange)
	for index, id := range overlay.countryIDs() {
		bounds := overlay.countryBounds(index)
		fill := colors.panelAlt
		if id == overlay.country {
			fill = mixColor(colors.panelAlt, colors.orange, .35)
		} else if index == overlay.pressedCountry {
			fill = mixColor(colors.panelAlt, colors.orange, .18)
		}
		rl.DrawRectangleRounded(bounds, .14, 8, fill)
		rl.DrawRectangleRoundedLinesEx(bounds, .14, 8, 1.5, colors.border)
		drawCentered(overlay.countryLabel(id), bounds, 12, simpleui.EnsureTextContrast(colors.text, fill))
	}

	overlay.cancel.SetLabel(T("CANCEL"))
	overlay.apply.SetLabel(T("APPLY"))
	overlay.cancel.Draw()
	overlay.apply.Draw()
}

var localeRegions = []struct{ id, label string }{
	{"itu-r1", "ITU Region 1 · Europe / Africa / Middle East"},
	{"itu-r2", "ITU Region 2 · Americas"},
	{"itu-r3", "ITU Region 3 · Asia / Pacific"},
}

func (overlay *LocaleSettings) countryIDs() []string {
	ids := []string{"auto"}
	for _, plan := range bandplan.CountriesFor(overlay.region) {
		ids = append(ids, plan.ID)
	}
	return ids
}

func (overlay *LocaleSettings) countryLabel(id string) string {
	if id == "auto" {
		return T("Automatic (best for this region)")
	}
	if plan, ok := bandplan.Get(id); ok {
		return plan.Name
	}
	return id
}

func countryFitsRegion(country, region string) bool {
	if country == "" || country == "auto" {
		return true
	}
	for _, plan := range bandplan.CountriesFor(region) {
		if plan.ID == country {
			return true
		}
	}
	return false
}

func (overlay *LocaleSettings) languageBounds(index int) rl.Rectangle {
	col, row := index%4, index/4
	return rl.Rectangle{X: 92 + float32(col)*355, Y: 122 + float32(row)*36, Width: 342, Height: 32}
}

func (overlay *LocaleSettings) regionBounds(index int) rl.Rectangle {
	return rl.Rectangle{X: 92 + float32(index)*470, Y: 522, Width: 452, Height: 48}
}

func (overlay *LocaleSettings) countryBounds(index int) rl.Rectangle {
	col, row := index%7, index/7
	return rl.Rectangle{X: 92 + float32(col)*202, Y: 604 + float32(row)*28, Width: 194, Height: 26}
}

func (overlay *LocaleSettings) languageAt(point rl.Vector2) int {
	for index := range i18n.Languages() {
		if rl.CheckCollisionPointRec(point, overlay.languageBounds(index)) {
			return index
		}
	}
	return -1
}

func (overlay *LocaleSettings) regionAt(point rl.Vector2) int {
	for index := range localeRegions {
		if rl.CheckCollisionPointRec(point, overlay.regionBounds(index)) {
			return index
		}
	}
	return -1
}

func (overlay *LocaleSettings) countryAt(point rl.Vector2) int {
	for index := range overlay.countryIDs() {
		if rl.CheckCollisionPointRec(point, overlay.countryBounds(index)) {
			return index
		}
	}
	return -1
}
