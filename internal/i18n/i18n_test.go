package i18n

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func key(text string) string {
	sum := sha256.Sum256([]byte(text))
	return fmt.Sprintf("text.%x", sum[:6])
}
func isolate(t *testing.T) {
	t.Helper()
	mu.Lock()
	old := map[string]Language{}
	for id, l := range languages {
		old[id] = l
	}
	oldSelected, oldPref := selected, preferencePath
	selected = "es"
	rebuild()
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		languages = old
		selected = oldSelected
		preferencePath = oldPref
		rebuild()
		mu.Unlock()
	})
}
func TestBundledEnglishFormatsAndCoverage(t *testing.T) {
	isolate(t)
	for k, source := range spanish {
		target, ok := languages["en"].Texts[k]
		if !ok || target == "" {
			t.Fatalf("English is missing %s", k)
		}
		if !samePlaceholders(source, target) {
			t.Fatalf("incompatible English format: %q => %q", source, target)
		}
	}
	mu.Lock()
	selected = "en"
	rebuild()
	mu.Unlock()
	for input, want := range map[string]string{"ABRIR MAPA": "OPEN MAP", "LIMPIAR": "CLEAR", "DETALLE DE LA ESTACIÓN": "STATION DETAILS", "INDICATIVO": "CALLSIGN", "VER REGISTRO": "VIEW LOG", "ENVÍAS 003": "SEND 003", "3 estaciones · arrastra para mover · rueda para zoom": "3 stations · drag to pan · mouse wheel to zoom", "Rumbo: 120": "Course: 120", "EA1ABC>APRS:hello": "EA1ABC>APRS:hello"} {
		if got := Display(input); got != want {
			t.Fatalf("%q => %q, want %q", input, got, want)
		}
	}
	if Source(key("LIMPIAR")) != "LIMPIAR" {
		t.Fatal("semantic source changed with language")
	}
}
func TestBundledGermanFormatsCoverageAndSelection(t *testing.T) {
	isolate(t)
	german, ok := languages["de"]
	if !ok || german.Name != "Deutsch" || german.Flag != "DE" {
		t.Fatalf("German catalog metadata is invalid: %+v", german)
	}
	for k, source := range spanish {
		target, ok := german.Texts[k]
		if !ok || target == "" {
			t.Fatalf("German is missing %s", k)
		}
		if !samePlaceholders(source, target) {
			t.Fatalf("incompatible German format: %q => %q", source, target)
		}
	}
	mu.Lock()
	selected = "de"
	rebuild()
	mu.Unlock()
	for input, want := range map[string]string{
		"ABRIR MAPA":                 "KARTE ÖFFNEN",
		"LIMPIAR":                    "LÖSCHEN",
		"INDICATIVO":                 "RUFZEICHEN",
		"VISTAS":                     "ANSICHTEN",
		"DECODIFICADORES":            "DEKODER",
		"BUSCANDO SINCRONISMO":       "SUCHE NACH SYNCHRONISATION",
		"ESPERANDO DATOS DE RED":     "WARTE AUF NETZWERKDATEN",
		"CONTROL TETRAPOL":           "TETRAPOL-STEUERUNG",
		"SINTONÍA":                   "ABSTIMMUNG",
		"DIRECCIÓN":                  "RICHTUNG",
		"ACCESOS DE BANDA":           "FREQUENZBEREICHE",
		"AJUSTE AUTOMÁTICO":          "AUTOMATISCHE EINSTELLUNG",
		"CAPTURAR IQ":                "IQ AUFZEICHNEN",
		"LLAMADAS":                   "ANRUFE",
		"SIN IDENTIFICAR":            "NICHT IDENTIFIZIERT",
		"DESCONOCIDO · AUDIO ACTIVO": "UNBEKANNT · AUDIO AKTIV",
	} {
		if got := Display(input); got != want {
			t.Fatalf("%q => %q, want %q", input, got, want)
		}
	}
}
func TestBundledFrenchFormatsCoverageAndSelection(t *testing.T) {
	isolate(t)
	french, ok := languages["fr"]
	if !ok || french.Name != "Français" || french.Flag != "FR" {
		t.Fatalf("French catalog metadata is invalid: %+v", french)
	}
	for k, source := range spanish {
		target, ok := french.Texts[k]
		if !ok || target == "" {
			t.Fatalf("French is missing %s", k)
		}
		if !samePlaceholders(source, target) {
			t.Fatalf("incompatible French format: %q => %q", source, target)
		}
	}
	mu.Lock()
	selected = "fr"
	rebuild()
	mu.Unlock()
	for input, want := range map[string]string{
		"ABRIR MAPA":             "OUVRIR LA CARTE",
		"LIMPIAR":                "EFFACER",
		"INDICATIVO":             "INDICATIF",
		"VISTAS":                 "VUES",
		"DECODIFICADORES":        "DÉCODEURS",
		"BUSCANDO SINCRONISMO":   "RECHERCHE DE SYNCHRONISATION",
		"ESPERANDO DATOS DE RED": "EN ATTENTE DES DONNÉES RÉSEAU",
		"CONTROL TETRAPOL":       "CONTRÔLE TETRAPOL",
		"DIRECCIÓN":              "DIRECTION",
		"CLARO":                  "EN CLAIR",
	} {
		if got := Display(input); got != want {
			t.Fatalf("%q => %q, want %q", input, got, want)
		}
	}
}
func TestDiscoveryPersistenceFallbackAndInvalidFiles(t *testing.T) {
	isolate(t)
	dir := t.TempDir()
	preference := filepath.Join(dir, "config", "language.json")
	extra := Language{ID: "fr", Name: "Français", Flag: "FR", Texts: map[string]string{key("ABRIR MAPA"): "OUVRIR LA CARTE"}}
	data, _ := json.Marshal(extra)
	_ = os.WriteFile(filepath.Join(dir, "fr.json"), data, 0644)
	_ = os.WriteFile(filepath.Join(dir, "broken.json"), []byte("broken"), 0644)
	bad := Language{ID: "bad", Name: "Bad", Texts: map[string]string{key("%d eventos"): "no number"}}
	data, _ = json.Marshal(bad)
	_ = os.WriteFile(filepath.Join(dir, "bad.json"), data, 0644)
	issues := Load(dir, preference)
	if len(issues) != 2 {
		t.Fatalf("invalid files not reported: %v", issues)
	}
	if Current() != "es" {
		t.Fatal("default is not Spanish")
	}
	if Select("fr") != nil {
		t.Fatal("discovered language not selectable")
	}
	if Display("ABRIR MAPA") != "OUVRIR LA CARTE" || Display("LIMPIAR") != "LIMPIAR" {
		t.Fatal("translation or Spanish fallback failed")
	}
	mu.Lock()
	selected = "es"
	mu.Unlock()
	Load(dir, preference)
	if Current() != "fr" {
		t.Fatal("preference not restored")
	}
	if Select("bad") == nil {
		t.Fatal("invalid language loaded")
	}
}
