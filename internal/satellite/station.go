package satellite

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
)

const (
	madridLatitude  = 40.4168
	madridLongitude = -3.7038
)

// DefaultStation is the observer used when the user has not saved a location.
// Madrid is only the Spanish default; other countries get a local city so the
// map and ADS-B CPR reference are not stuck in Spain.
func DefaultStation(country string) Station {
	switch strings.ToLower(strings.TrimSpace(country)) {
	case "es":
		return Station{Name: "Madrid", Latitude: madridLatitude, Longitude: madridLongitude, AltitudeMeters: 657}
	case "au":
		return Station{Name: "Sydney", Latitude: -33.8688, Longitude: 151.2093, AltitudeMeters: 58}
	case "nz":
		return Station{Name: "Wellington", Latitude: -41.2866, Longitude: 174.7756, AltitudeMeters: 20}
	case "us":
		return Station{Name: "United States", Latitude: 39.8283, Longitude: -98.5795, AltitudeMeters: 500}
	case "ca":
		return Station{Name: "Ottawa", Latitude: 45.4215, Longitude: -75.6972, AltitudeMeters: 70}
	case "gb", "uk":
		return Station{Name: "London", Latitude: 51.5074, Longitude: -0.1278, AltitudeMeters: 35}
	case "de":
		return Station{Name: "Berlin", Latitude: 52.52, Longitude: 13.405, AltitudeMeters: 34}
	case "fr":
		return Station{Name: "Paris", Latitude: 48.8566, Longitude: 2.3522, AltitudeMeters: 35}
	case "it":
		return Station{Name: "Rome", Latitude: 41.9028, Longitude: 12.4964, AltitudeMeters: 21}
	case "jp":
		return Station{Name: "Tokyo", Latitude: 35.6762, Longitude: 139.6503, AltitudeMeters: 40}
	case "in":
		return Station{Name: "New Delhi", Latitude: 28.6139, Longitude: 77.209, AltitudeMeters: 216}
	case "br":
		return Station{Name: "Brasilia", Latitude: -15.7975, Longitude: -47.8919, AltitudeMeters: 1172}
	case "mx":
		return Station{Name: "Mexico City", Latitude: 19.4326, Longitude: -99.1332, AltitudeMeters: 2240}
	case "za":
		return Station{Name: "Johannesburg", Latitude: -26.2041, Longitude: 28.0473, AltitudeMeters: 1753}
	default:
		return Station{Name: "Home", Latitude: 0, Longitude: 0, AltitudeMeters: 0}
	}
}

func sameStation(a, b Station) bool {
	return strings.EqualFold(strings.TrimSpace(a.Name), strings.TrimSpace(b.Name)) &&
		math.Abs(a.Latitude-b.Latitude) < 0.02 &&
		math.Abs(a.Longitude-b.Longitude) < 0.02
}

func IsLegacyMadridDefault(station Station) bool {
	return strings.EqualFold(strings.TrimSpace(station.Name), "Madrid") &&
		math.Abs(station.Latitude-madridLatitude) < 0.02 &&
		math.Abs(station.Longitude-madridLongitude) < 0.02
}

func IsFactoryDefault(station Station) bool {
	if IsLegacyMadridDefault(station) {
		return true
	}
	for _, country := range []string{"es", "au", "nz", "us", "ca", "gb", "de", "fr", "it", "jp", "in", "br", "mx", "za", ""} {
		if sameStation(station, DefaultStation(country)) {
			return true
		}
	}
	return false
}

func unknownCountry(country string) bool {
	country = strings.ToLower(strings.TrimSpace(country))
	return country == "" || country == "auto"
}

// ResolveStation returns the observer to use. A leftover factory Madrid (or
// another country default) is replaced when the app country is known. A
// location the user saved is never overwritten.
func ResolveStation(path, country string) Station {
	current := DefaultStation(country)
	station, ok := LoadStationFile(path)
	if ok {
		if !IsFactoryDefault(station) || unknownCountry(country) || sameStation(station, current) {
			return station
		}
	}
	_ = SaveStationFile(path, current)
	return current
}

func (s Station) Valid() bool {
	return s.Latitude >= -90 && s.Latitude <= 90 && s.Longitude >= -180 && s.Longitude <= 180 &&
		s.AltitudeMeters >= -500 && s.AltitudeMeters <= 9000
}

func LoadStationFile(path string) (Station, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Station{}, false
	}
	var station Station
	if json.Unmarshal(data, &station) != nil || !station.Valid() {
		return Station{}, false
	}
	return station, true
}

func SaveStationFile(path string, station Station) error {
	if strings.TrimSpace(station.Name) == "" {
		station.Name = "Home"
	}
	data, err := json.MarshalIndent(station, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
