package bandplan

import "strings"

// FromLocale maps an OS locale tag (en-AU, es-ES, pt-BR) to UI language plus
// the ITU region and country band plan that should start with the app.
func FromLocale(tag string) (language, region, country string) {
	tag = strings.ReplaceAll(strings.TrimSpace(tag), "_", "-")
	if tag == "" {
		return "en", "itu-r1", "auto"
	}
	parts := strings.Split(tag, "-")
	lang := strings.ToLower(parts[0])
	regionTag := ""
	if len(parts) > 1 {
		regionTag = strings.ToUpper(parts[len(parts)-1])
	}
	language = lang
	if strings.EqualFold(tag, "pt-BR") || regionTag == "BR" && lang == "pt" {
		language = "pt-BR"
	}
	switch regionTag {
	case "US", "CA", "MX", "BR", "AR", "CL", "CO", "PE", "VE", "UY", "PY", "BO", "EC", "CR", "PA", "GT", "HN", "NI", "SV", "DO", "PR", "CU":
		region = "itu-r2"
	case "AU", "NZ", "JP", "CN", "KR", "IN", "ID", "PH", "TH", "VN", "MY", "SG", "TW", "HK", "MO", "PK", "BD", "LK", "MM", "KH", "LA", "MN", "FJ", "PG":
		region = "itu-r3"
	default:
		region = "itu-r1"
	}
	switch regionTag {
	case "US", "CA", "MX", "BR", "AR", "AU", "NZ", "JP", "UK", "GB", "DE", "ES", "FR", "IT", "PL", "ZA", "RU",
		"IN", "CN", "KR", "ID", "PH", "NL", "BE", "AT", "CH", "IE", "PT", "SE", "NO", "FI", "DK", "CZ", "HU",
		"RO", "TR", "UA", "GR", "BG", "CL", "CO", "PE", "VE", "UY", "SG", "MY", "TW", "VN", "TH", "HK":
		country = strings.ToLower(regionTag)
		if regionTag == "GB" {
			country = "uk"
		}
	default:
		country = "auto"
	}
	return language, region, country
}
