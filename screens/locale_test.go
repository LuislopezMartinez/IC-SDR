package screens

import (
	"path/filepath"
	"testing"

	"go-zero/internal/bandplan"
	"go-zero/internal/i18n"
	"go-zero/simpleui"
)

func TestUSPlanRemovesPMR446AndAddsFRS(t *testing.T) {
	restoreDefaultLocaleForTests()
	t.Cleanup(restoreDefaultLocaleForTests)
	screen := NewMainScreen(nil)
	screen.setLocale("en", "itu-r2", "us")
	for _, band := range bandsByCategory["ISM"] {
		if band.Name == "PMR446" {
			t.Fatal("US plan still lists PMR446")
		}
	}
	found := false
	for _, band := range bandsByCategory["ISM"] {
		if band.Name == "FRS / GMRS" {
			found = true
		}
	}
	if !found {
		t.Fatal("US plan missing FRS / GMRS")
	}
	if activeBandPlan.APRSHz != 144_390_000 {
		t.Fatalf("US APRS %d", activeBandPlan.APRSHz)
	}
}

func TestAustraliaPlanUsesUHFCB(t *testing.T) {
	restoreDefaultLocaleForTests()
	t.Cleanup(restoreDefaultLocaleForTests)
	screen := NewMainScreen(nil)
	screen.setLocale("en", "itu-r3", "au")
	found := false
	for _, band := range bandsByCategory["ISM"] {
		if band.Name == "UHF CB" {
			found = true
		}
	}
	if !found {
		t.Fatal("Australia missing UHF CB")
	}
	if activeBandPlan.APRSHz != 145_175_000 {
		t.Fatalf("AU APRS %d", activeBandPlan.APRSHz)
	}
}

func TestLanguageSwitchTranslatesChrome(t *testing.T) {
	restoreDefaultLocaleForTests()
	t.Cleanup(restoreDefaultLocaleForTests)
	screen := NewMainScreen(nil)
	screen.localeButton = simpleui.NewButton("locale", 0, 0, 140, 40, "LANGUAGE", 13)
	screen.menuButton = simpleui.NewButton("menu", 0, 0, 110, 40, "MENU", 15)
	screen.setLocale("es", "itu-r1", "es")
	if screen.localeButton.Label() != "IDIOMA" {
		t.Fatalf("language button = %q", screen.localeButton.Label())
	}
	if screen.menuButton.Label() != "MENÚ" {
		t.Fatalf("menu = %q", screen.menuButton.Label())
	}
	if i18n.Current() != "es" {
		t.Fatalf("current language %s", i18n.Current())
	}
}

func TestLocaleSettingsRoundTripFile(t *testing.T) {
	restoreDefaultLocaleForTests()
	t.Cleanup(restoreDefaultLocaleForTests)
	path := filepath.Join(t.TempDir(), "settings.json")
	settings := persistedAppSettings{
		Version: appSettingsVersion, Language: "de", ITURegion: "itu-r2", Country: "us",
		BandCategory: "HAM", BandName: "20 m", Mode: "USB",
		FrequencyHz: 14_261_000, CenterFrequencyHz: 14_261_000,
		SpanHz: 2_000_000, TuningStepHz: 100, CenterMode: true,
	}
	if err := writeAppSettings(path, settings); err != nil {
		t.Fatal(err)
	}
	screen := NewMainScreen(nil)
	loadAppSettings(path, screen)
	screen.applyLocaleAndRegion()
	if screen.language != "de" || bandplan.Resolve(screen.ituRegion, screen.country) != "us" {
		t.Fatalf("locale %s %s %s", screen.language, screen.ituRegion, screen.country)
	}
	foundFRS := false
	for _, band := range bandsByCategory["ISM"] {
		if band.Name == "FRS / GMRS" {
			foundFRS = true
		}
	}
	if !foundFRS {
		t.Fatal("saved US region was not applied")
	}
}

func TestFirstRunLocaleUsesOverride(t *testing.T) {
	localeOverride = "en-AU"
	t.Cleanup(restoreDefaultLocaleForTests)
	screen := &MainScreen{}
	screen.applyLocaleAndRegion()
	if screen.language != "en" || screen.ituRegion != "itu-r3" || screen.country != "au" {
		t.Fatalf("detected %s %s %s", screen.language, screen.ituRegion, screen.country)
	}
	if activeBandPlan.ID != "au" {
		t.Fatalf("applied plan %s", activeBandPlan.ID)
	}
}
