package screens

import (
	"testing"
	"time"

	"go-zero/internal/omnirig"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestRigSelectorLabelIncludesNumberAndModel(t *testing.T) {
	if got := rigSelectorLabel(2, omnirig.RigSummary{RigType: "FT-991A"}); got != "RIG 2 · FT-991A" {
		t.Fatalf("selector label = %q", got)
	}
	if got := rigSelectorLabel(1, omnirig.RigSummary{}); got != "RIG 1 · SIN CONFIGURAR" {
		t.Fatalf("empty selector label = %q", got)
	}
}

func TestRigTXMuteIsDebouncedAndRestoresManualState(t *testing.T) {
	screen := NewMainScreen(nil)
	screen.rigMuteOnTX = true
	screen.muteSwitch = simpleui.NewSwitch("mute-test", 0, 0, 160, 28, "MUTE", false, 12)
	screen.volumeLabel = simpleui.NewLabel("volume-test", 0, 0, 100, 20, "VOL", 12)
	screen.volumeSlider = simpleui.NewSlider("volume-slider-test", 0, 0, 100, 20, 0, 100, 50)
	tx := omnirig.State{Online: true, TXReadable: true, Transmitting: true}

	screen.updateRigTXMute(tx)
	if screen.rigMuteApplied {
		t.Fatal("TX mute was applied without debounce")
	}
	screen.rigTXSince = time.Now().Add(-100 * time.Millisecond)
	screen.updateRigTXMute(tx)
	if !screen.rigMuteApplied || screen.muteSwitch.Label() != "RIG → MUTE" || !screen.muteSwitch.Active() {
		t.Fatalf("CAT mute presentation not applied: applied=%v label=%q", screen.rigMuteApplied, screen.muteSwitch.Label())
	}

	screen.muted = true // The user had also requested manual mute.
	screen.updateRigTXMute(omnirig.State{Online: true, TXReadable: true, Transmitting: false})
	if screen.rigMuteApplied || !screen.muteSwitch.Active() || screen.muteSwitch.Label() == "RIG → MUTE" {
		t.Fatalf("RX did not restore manual mute: applied=%v active=%v label=%q", screen.rigMuteApplied, screen.muteSwitch.Active(), screen.muteSwitch.Label())
	}
}

func TestRigSelectorsAreLargeAndRightAligned(t *testing.T) {
	panel := NewOmniRigPanel(NewMainScreen(nil))
	for number, button := range map[int]interface{ Bounds() rl.Rectangle }{1: panel.rig1, 2: panel.rig2} {
		bounds := button.Bounds()
		if bounds.X < 1200 || bounds.Width < 175 || bounds.Height < 170 {
			t.Fatalf("RIG %d selector is not a large right-hand button: %+v", number, bounds)
		}
		if bounds.X+bounds.Width > toolContentRight {
			t.Fatalf("RIG %d selector exceeds tool panel: %+v", number, bounds)
		}
	}
}

func TestTuneFromRigPreservesFixedCenterInsideSpan(t *testing.T) {
	screen := NewMainScreen(nil)
	screen.frequencyHz = 145_500_000
	screen.centerFrequencyHz = 145_500_000
	screen.spanHz = 500_000
	screen.centerMode = false
	screen.demodBandwidthHz = 12_500

	screen.tuneFromRig(145_600_000)

	if screen.frequencyHz != 145_600_000 {
		t.Fatalf("frequency = %d", screen.frequencyHz)
	}
	if screen.centerFrequencyHz != 145_500_000 {
		t.Fatalf("FIX center moved unnecessarily to %d", screen.centerFrequencyHz)
	}
}

func TestTuneFromRigRecentersCenterMode(t *testing.T) {
	screen := NewMainScreen(nil)
	screen.frequencyHz = 145_500_000
	screen.centerFrequencyHz = 145_500_000
	screen.spanHz = 500_000
	screen.centerMode = true

	screen.tuneFromRig(145_625_000)

	if screen.frequencyHz != 145_625_000 || screen.centerFrequencyHz != 145_625_000 {
		t.Fatalf("CENTER did not follow RIG: tuned=%d center=%d", screen.frequencyHz, screen.centerFrequencyHz)
	}
}
