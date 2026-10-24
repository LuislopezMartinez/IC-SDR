package sdr

import "strings"

func IsRSPDx(name string) bool {
	name = normalizedDeviceName(name)
	return strings.Contains(name, "rspdx")
}

func IsRSPDuo(name string) bool {
	return strings.Contains(normalizedDeviceName(name), "rspduo")
}

func HasSelectableAntennas(name string) bool { return IsRSPDx(name) || IsRSPDuo(name) }

func normalizedDeviceName(name string) string {
	name = strings.ToLower(name)
	return strings.NewReplacer("-", "", " ", "").Replace(name)
}

func AntennaShortLabel(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.Contains(lower, "tuner 1") && strings.Contains(lower, "hi-z"):
		return "T1 Hi-Z"
	case strings.Contains(lower, "tuner 1"):
		return "T1 50Ω"
	case strings.Contains(lower, "tuner 2"):
		return "T2 50Ω"
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
