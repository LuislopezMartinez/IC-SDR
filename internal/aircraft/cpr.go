package aircraft

import "math"

// Compact Position Reporting follows 1090-WP-9-14 / DO-260B.
// The NL table is symmetric about the equator, so the same decoder
// works in both hemispheres. A closed-form NL approximation does not.

var cprNLLimits = []float64{
	10.47047130, 14.82817437, 18.18626357, 21.02939493, 23.54504487,
	25.82924707, 27.93898710, 29.91135686, 31.77209708, 33.53993436,
	35.22899598, 36.85025108, 38.41241892, 39.92256684, 41.38651832,
	42.80914012, 44.19454951, 45.54626723, 46.86733252, 48.16039128,
	49.42776439, 50.67150166, 51.89342469, 53.09516153, 54.27817472,
	55.44378444, 56.59318756, 57.72747354, 58.84763776, 59.95459277,
	61.04917774, 62.13216659, 63.20427479, 64.26616523, 65.31845310,
	66.36171008, 67.39646774, 68.42322022, 69.44242631, 70.45451075,
	71.45986473, 72.45884545, 73.45177442, 74.43893416, 75.42056257,
	76.39684391, 77.36789461, 78.33374083, 79.29428225, 80.24923213,
	81.19801349, 82.13956981, 83.07199445, 83.99173563, 84.89166191,
	85.75541621, 86.53536998, 87.00000000,
}

func cprModInt(a, b int) int {
	r := a % b
	if r < 0 {
		r += b
	}
	return r
}

func cprModFloat(a, b float64) float64 {
	r := math.Mod(a, b)
	if r < 0 {
		r += b
	}
	return r
}

func cprNL(lat float64) int {
	lat = math.Abs(lat)
	for i, limit := range cprNLLimits {
		if lat < limit {
			return 59 - i
		}
	}
	return 1
}

func cprN(lat float64, odd bool) int {
	n := cprNL(lat)
	if odd {
		n--
	}
	if n < 1 {
		return 1
	}
	return n
}

func cprDlon(lat float64, odd, surface bool) float64 {
	span := 360.0
	if surface {
		span = 90
	}
	return span / float64(cprN(lat, odd))
}

func wrap180(lon float64) float64 {
	return lon - math.Floor((lon+180)/360)*360
}

func decodeCPRAirborne(evenLat, evenLon, oddLat, oddLon int, odd bool) (float64, float64, bool) {
	j := int(math.Floor((59*float64(evenLat)-60*float64(oddLat))/131072 + 0.5))
	rlat0 := 6 * (float64(cprModInt(j, 60)) + float64(evenLat)/131072)
	rlat1 := (360.0 / 59) * (float64(cprModInt(j, 59)) + float64(oddLat)/131072)
	if rlat0 >= 270 {
		rlat0 -= 360
	}
	if rlat1 >= 270 {
		rlat1 -= 360
	}
	if rlat0 < -90 || rlat0 > 90 || rlat1 < -90 || rlat1 > 90 {
		return 0, 0, false
	}
	if cprNL(rlat0) != cprNL(rlat1) {
		return 0, 0, false
	}
	lat, cprlon := rlat0, evenLon
	if odd {
		lat, cprlon = rlat1, oddLon
	}
	ni := cprN(lat, odd)
	m := int(math.Floor((float64(evenLon)*(float64(cprNL(lat))-1)-float64(oddLon)*float64(cprNL(lat)))/131072 + 0.5))
	lon := cprDlon(lat, odd, false) * (float64(cprModInt(m, ni)) + float64(cprlon)/131072)
	return lat, wrap180(lon), true
}

func decodeCPRSurface(evenLat, evenLon, oddLat, oddLon int, odd bool, reflon float64) (float64, float64, bool) {
	j := int(math.Floor((59*float64(evenLat)-60*float64(oddLat))/131072 + 0.5))
	rlat0 := 1.5 * (float64(cprModInt(j, 60)) + float64(evenLat)/131072)
	rlat1 := (90.0 / 59) * (float64(cprModInt(j, 59)) + float64(oddLat)/131072)
	if rlat0 >= 270 {
		rlat0 -= 360
	}
	if rlat1 >= 270 {
		rlat1 -= 360
	}
	if rlat0 < -90 || rlat0 > 90 || rlat1 < -90 || rlat1 > 90 {
		return 0, 0, false
	}
	if cprNL(rlat0) != cprNL(rlat1) {
		return 0, 0, false
	}
	lat, cprlon := rlat0, evenLon
	if odd {
		lat, cprlon = rlat1, oddLon
	}
	ni := cprN(lat, odd)
	m := int(math.Floor((float64(evenLon)*(float64(cprNL(lat))-1)-float64(oddLon)*float64(cprNL(lat)))/131072 + 0.5))
	lon := cprDlon(lat, odd, true) * (float64(cprModInt(m, ni)) + float64(cprlon)/131072)
	lon -= math.Floor(lon/90) * 90
	lon += 90 * math.Floor(0.5+(wrap180(reflon)-lon)/90)
	return lat, wrap180(lon), true
}

func validLatLon(lat, lon float64) bool {
	return !math.IsNaN(lat) && !math.IsNaN(lon) && lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180
}

func earthDistanceKm(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371.0
	p1, p2 := lat1*math.Pi/180, lat2*math.Pi/180
	dlat := (lat2 - lat1) * math.Pi / 180
	dlon := wrap180(lon2-lon1) * math.Pi / 180
	a := math.Sin(dlat/2)*math.Sin(dlat/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dlon/2)*math.Sin(dlon/2)
	return 2 * r * math.Asin(math.Min(1, math.Sqrt(a)))
}

func decodeCPRRelative(reflat, reflon float64, cprlat, cprlon int, odd, surface bool) (float64, float64, bool) {
	span := 360.0
	if surface {
		span = 90
	}
	dlat := span / 60
	if odd {
		dlat = span / 59
	}
	fracLat := float64(cprlat) / 131072
	j := int(math.Floor(reflat/dlat) + math.Floor(0.5+cprModFloat(reflat, dlat)/dlat-fracLat))
	lat := dlat * (float64(j) + fracLat)
	if lat >= 270 {
		lat -= 360
	}
	if lat < -90 || lat > 90 || math.Abs(lat-reflat) > dlat/2 {
		return 0, 0, false
	}
	dlon := cprDlon(lat, odd, surface)
	fracLon := float64(cprlon) / 131072
	m := int(math.Floor(reflon/dlon) + math.Floor(0.5+cprModFloat(reflon, dlon)/dlon-fracLon))
	lon := dlon * (float64(m) + fracLon)
	lon = wrap180(lon)
	if math.Abs(wrap180(lon-reflon)) > dlon/2 {
		return 0, 0, false
	}
	return lat, lon, true
}
