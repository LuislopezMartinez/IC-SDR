package i18n

import (
	"fmt"
	"strings"
)

// Language is a user-selectable UI pack. Codes are BCP-47-ish (pt-BR).
type Language struct {
	Code, Name, Native string
}

var languages = []Language{
	{Code: "en", Name: "English", Native: "English"},
	{Code: "es", Name: "Spanish", Native: "Español"},
	{Code: "ca", Name: "Catalan", Native: "Català"},
	{Code: "gl", Name: "Galician", Native: "Galego"},
	{Code: "eu", Name: "Basque", Native: "Euskara"},
	{Code: "pt", Name: "Portuguese", Native: "Português"},
	{Code: "pt-BR", Name: "Portuguese (Brazil)", Native: "Português (Brasil)"},
	{Code: "fr", Name: "French", Native: "Français"},
	{Code: "de", Name: "German", Native: "Deutsch"},
	{Code: "it", Name: "Italian", Native: "Italiano"},
	{Code: "nl", Name: "Dutch", Native: "Nederlands"},
	{Code: "pl", Name: "Polish", Native: "Polski"},
	{Code: "cs", Name: "Czech", Native: "Čeština"},
	{Code: "sk", Name: "Slovak", Native: "Slovenčina"},
	{Code: "hu", Name: "Hungarian", Native: "Magyar"},
	{Code: "ro", Name: "Romanian", Native: "Română"},
	{Code: "hr", Name: "Croatian", Native: "Hrvatski"},
	{Code: "sl", Name: "Slovenian", Native: "Slovenščina"},
	{Code: "sr", Name: "Serbian", Native: "Srpski"},
	{Code: "tr", Name: "Turkish", Native: "Türkçe"},
	{Code: "sv", Name: "Swedish", Native: "Svenska"},
	{Code: "da", Name: "Danish", Native: "Dansk"},
	{Code: "nb", Name: "Norwegian", Native: "Norsk"},
	{Code: "fi", Name: "Finnish", Native: "Suomi"},
	{Code: "et", Name: "Estonian", Native: "Eesti"},
	{Code: "lv", Name: "Latvian", Native: "Latviešu"},
	{Code: "lt", Name: "Lithuanian", Native: "Lietuvių"},
	{Code: "el", Name: "Greek", Native: "Ελληνικά"},
	{Code: "ru", Name: "Russian", Native: "Русский"},
	{Code: "uk", Name: "Ukrainian", Native: "Українська"},
	{Code: "bg", Name: "Bulgarian", Native: "Български"},
	{Code: "id", Name: "Indonesian", Native: "Bahasa Indonesia"},
	{Code: "vi", Name: "Vietnamese", Native: "Tiếng Việt"},
	{Code: "sq", Name: "Albanian", Native: "Shqip"},
	{Code: "bs", Name: "Bosnian", Native: "Bosanski"},
	{Code: "is", Name: "Icelandic", Native: "Íslenska"},
	{Code: "ga", Name: "Irish", Native: "Gaeilge"},
	{Code: "cy", Name: "Welsh", Native: "Cymraeg"},
	{Code: "mk", Name: "Macedonian", Native: "Македонски"},
	{Code: "be", Name: "Belarusian", Native: "Беларуская"},
}

var (
	current = "en"
	packs   = map[string]map[string]string{}
)

func init() {
	registerCatalogs()
}

func Languages() []Language { return append([]Language(nil), languages...) }

func Normalize(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return "en"
	}
	if strings.EqualFold(code, "pt-BR") || strings.EqualFold(code, "pt_BR") {
		return "pt-BR"
	}
	for _, language := range languages {
		if strings.EqualFold(language.Code, code) {
			return language.Code
		}
	}
	if len(code) >= 2 {
		base := strings.ToLower(code[:2])
		for _, language := range languages {
			if language.Code == base {
				return base
			}
		}
	}
	return "en"
}

func SetLanguage(code string) { current = Normalize(code) }

func Current() string { return current }

func T(message string, args ...any) string {
	translated := message
	if current != "en" {
		if pack := packs[current]; pack != nil {
			if value, ok := pack[message]; ok && value != "" {
				translated = value
			}
		}
	}
	if len(args) == 0 {
		return translated
	}
	return fmt.Sprintf(translated, args...)
}

func DisplayName(code string) string {
	code = Normalize(code)
	for _, language := range languages {
		if language.Code == code {
			return language.Native
		}
	}
	return code
}

func pair(values ...string) map[string]string {
	out := make(map[string]string, len(values)/2)
	for i := 0; i+1 < len(values); i += 2 {
		out[values[i]] = values[i+1]
	}
	return out
}
