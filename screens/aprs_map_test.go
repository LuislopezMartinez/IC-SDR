package screens

import (
	"encoding/json"
	"go-zero/internal/aprs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAPRSMapSnapshotUpdatesAndClear(t *testing.T) {
	p := &APRSPanel{mapPath: filepath.Join(t.TempDir(), "stations.json"), stations: map[string]aprs.MapStation{}}
	now := time.Now()
	packet := aprs.Packet{Source: "EA1ABC", Received: now, Coordinates: "40,-3", Latitude: 40, Longitude: -3, Symbol: "/-"}
	p.writeMapSnapshot([]aprs.Packet{packet})
	data, err := os.ReadFile(p.mapPath)
	if err != nil {
		t.Fatal(err)
	}
	var list []aprs.MapStation
	if json.Unmarshal(data, &list) != nil || len(list) != 1 {
		t.Fatal("missing station snapshot")
	}
	packet.Received = now.Add(time.Second)
	packet.Latitude = 41
	p.writeMapSnapshot([]aprs.Packet{packet})
	data, _ = os.ReadFile(p.mapPath)
	_ = json.Unmarshal(data, &list)
	if len(list) != 1 || list[0].Latitude != 41 {
		t.Fatal("duplicate or stale station")
	}
	p.stations = map[string]aprs.MapStation{}
	p.writeMapSnapshot(nil)
	data, _ = os.ReadFile(p.mapPath)
	_ = json.Unmarshal(data, &list)
	if len(list) != 0 {
		t.Fatal("clear retained markers")
	}
}
func TestAPRSIconTables(t *testing.T) {
	for code, kind := range map[string]string{"/>": "Vehículo", "/-": "Estación fija", "/_": "Meteorología", "/[": "Persona", "/s": "Barco", "\\[": "Estación", "1>": "Vehículo"} {
		if aprsIconKind(code) != kind {
			t.Fatalf("wrong kind for %q", code)
		}
	}
}
