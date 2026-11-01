package screens

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go-zero/internal/worldclock"
)

func TestWorldClockPanelConsumesMapResultAndPersistsLocation(t *testing.T) {
	directory := t.TempDir()
	p := &WorldClockPanel{
		config:     worldclock.Defaults(),
		configPath: filepath.Join(directory, "world-clocks.json"),
		resultPath: filepath.Join(directory, "map-result.json"),
	}
	result := worldClockMapResult{Clock: 2, Location: worldclock.Location{
		City: "Buenos Aires", Country: "Argentina", Lat: -34.6037, Lon: -58.3816,
	}}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(p.resultPath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	p.Tick()

	if got := p.config.Locations[2]; got.City != "Buenos Aires" || got.Zone != "America/Argentina/Buenos_Aires" {
		t.Fatalf("map result was not applied: %+v", got)
	}
	if _, err = os.Stat(p.resultPath); !os.IsNotExist(err) {
		t.Fatalf("consumed result was not removed: %v", err)
	}
	loaded := worldclock.Load(p.configPath)
	if loaded.Locations[2].City != "Buenos Aires" {
		t.Fatalf("updated location was not persisted: %+v", loaded.Locations[2])
	}
}
