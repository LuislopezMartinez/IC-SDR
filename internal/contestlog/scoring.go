package contestlog

import "strings"

type BandScore struct {
	QSOs, Points, Multipliers int
}

type Score struct {
	ValidQSOs, Duplicates, Warnings, NonScoring, Incomplete int
	Points, Multipliers, Total                              int
	Bands                                                   map[string]BandScore
}

var bandPoints = map[string]int{"160M": 20, "80M": 10, "40M": 5, "20M": 1, "15M": 2, "10M": 3}

func (s *Session) Score() Score {
	result := Score{Bands: make(map[string]BandScore)}
	if s == nil {
		return result
	}
	if s.Contest != OceaniaDXSSB {
		return s.genericScore()
	}
	stationInOceania := IsOceaniaCall(s.StationCall)
	multipliers := make(map[string]bool)
	for _, q := range s.QSOs {
		if q.Time.IsZero() || q.FrequencyHz <= 0 || q.WorkedCall == "" || q.RSTSent == "" || q.RSTReceived == "" || q.SerialSent < 1 || q.SentExchangeText() == "" || q.ReceivedExchangeText() == "" {
			result.Incomplete++
			continue
		}
		if q.Duplicate {
			result.Duplicates++
			continue
		}
		if len(q.Warnings) > 0 {
			result.Warnings++
			continue
		}
		if s.CategoryBand != "" && s.CategoryBand != "ALL" && q.Band != s.CategoryBand {
			result.NonScoring++
			continue
		}
		if !stationInOceania && !IsOceaniaCall(q.WorkedCall) {
			result.NonScoring++
			continue
		}
		points, ok := bandPoints[q.Band]
		if !ok {
			result.NonScoring++
			continue
		}
		prefix := WPXPrefix(q.WorkedCall)
		if prefix == "" {
			result.NonScoring++
			continue
		}
		band := result.Bands[q.Band]
		band.QSOs++
		band.Points += points
		key := q.Band + "\x00" + prefix
		if !multipliers[key] {
			multipliers[key] = true
			band.Multipliers++
		}
		result.Bands[q.Band] = band
		result.ValidQSOs++
		result.Points += points
	}
	result.Multipliers = len(multipliers)
	result.Total = result.Points * result.Multipliers
	return result
}

func (s *Session) genericScore() Score {
	result := Score{Bands: make(map[string]BandScore)}
	pointsPerQSO := s.PointsPerQSO
	if pointsPerQSO < 1 {
		pointsPerQSO = 1
	}
	for _, q := range s.QSOs {
		if q.Time.IsZero() || q.FrequencyHz <= 0 || q.WorkedCall == "" || q.RSTSent == "" || q.RSTReceived == "" || q.SerialSent < 1 || q.SentExchangeText() == "" || q.ReceivedExchangeText() == "" {
			result.Incomplete++
			continue
		}
		if q.Duplicate {
			result.Duplicates++
			continue
		}
		if len(q.Warnings) > 0 {
			result.Warnings++
			continue
		}
		if s.CategoryBand != "" && s.CategoryBand != "ALL" && q.Band != s.CategoryBand {
			result.NonScoring++
			continue
		}
		band := result.Bands[q.Band]
		band.QSOs++
		band.Points += pointsPerQSO
		result.Bands[q.Band] = band
		result.ValidQSOs++
		result.Points += pointsPerQSO
	}
	if result.Points > 0 {
		result.Multipliers = 1
		result.Total = result.Points
	}
	return result
}

func WPXPrefix(call string) string {
	call = NormalizeCall(call)
	if call == "" {
		return ""
	}
	parts := strings.Split(call, "/")
	ignored := map[string]bool{"P": true, "M": true, "MM": true, "AM": true, "A": true, "E": true, "J": true, "QRP": true}
	base := ""
	for _, part := range parts {
		if part == "" || ignored[part] {
			continue
		}
		if base == "" {
			base = part
		}
		if len(parts) > 1 && len(part) <= 4 && part != base {
			base = part
		}
		if len(parts) > 1 && len(part) <= 4 && strings.IndexFunc(part, func(r rune) bool { return r >= '0' && r <= '9' }) >= 0 {
			base = part
			break
		}
	}
	if base == "" {
		return ""
	}
	lastDigit := -1
	for index, r := range base {
		if r >= '0' && r <= '9' {
			lastDigit = index
		}
	}
	if lastDigit >= 0 {
		return base[:lastDigit+1]
	}
	letters := []rune(base)
	if len(letters) > 2 {
		letters = letters[:2]
	}
	return string(letters) + "0"
}

func IsOceaniaCall(call string) bool {
	call = NormalizeCall(call)
	for _, part := range strings.Split(call, "/") {
		for _, prefix := range []string{"9M6", "9M8", "4W", "P2", "VK", "AX", "VH", "VJ", "VL", "ZL", "ZM",
			"DU", "DV", "DW", "DX", "DY", "DZ", "YB", "YC", "YD", "YE", "YF", "YG", "YH",
			"A3", "C2", "E5", "E6", "FK", "FO", "H4", "KH0", "WH0", "AH0", "NH0", "KH2", "WH2", "AH2", "NH2",
			"KH6", "WH6", "AH6", "NH6", "KH7", "WH7", "AH7", "NH7", "KH8", "WH8", "AH8", "NH8", "KH9",
			"T2", "T3", "T8", "V6", "V7", "5W", "YJ", "ZK3", "3D2", "JD1"} {
			if strings.HasPrefix(part, prefix) {
				return true
			}
		}
		if len(part) >= 2 && (part[0] == '7' || part[0] == '8') && part[1] >= 'A' && part[1] <= 'I' {
			return true
		}
	}
	return false
}
