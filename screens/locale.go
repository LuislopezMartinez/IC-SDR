package screens

import (
	"fmt"

	"go-zero/internal/bandplan"
	"go-zero/internal/i18n"
	"go-zero/internal/locale"
)

var localeOverride string

func currentLocaleTag() string {
	if localeOverride != "" {
		return localeOverride
	}
	return locale.SystemTag()
}

var activeBandPlan = bandplan.Must("itu-r1")

func init() {
	applyBandPlan(activeBandPlan)
}

func T(message string, args ...any) string { return i18n.T(message, args...) }

func applyBandPlan(plan bandplan.Plan) {
	activeBandPlan = plan
	bandPlan = make([]bandRange, len(plan.Ranges))
	for i, item := range plan.Ranges {
		bandPlan[i] = bandRange{category: item.Category, name: item.Name, minimumHz: item.MinHz, maximumHz: item.MaxHz}
	}
	next := map[string][]BandDefinition{}
	for category, list := range plan.Selector {
		bands := make([]BandDefinition, 0, len(list))
		for _, item := range list {
			bands = append(bands, BandDefinition{Category: item.Category, Name: item.Name, FrequencyHz: item.FrequencyHz, SpanHz: item.SpanHz})
		}
		next[category] = bands
	}
	bandsByCategory = next
}

func (screen *MainScreen) applyLocaleAndRegion() {
	detected := screen.language == "" || screen.ituRegion == ""
	if detected {
		language, region, country := bandplan.FromLocale(currentLocaleTag())
		if screen.language == "" {
			screen.language = i18n.Normalize(language)
		}
		if screen.ituRegion == "" {
			screen.ituRegion = region
		}
		if screen.country == "" {
			screen.country = country
		}
	}
	screen.language = i18n.Normalize(screen.language)
	if screen.ituRegion == "" {
		screen.ituRegion = "itu-r1"
	}
	if screen.country == "" {
		screen.country = "auto"
	}
	i18n.SetLanguage(screen.language)
	applyBandPlan(bandplan.Must(bandplan.Resolve(screen.ituRegion, screen.country)))
	if screen.frequencyHz >= 1_000 {
		screen.updateBandForFrequency(screen.frequencyHz)
	}
	if !validBand(screen.bandCategory, screen.bandName) {
		def := activeBandPlan.Default
		screen.bandCategory, screen.bandName = def.Category, def.Name
	}
	if screen.satellitePanel != nil {
		screen.satellitePanel.resolveStation()
		screen.satellitePanel.syncStationFields()
	}
	if detected {
		screen.markSettingsDirty()
	}
}

func (screen *MainScreen) setLocale(language, region, country string) {
	screen.language = i18n.Normalize(language)
	screen.ituRegion = bandplan.Normalize(region)
	screen.country = country
	if screen.country == "" {
		screen.country = "auto"
	}
	i18n.SetLanguage(screen.language)
	applyBandPlan(bandplan.Must(bandplan.Resolve(screen.ituRegion, screen.country)))
	if screen.bandSelector != nil {
		screen.bandSelector.selectedCategory = screen.bandCategory
		screen.bandSelector.selectedName = screen.bandName
	}
	screen.updateBandForFrequency(screen.frequencyHz)
	if !validBand(screen.bandCategory, screen.bandName) {
		def := activeBandPlan.Default
		screen.selectBand(BandDefinition{Category: def.Category, Name: def.Name, FrequencyHz: def.FrequencyHz, SpanHz: def.SpanHz})
	}
	if screen.aprsPanel != nil {
		screen.aprsPanel.refreshLocale()
	}
	if screen.satellitePanel != nil {
		screen.satellitePanel.resolveStation()
		screen.satellitePanel.syncStationFields()
	}
	screen.refreshLocaleChrome()
	screen.markSettingsDirty()
}

func (screen *MainScreen) refreshLocaleChrome() {
	if screen.band != nil {
		label := T("BAND") + "  " + screen.bandName
		if screen.bandName == "OUT OF BAND" {
			label = T("BAND OUT")
		}
		screen.band.SetLabel(label)
	}
	if screen.squelchSwitch != nil {
		screen.squelchSwitch.SetLabel(T("SQL"))
	}
	if screen.memViewSwitch != nil {
		screen.memViewSwitch.SetLabel(T("MEM VIEW"))
	}
	if screen.vfoModeSwitch != nil {
		if screen.centerMode {
			screen.vfoModeSwitch.SetLabel(T("CENTER"))
		} else {
			screen.vfoModeSwitch.SetLabel(T("FIX"))
		}
	}
	if screen.themeButton != nil {
		screen.themeButton.SetLabel(T("THEME"))
	}
	if screen.localeButton != nil {
		screen.localeButton.SetLabel(T("LANGUAGE"))
	}
	if screen.step != nil {
		screen.step.SetLabel(T("STEP"))
	}
	if screen.viewButton != nil {
		screen.viewButton.SetLabel(T("VIEW") + "  " + formatView(screen.viewMode))
	}
	if screen.menuButton != nil {
		screen.menuButton.SetLabel(T("MENU"))
	}
	if screen.muteSwitch != nil {
		screen.muteSwitch.SetLabel(T("MUTE"))
	}
	if screen.squelchLabel != nil {
		screen.squelchLabel.SetText(squelchLevelText(screen.squelchThreshold))
	}
	if screen.holdLabel != nil {
		screen.holdLabel.SetText(holdTimeText(screen.squelchHoldMs))
	}
	if screen.closeLabel != nil {
		screen.closeLabel.SetText(closeTimeText(screen.squelchCloseMs))
	}
	if screen.volumeLabel != nil {
		if screen.muted {
			screen.volumeLabel.SetText(T("MUTED"))
			screen.volumeLabel.SetColor(colors.red)
		} else {
			screen.volumeLabel.SetText(fmt.Sprintf("%s %.0f%%", T("VOL"), screen.volume))
			screen.volumeLabel.SetColor(colors.cyan)
		}
	}
	if screen.sdrSettings != nil {
		screen.sdrSettings.refreshLocale()
	}
	if screen.sdrHeader != nil {
		screen.sdrHeader.refreshLocale()
	}
	if screen.utilitiesSidebar != nil {
		screen.utilitiesSidebar.refreshLocale()
	}
	if screen.fftDisplay != nil {
		screen.fftDisplay.refreshLabels()
	}
	if screen.audioPanel != nil {
		screen.audioPanel.refreshLocale()
	}
	if screen.wfOffsetLabel != nil {
		screen.refreshWaterfallControls()
	}
	if screen.filterSelector != nil {
		screen.filterSelector.refreshLocale()
	}
	if screen.stepSelector != nil {
		screen.stepSelector.refreshLocale()
	}
}

func formatView(mode int) string {
	switch mode {
	case 2:
		return "2"
	case 3:
		return "3"
	default:
		return "1"
	}
}

func squelchLevelText(value int) string {
	return fmt.Sprintf("%s %d dBm", T("LEVEL"), value)
}

func holdTimeText(value int) string {
	return fmt.Sprintf("%s %d ms", T("HOLD TIME"), value)
}

func closeTimeText(value int) string {
	return fmt.Sprintf("%s %d ms", T("CLOSE TIME"), value)
}

func aprsTuneLabel() string {
	return fmt.Sprintf("APRS  %.3f", float64(activeBandPlan.APRSHz)/1e6)
}

func restoreDefaultLocaleForTests() {
	localeOverride = "en-GB"
	i18n.SetLanguage("en")
	applyBandPlan(bandplan.Must("itu-r1"))
}
