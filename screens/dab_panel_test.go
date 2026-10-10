package screens

import (
	"testing"

	"go-zero/internal/sdr"
	"go-zero/simpleui"
)

func TestDABPanelControlsStayInBottomToolWorkspace(t *testing.T) {
	screen := NewMainScreen(nil)
	panel := NewDABPanel(screen)
	for _, control := range panel.controls {
		bounds := control.Bounds()
		if bounds.X < toolContentX || bounds.X+bounds.Width > toolContentRight {
			t.Fatalf("DAB control %s outside horizontal tool workspace: %+v", control.ID(), bounds)
		}
		if bounds.Y < toolY || bounds.Y+bounds.Height > toolY+toolH {
			t.Fatalf("DAB control %s outside bottom tool workspace: %+v", control.ID(), bounds)
		}
	}
	if panel.previous.Label() != "<" || panel.next.Label() != ">" {
		t.Fatalf("channel navigation labels = %q / %q", panel.previous.Label(), panel.next.Label())
	}
}

func TestDABTuneUsesSelectedBandIIIChannel(t *testing.T) {
	screen := NewMainScreen(nil)
	panel := NewDABPanel(screen)
	panel.channelIndex = 27
	panel.tuneSelected()
	if screen.frequencyHz != 222_064_000 || screen.centerFrequencyHz != 222_064_000 {
		t.Fatalf("channel 11D tuned %d / %d", screen.frequencyHz, screen.centerFrequencyHz)
	}
	if screen.spanHz != 2_048_000 {
		t.Fatalf("DAB span = %d, want 2048000", screen.spanHz)
	}
}

func TestSelectingDABToolPublishesDABPlusInHeader(t *testing.T) {
	screen := NewMainScreen(nil)
	screen.mode = simpleui.NewDropdown("dabTestMode", 0, 0, 100, 30, "MODE", []string{"USB", "DAB+"}, 12)
	screen.filter = simpleui.NewButton("dabTestFilter", 0, 0, 100, 30, "", 12)
	screen.filterSelector = NewFilterSelector(screen.selectFilter)
	panel := NewDABPanel(screen)
	panel.Enter()
	if got := screen.mode.SelectedText(); got != "DAB+" {
		t.Fatalf("header mode = %q, want DAB+", got)
	}
	if got := screen.filter.Label(); got != "DAB  1536 kHz" {
		t.Fatalf("header filter = %q, want DAB multiplex bandwidth", got)
	}
	for _, control := range panel.controls {
		if control.ID() == "dabStop" || control.ID() == "dabDetails" {
			t.Fatalf("redundant DAB control is still present: %s", control.ID())
		}
	}
}

func TestDABOffRestoresPreviousAnalogAudioMode(t *testing.T) {
	receiver := sdr.NewReceiver(sdr.Config{FrequencyHz: 223_936_000, SampleRate: 2_048_000, FFTSize: 4096})
	receiver.SetDemodulator("USB", 223_936_000, 2_400)
	screen := NewMainScreen(receiver)
	screen.frequencyHz = 223_936_000
	panel := NewDABPanel(screen)
	panel.analogMode, panel.analogBW = "USB", 2_400
	panel.restoreAnalogAudio()
	mode, _ := receiver.AudioPlaybackState()
	if mode != "USB" {
		t.Fatalf("decoder off restored %q, want USB", mode)
	}
}
