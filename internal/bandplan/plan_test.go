package bandplan

import "testing"

func TestR1MatchesLegacyEuropeCatalog(t *testing.T) {
	plan := Must("itu-r1")
	if len(plan.Selector["HAM"]) != 15 || len(plan.Selector["COMMERCIAL"]) != 13 || len(plan.Selector["ISM"]) != 6 {
		t.Fatalf("R1 counts HAM=%d COMMERCIAL=%d ISM=%d", len(plan.Selector["HAM"]), len(plan.Selector["COMMERCIAL"]), len(plan.Selector["ISM"]))
	}
	pmr := plan.Selector["ISM"][1]
	if pmr.Name != "PMR446" || pmr.FrequencyHz != 446_006_250 {
		t.Fatalf("PMR446 = %+v", pmr)
	}
}

func TestR2UsesAmericanAmateurEdges(t *testing.T) {
	plan := Must("itu-r2")
	found4m, found125, foundPMR := false, false, false
	for _, item := range plan.Ranges {
		if item.Name == "4 m" {
			found4m = true
		}
		if item.Name == "1.25 m" {
			found125 = true
		}
		if item.Name == "PMR446" {
			foundPMR = true
		}
		if item.Name == "2 m" && (item.MinHz != 144_000_000 || item.MaxHz != 148_000_000) {
			t.Fatalf("R2 2 m = %+v", item)
		}
		if item.Name == "80 m" && item.MaxHz != 4_000_000 {
			t.Fatalf("R2 80 m = %+v", item)
		}
	}
	if found4m || foundPMR || !found125 {
		t.Fatalf("R2 shape 4m=%v 1.25m=%v PMR=%v", found4m, found125, foundPMR)
	}
	if plan.APRSHz != 144_390_000 {
		t.Fatalf("R2 APRS %d", plan.APRSHz)
	}
}

func TestAustraliaOverridesAPRSAndUHFCB(t *testing.T) {
	plan := Must("au")
	if plan.ITU != "R3" || plan.APRSHz != 145_175_000 {
		t.Fatalf("AU plan %+v", plan)
	}
	found := false
	for _, item := range plan.Selector["ISM"] {
		if item.Name == "UHF CB" {
			found = true
		}
		if item.Name == "PMR446" {
			t.Fatal("Australia should not list PMR446")
		}
	}
	if !found {
		t.Fatal("Australia missing UHF CB")
	}
}

func TestResolvePrefersCountryOverRegion(t *testing.T) {
	if got := Resolve("itu-r1", "au"); got != "au" {
		t.Fatalf("got %s", got)
	}
	if got := Resolve("itu-r3", "auto"); got != "itu-r3" {
		t.Fatalf("got %s", got)
	}
}

func TestFromLocaleAustralia(t *testing.T) {
	lang, region, country := FromLocale("en-AU")
	if lang != "en" || region != "itu-r3" || country != "au" {
		t.Fatalf("%s %s %s", lang, region, country)
	}
	lang, region, country = FromLocale("en-US")
	if region != "itu-r2" || country != "us" {
		t.Fatalf("US %s %s", region, country)
	}
	lang, region, country = FromLocale("es-ES")
	if lang != "es" || region != "itu-r1" || country != "es" {
		t.Fatalf("ES %s %s %s", lang, region, country)
	}
}

func TestCountriesForRegion(t *testing.T) {
	r2 := CountriesFor("R2")
	if len(r2) < 8 {
		t.Fatalf("expected several R2 countries, got %d", len(r2))
	}
	for _, plan := range r2 {
		if plan.ITU != "R2" {
			t.Fatalf("%s is %s", plan.ID, plan.ITU)
		}
	}
}

func TestExtraCountryPlansRegister(t *testing.T) {
	if _, ok := Get("nl"); !ok {
		t.Fatal("missing Netherlands plan")
	}
	if _, ok := Get("sg"); !ok {
		t.Fatal("missing Singapore plan")
	}
	if Resolve("itu-r1", "nl") != "nl" {
		t.Fatal("Netherlands should win over ITU R1")
	}
}
