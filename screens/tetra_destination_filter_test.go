package screens

import (
	"path/filepath"
	"reflect"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestParseTETRADestinations(t *testing.T) {
	got, err := parseTETRADestinations("1001, 5000\t6009004  1001, 16777215")
	if err != nil {
		t.Fatal(err)
	}
	want := []uint32{1001, 5000, 6009004, 16777215}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("destinations=%v, want %v", got, want)
	}
}

func TestParseTETRADestinationsRejectsMalformedValues(t *testing.T) {
	for _, value := range []string{"50-00", "1001abc", "1001;5000", "0", "16777216", "4294967296"} {
		if got, err := parseTETRADestinations(value); err == nil {
			t.Errorf("parseTETRADestinations(%q)=%v, want error", value, got)
		}
	}
}

func TestTETRADestinationSettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tetra-viewer.json")
	written := &TETRAPanel{
		viewerSettingsPath:       path,
		viewerTopmost:            true,
		destinationFilterEnabled: true,
		destinationFilterText:    "1001, 5000",
	}
	written.writeViewerSettings()

	loaded := &TETRAPanel{viewerSettingsPath: path}
	loaded.loadViewerSettings()
	if !loaded.viewerTopmost || !loaded.destinationFilterEnabled || loaded.destinationFilterText != "1001, 5000" {
		t.Fatalf("unexpected loaded settings: topmost=%v enabled=%v destinations=%q", loaded.viewerTopmost, loaded.destinationFilterEnabled, loaded.destinationFilterText)
	}
}

func TestTETRADestinationControlsStayInsideToolPanel(t *testing.T) {
	panel := NewTETRAPanel(&MainScreen{})
	controls := []struct {
		name   string
		bounds rl.Rectangle
	}{
		{"field", panel.destinationField.Bounds()},
		{"switch", panel.destinationSwitch.Bounds()},
	}
	for _, control := range controls {
		name, bounds := control.name, control.bounds
		if bounds.X < toolContentX || bounds.X+bounds.Width > toolContentRight || bounds.Y < toolY || bounds.Y+bounds.Height > toolY+toolH {
			t.Fatalf("destination %s is outside TETRA panel: %+v", name, bounds)
		}
	}
}
