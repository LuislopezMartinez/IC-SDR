package voacap

import (
	"math"
	"time"
)

type LiveAssessment struct {
	Confidence string
	Reason     string
}

// AssessLiveConditions qualifies the climatological VOACAP result without
// pretending that space-weather indices are direct REL inputs. R events apply
// on the sunlit side; S and geomagnetic disturbances primarily penalize polar
// and high-latitude paths.
func AssessLiveConditions(lat, lon float64, at time.Time, solar SolarData) LiveAssessment {
	severity, reason := 0, "CONDICIONES NORMALES"
	if solar.RScale > 0 && solarElevation(lat, lon, at) > -6 {
		severity, reason = min(2, solar.RScale), "ABSORCIÓN CAPA D"
	}
	if math.Abs(lat) >= 55 && solar.SScale > 0 && solar.SScale >= severity {
		severity, reason = min(2, solar.SScale), "ABSORCIÓN POLAR"
	}
	geomagnetic := solar.GScale
	if solar.Kp >= 7 {
		geomagnetic = max(geomagnetic, 2)
	} else if solar.Kp >= 5 {
		geomagnetic = max(geomagnetic, 1)
	}
	if math.Abs(lat) >= 50 && geomagnetic > 0 && geomagnetic >= severity {
		severity, reason = min(2, geomagnetic), "PERTURBACIÓN GEOMAGNÉTICA"
	}
	confidence := "ALTA"
	if severity == 1 {
		confidence = "MEDIA"
	} else if severity >= 2 {
		confidence = "BAJA"
	}
	return LiveAssessment{Confidence: confidence, Reason: reason}
}

func solarElevation(lat, lon float64, at time.Time) float64 {
	at = at.UTC()
	day := float64(at.YearDay())
	hour := float64(at.Hour()) + float64(at.Minute())/60 + float64(at.Second())/3600
	gamma := 2 * math.Pi / 365 * (day - 1 + (hour-12)/24)
	equation := 229.18 * (.000075 + .001868*math.Cos(gamma) - .032077*math.Sin(gamma) - .014615*math.Cos(2*gamma) - .040849*math.Sin(2*gamma))
	declination := .006918 - .399912*math.Cos(gamma) + .070257*math.Sin(gamma) - .006758*math.Cos(2*gamma) + .000907*math.Sin(2*gamma) - .002697*math.Cos(3*gamma) + .00148*math.Sin(3*gamma)
	trueSolarMinutes := math.Mod(hour*60+equation+4*lon+1440, 1440)
	hourAngle := trueSolarMinutes/4 - 180
	latRad, hourRad := lat*math.Pi/180, hourAngle*math.Pi/180
	zenith := math.Acos(math.Sin(latRad)*math.Sin(declination) + math.Cos(latRad)*math.Cos(declination)*math.Cos(hourRad))
	return 90 - zenith*180/math.Pi
}
