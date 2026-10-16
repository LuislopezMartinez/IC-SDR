package screens

import "testing"

func TestSSTVExposesAllFourDecoderCandidates(t *testing.T) {
	screen := &MainScreen{sstvAutomatic: true, sstvMode: "R36", sstvCandidateModes: [4]string{"R36", "R72", "M1", "S1"}}
	panel := NewSSTVPanel(screen)
	if len(panel.candidateModes) != 4 {
		t.Fatalf("manual selector count = %d, want 4", len(panel.candidateModes))
	}
	for i, dropdown := range panel.candidateModes {
		if got, want := dropdown.SelectedText(), screen.sstvCandidateModes[i]; got != want {
			t.Fatalf("RX%d mode = %q, want %q", i+2, got, want)
		}
		if dropdown.Bounds().Y < 458 || dropdown.Bounds().Y >= toolY+toolH {
			t.Fatalf("RX%d selector is outside expanded SSTV workspace: %+v", i+2, dropdown.Bounds())
		}
	}
}

func TestSSTVKeepsFFTButSuppressesWaterfall(t *testing.T) {
	screen := &MainScreen{activeTool: "SSTV"}
	if screen.shouldDrawWaterfall() {
		t.Fatal("SSTV should reserve the waterfall area for its image workspace")
	}
	screen.activeTool = "FFT DISPLAY"
	if !screen.shouldDrawWaterfall() {
		t.Fatal("ordinary tools should keep the waterfall visible")
	}
}
