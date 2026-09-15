package aprs

import (
	"math"
	"testing"
	"time"
)

func TestMapStationUpdatesKeepPositionAndTelemetry(t *testing.T) {
	stations := map[string]MapStation{}
	now := time.Now()
	position := Packet{Source: "EA1ABC-9", Received: now, Coordinates: "40,-3", Latitude: 40, Longitude: -3, Symbol: "/>", Temperature: "—"}
	telemetry := Packet{Source: "EA1ABC-9", Received: now.Add(time.Second), Coordinates: "—", Telemetry: "T#001,42", Temperature: "21 C", Symbol: "—"}
	UpdateMapStations(stations, []Packet{telemetry, position})
	s := stations["EA1ABC-9"]
	if len(stations) != 1 || !s.HasPosition || s.Latitude != 40 || s.Symbol != "/>" || s.Telemetry != "T#001,42" || !s.PositionTime.Equal(now) {
		t.Fatalf("wrong station: %+v", s)
	}
	moved := position
	moved.Received = now.Add(2 * time.Second)
	moved.Latitude = 41
	UpdateMapStations(stations, []Packet{moved, telemetry, position})
	s = stations["EA1ABC-9"]
	if s.Latitude != 41 || s.Temperature != "21 C" || s.Telemetry != "T#001,42" {
		t.Fatalf("lost state: %+v", s)
	}
	invalid := moved
	invalid.Received = now.Add(3 * time.Second)
	invalid.Latitude = math.NaN()
	UpdateMapStations(stations, []Packet{invalid})
	if stations["EA1ABC-9"].Latitude != 41 {
		t.Fatal("invalid position replaced last valid one")
	}
	UpdateMapStations(stations, []Packet{{Source: "EA2XYZ", Received: now, Coordinates: "—"}})
	if len(MapStationList(stations)) != 1 {
		t.Fatal("unpositioned station appeared on map")
	}
	zero := position
	zero.Source = "ZERO"
	zero.Latitude = 0
	zero.Longitude = 0
	zero.Coordinates = "0,0"
	UpdateMapStations(stations, []Packet{zero})
	if !stations["ZERO"].HasPosition {
		t.Fatal("valid zero coordinates rejected")
	}
}
