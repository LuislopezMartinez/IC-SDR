package i18n

import "testing"

func TestLanguageNormalize(t *testing.T) {
	if Normalize("es") != "es" {
		t.Fatalf("es: %q", Normalize("es"))
	}
	if Normalize("ES") != "es" {
		t.Fatalf("ES: %q", Normalize("ES"))
	}
	if Normalize("pt-BR") != "pt-BR" {
		t.Fatalf("pt-BR: %q", Normalize("pt-BR"))
	}
	if Normalize("pt-PT") != "pt" {
		t.Fatalf("pt-PT should fall back to pt, got %q", Normalize("pt-PT"))
	}
	if Normalize("xx") != "en" {
		t.Fatalf("unknown language should be en, got %q", Normalize("xx"))
	}
}

func TestSpanishTranslatesChrome(t *testing.T) {
	SetLanguage("es")
	defer SetLanguage("en")
	if T("START") != "INICIAR" || T("OPEN MAP") != "ABRIR MAPA" {
		t.Fatalf("spanish chrome: %q %q", T("START"), T("OPEN MAP"))
	}
	if T("not a catalog key") != "not a catalog key" {
		t.Fatal("missing keys must fall back to the English source")
	}
}

func TestFormatUsesTranslatedTemplate(t *testing.T) {
	SetLanguage("en")
	if got := T("VIEW %d", 2); got != "VIEW 2" {
		t.Fatalf("format = %q", got)
	}
}

func TestEveryLanguageIsRegistered(t *testing.T) {
	for _, language := range Languages() {
		if language.Code == "en" {
			continue
		}
		if packs[language.Code] == nil {
			t.Fatalf("language %s has no catalog", language.Code)
		}
		SetLanguage(language.Code)
		if T("START") == "" {
			t.Fatalf("%s START empty", language.Code)
		}
	}
	SetLanguage("en")
}
