package qrz

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSnapshotExtractsQRZSummary(t *testing.T) {
	text := "EA3IWW\nSpain\nLuis Lopez \"Luis\" Martinez\nGava\nBarcelona 08850\nSpain\nEmail: Use mouse to view...\nHam Member Lookups: 1151\nLatitude 41.307621\nLongitude 2.002385\nGrid Square JN11ah"
	result := parseSnapshot("EA3IWW", pageSnapshot{Text: text, ImageURL: "https://example.test/photo.jpg"})
	if result.Name != `Luis Lopez "Luis" Martinez` || result.City != "Gava" || result.Locator != "JN11ah" || !result.Coordinates {
		t.Fatalf("unexpected QRZ result: %+v", result)
	}
	if result.CountryCode != "ES" || result.Continent != "Europa" {
		t.Fatalf("country metadata = %q/%q", result.CountryCode, result.Continent)
	}
}

func TestCachedImageIsReusedWhenPageImageLoadsLate(t *testing.T) {
	cache := t.TempDir()
	path := filepath.Join(cache, "EA3IWW.jpg")
	if err := os.WriteFile(path, []byte("image"), 0600); err != nil {
		t.Fatal(err)
	}
	client := New(t.TempDir(), cache)
	if got := client.cachedImage("EA3IWW"); got != path {
		t.Fatalf("cached image = %q, want %q", got, path)
	}
}

func TestParseSnapshotAcceptsCallsignAndCountryOnSameVisualLine(t *testing.T) {
	text := "Search by Callsign\nEA3IWW Spain\nLuis lopez \"Luis\" martinez\nGava\nBarcelona 08850\nSpain\nEmail: Use mouse to view...\nHam Member Lookups: 1152\nLatitude 41.307621\nLongitude 2.002385\nGrid Square JN11ah"
	result := parseSnapshot("EA3IWW", pageSnapshot{Text: text})
	if result.Name != `Luis lopez "Luis" martinez` || result.City != "Gava" || result.Region != "Barcelona 08850" || result.Country != "Spain" {
		t.Fatalf("combined header was not parsed: %+v", result)
	}
	if result.Member != "HAM MEMBER" || result.Lookups != "1152" {
		t.Fatalf("membership summary was not parsed: %+v", result)
	}
}

func TestParseSnapshotResolvesDenmarkFlagMetadata(t *testing.T) {
	text := "OV2E\nDenmark\nEDR Ballerup Contest Team\nMagleparken 5 1.\nBallerup 2750\n🇩🇰 Denmark\nEmail: Use mouse to view...\nLookups: 2772\nLatitude 55.728333\nLongitude 12.373333\nGrid Square JO65er"
	result := parseSnapshot("OV2E", pageSnapshot{Text: text})
	if result.Country != "Denmark" || result.CountryCode != "DK" || result.Continent != "Europa" {
		t.Fatalf("Denmark metadata = country %q, code %q, continent %q", result.Country, result.CountryCode, result.Continent)
	}
}

func TestNormalizeCountryName(t *testing.T) {
	for input, want := range map[string]string{
		"🇩🇰 Denmark":       "Denmark",
		"  🇪🇸 Spain  ":     "Spain",
		"(Portugal)":       "Portugal",
		"Country: Germany": "Germany",
		"País - México":    "México",
	} {
		if got := normalizeCountryName(input); got != want {
			t.Errorf("normalizeCountryName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestCountryFromSnapshotPrefersISOHintAndHandlesDecoratedNames(t *testing.T) {
	if got := countryFromSnapshot("texto inesperado", "DK").Alpha2(); got != "DK" {
		t.Fatalf("ISO hint resolved as %q", got)
	}
	if got := countryFromSnapshot("Flag United States profile", "").Alpha2(); got != "US" {
		t.Fatalf("decorated multi-word country resolved as %q", got)
	}
	if got := countryFromSnapshot("🇩🇰 Denmark", "ES").Alpha2(); got != "DK" {
		t.Fatalf("country name did not reject mismatched page flag: %q", got)
	}
}

func TestCountryFromSnapshotDoesNotTreatLoginWarningAsIceland(t *testing.T) {
	if got := countryFromSnapshot("Login is required for additional details", ""); got != 0 {
		t.Fatalf("login warning resolved as country %q", got.Alpha2())
	}
}

func TestExtractSnapshotRequiresAuthenticatedSession(t *testing.T) {
	snapshot := pageSnapshot{
		Text: "EA3IWW Spain\nLuis\nGava\nSpain\nEmail: login required\nGrid: JN11ah",
	}
	if _, ready := extractSnapshot("EA3IWW", snapshot); ready {
		t.Fatal("unauthenticated QRZ page was accepted as a station profile")
	}
	snapshot.Authenticated = true
	result, ready := extractSnapshot("EA3IWW", snapshot)
	if !ready || result.CountryCode != "ES" || result.Locator != "JN11ah" {
		t.Fatalf("authenticated profile was not parsed: ready=%v result=%+v", ready, result)
	}
}

func TestValidPNGRejectsHTMLAndTruncatedData(t *testing.T) {
	valid := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 16)...)
	if !validPNG(valid) {
		t.Fatal("valid PNG signature was rejected")
	}
	if validPNG([]byte("<html>Cloudflare challenge</html>")) || validPNG([]byte("\x89PNG")) {
		t.Fatal("invalid flag payload was accepted")
	}
}
