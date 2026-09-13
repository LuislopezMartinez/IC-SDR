package screens

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/internal/radiosonde"
	"go-zero/internal/sdr"
	"go-zero/simpleui"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRadiosondeToolPreservesBandAndFrequency(t *testing.T) {
	for _, saved := range []int64{0, 14_261_000, 402_900_000} {
		r := sdr.NewReceiver(sdr.Config{SampleRate: 2_048_000, FrequencyHz: 14_261_000, FFTSize: 4096})
		s := NewMainScreen(r)
		s.frequencyHz = 14_261_000
		s.centerFrequencyHz = 14_261_000
		s.spanHz = 100_000
		s.bandCategory = "HAM"
		s.bandName = "20 m"
		s.activeTool = "WATERFALL"
		s.radiosondeFrequencyHz = saved
		s.mode = simpleui.NewDropdown("mode", 0, 0, 100, 40, "MODE", []string{"USB", "NFM"}, 12)
		s.band = simpleui.NewButton("band", 0, 0, 100, 40, "BAND  20 m", 12)
		s.bandSelector = NewBandSelector("HAM", "20 m", nil)
		s.radiosondePanel = NewRadiosondePanel(s)
		s.selectTool("RADIOSONDE")
		if s.mode.SelectedText() != "NFM" || s.savedMode != "NFM" {
			t.Fatalf("selecting radiosonde did not select NFM: mode=%q saved=%q", s.mode.SelectedText(), s.savedMode)
		}
		if s.frequencyHz != 14_261_000 || s.centerFrequencyHz != 14_261_000 || r.CenterFrequency() != 14_261_000 {
			t.Fatalf("selecting radiosonde changed tuning: dial=%d center=%d receiver=%d", s.frequencyHz, s.centerFrequencyHz, r.CenterFrequency())
		}
		if s.bandName != "20 m" || s.band.Label() != "BAND  20 m" || s.bandSelector.selectedName != "20 m" {
			t.Fatal("selecting radiosonde changed the band")
		}
		s.selectTool("WATERFALL")
		if s.frequencyHz != 14_261_000 || r.CenterFrequency() != 14_261_000 || s.bandName != "20 m" || s.bandSelector.selectedName != "20 m" {
			t.Fatal("previous tuning not restored")
		}
	}
}

func TestRadiosondeEnterSelectsNFMFilterWithoutRetuning(t *testing.T) {
	s := NewMainScreen(nil)
	s.frequencyHz = 402_900_000
	s.centerFrequencyHz = 402_950_000
	s.mode = simpleui.NewDropdown("mode", 0, 0, 100, 40, "MODE", []string{"USB", "NFM"}, 12)
	s.filterSelector = NewFilterSelector(s.selectFilter)
	s.demodBandwidthHz = 2_400
	p := NewRadiosondePanel(s)
	p.Enter()
	if s.mode.SelectedText() != "NFM" || s.demodBandwidthHz != s.filterSelector.Current("NFM").BandwidthHz {
		t.Fatalf("radiosonde mode/filter = %q/%d", s.mode.SelectedText(), s.demodBandwidthHz)
	}
	if s.frequencyHz != 402_900_000 || s.centerFrequencyHz != 402_950_000 {
		t.Fatalf("radiosonde entry retuned dial: %d/%d", s.frequencyHz, s.centerFrequencyHz)
	}
}

func TestRadiosondeMenuBounds(t *testing.T) {
	menu := NewToolMenu("RADIOSONDE", nil)
	if !validTool("RADIOSONDE") {
		t.Fatal("radiosonde tool is not registered")
	}
	for i := range toolMenuItems {
		a := menu.itemBounds(i)
		if a.X < 190 || a.X+a.Width > 1090 {
			t.Fatalf("tool %s outside menu: %+v", toolMenuItems[i].id, a)
		}
		for j := 0; j < i; j++ {
			b := menu.itemBounds(j)
			if a.X < b.X+b.Width && b.X < a.X+a.Width && a.Y < b.Y+b.Height && b.Y < a.Y+a.Height {
				t.Fatalf("tools %d and %d overlap", i, j)
			}
		}
	}
}

func TestRadiosondeSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := writeAppSettings(path, persistedAppSettings{Version: appSettingsVersion, RadiosondeFamily: "M10/M20", RadiosondeFrequencyHz: 402900000, ActiveTool: "RADIOSONDE"}); err != nil {
		t.Fatal(err)
	}
	s := NewMainScreen(nil)
	loadAppSettings(path, s)
	p := NewRadiosondePanel(s)
	if p.family != "M10/M20" || p.targetHz != 402900000 || p.enabled || s.activeTool != "RADIOSONDE" {
		t.Fatalf("bad restored radiosonde state: %+v", p)
	}
}

func TestRadiosondeTuneButtonUsesSavedFrequency(t *testing.T) {
	r := sdr.NewReceiver(sdr.Config{SampleRate: 2_048_000, FrequencyHz: 14_261_000, FFTSize: 4096})
	s := NewMainScreen(r)
	s.frequencyHz = 14_261_000
	s.centerFrequencyHz = 14_261_000
	s.radiosondeFrequencyHz = 402_900_000
	s.mode = simpleui.NewDropdown("mode", 0, 0, 100, 40, "MODE", []string{"NFM"}, 12)
	p := NewRadiosondePanel(s)
	if p.tuneButton.Label() != "SINTONIZAR 402.900 MHz" {
		t.Fatalf("tune button label = %q", p.tuneButton.Label())
	}
	p.Enter()
	if s.frequencyHz != 14_261_000 {
		t.Fatal("entering radiosonde tool tuned without clicking")
	}
	p.tuneRecommended()
	if s.frequencyHz != 402_900_000 || s.centerFrequencyHz != 402_900_000 || r.CenterFrequency() != 402_900_000 || p.targetHz != 402_900_000 {
		t.Fatalf("button did not tune receiver and decoder target: dial=%d center=%d receiver=%d target=%d", s.frequencyHz, s.centerFrequencyHz, r.CenterFrequency(), p.targetHz)
	}
}

// Optional visual QA renders the actual panel and menu to a hidden window.
func TestRadiosondeRender(t *testing.T) {
	dir := os.Getenv("RADIOSONDE_RENDER_DIR")
	if dir == "" {
		t.Skip("visual QA not requested")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetConfigFlags(rl.FlagWindowHidden)
	rl.InitWindow(1600, 900, "Radiosonde visual QA")
	defer rl.CloseWindow()
	simpleui.SetTextScale(1.25)
	p := NewRadiosondePanel(NewMainScreen(nil))
	p.SetVisible(true)
	line := []byte(`{"type":"RS41","id":"K1930308","frame":5047,"lat":45.66939,"lon":15.87963,"alt":28527.16,"vel_h":9.8,"vel_v":6.1,"temp":-52.7,"humidity":12.1,"ref_position":"GPS"}`)
	e, _ := radiosonde.ParseEvent(line)
	canvas := rl.LoadRenderTexture(1600, 900)
	defer rl.UnloadRenderTexture(canvas)
	rl.BeginTextureMode(canvas)
	rl.ClearBackground(colors.background)
	drawPanel(24, toolY, 1552, toolH)
	p.enabled = true
	p.style()
	p.drawTelemetry(radiosonde.Status{State: "AUTO · RS41", Running: true}, []radiosonde.Event{e})
	for _, c := range p.controls {
		c.Draw()
	}
	rl.EndTextureMode()
	panelImage := rl.LoadImageFromTexture(canvas.Texture)
	rl.ImageFlipVertical(panelImage)
	if !rl.ExportImage(*panelImage, filepath.Join(dir, "radiosonde-panel.png")) {
		t.Fatal("panel image export failed")
	}
	rl.UnloadImage(panelImage)
	rl.BeginTextureMode(canvas)
	menu := NewToolMenu("RADIOSONDE", nil)
	menu.Open()
	menu.DrawOverlay()
	rl.EndTextureMode()
	img := rl.LoadImageFromTexture(canvas.Texture)
	defer rl.UnloadImage(img)
	rl.ImageFlipVertical(img)
	if !rl.ExportImage(*img, filepath.Join(dir, "radiosonde-ui.png")) {
		t.Fatal("image export failed")
	}
}
