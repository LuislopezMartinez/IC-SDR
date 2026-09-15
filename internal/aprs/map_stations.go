package aprs

import (
	"math"
	"sort"
	"strings"
	"time"
)

type MapStation struct {
	Packet
	PositionTime time.Time `json:"positionTime"`
	HasPosition  bool      `json:"hasPosition"`
}

func ValidMapPosition(p Packet) bool {
	return p.Coordinates != "" && p.Coordinates != "—" && !math.IsNaN(p.Latitude) && !math.IsNaN(p.Longitude) && !math.IsInf(p.Latitude, 0) && !math.IsInf(p.Longitude, 0) && math.Abs(p.Latitude) <= 90 && math.Abs(p.Longitude) <= 180
}

// UpdateMapStations processes history chronologically; telemetry keeps the last position.
func UpdateMapStations(stations map[string]MapStation, packets []Packet) {
	ordered := append([]Packet(nil), packets...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Received.Before(ordered[j].Received) })
	for _, p := range ordered {
		key := strings.ToUpper(strings.TrimSpace(p.Source))
		if key == "" {
			continue
		}
		s, exists := stations[key]
		if exists && !p.Received.After(s.Received) {
			continue
		}
		s.Source = key
		s.Received = p.Received
		s.Raw = p.Raw
		s.Information = p.Information
		s.Type = p.Type
		fields := []struct {
			dst   *string
			value string
		}{{&s.Summary, p.Summary}, {&s.Course, p.Course}, {&s.Speed, p.Speed}, {&s.Altitude, p.Altitude}, {&s.Temperature, p.Temperature}, {&s.Humidity, p.Humidity}, {&s.Pressure, p.Pressure}, {&s.Wind, p.Wind}, {&s.Rain, p.Rain}, {&s.Telemetry, p.Telemetry}}
		for _, f := range fields {
			if f.value != "" && f.value != "—" {
				*f.dst = f.value
			}
		}
		if ValidMapPosition(p) {
			s.Latitude = p.Latitude
			s.Longitude = p.Longitude
			s.Coordinates = p.Coordinates
			s.PositionTime = p.Received
			s.HasPosition = true
			if len(p.Symbol) == 2 {
				s.Symbol = p.Symbol
			}
		}
		stations[key] = s
	}
}

func MapStationList(stations map[string]MapStation) []MapStation {
	out := make([]MapStation, 0, len(stations))
	for _, s := range stations {
		if s.HasPosition {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Source < out[j].Source })
	return out
}
