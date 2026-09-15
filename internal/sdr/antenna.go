package sdr

import "strings"

func IsRSPDx(name string) bool {
	name = strings.ToLower(name)
	name = strings.NewReplacer("-", "", " ", "").Replace(name)
	return strings.Contains(name, "rspdx")
}

func AntennaShortLabel(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	switch {
	case lower == "a" || strings.Contains(lower, "antenna a"):
		return "A"
	case lower == "b" || strings.Contains(lower, "antenna b"):
		return "B"
	case lower == "c" || strings.Contains(lower, "antenna c"):
		return "C"
	default:
		return strings.TrimSpace(name)
	}
}

func MatchAntenna(name string, available []string) string {
	for _, candidate := range available {
		if strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(name)) {
			return candidate
		}
	}
	want := AntennaShortLabel(name)
	for _, candidate := range available {
		if AntennaShortLabel(candidate) == want {
			return candidate
		}
	}
	return ""
}
