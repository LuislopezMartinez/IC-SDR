package screens

import "testing"

func TestNFMCustomFilterRange(t *testing.T) {
	minimum, maximum, step := customFilterRange("NFM")
	if minimum != 500 || maximum != 25_000 || step != 500 {
		t.Fatalf("NFM custom range = %d..%d step %d; want 500..25000 step 500", minimum, maximum, step)
	}
}

func TestTETRAPOLFilterDefaultsTo12500Hz(t *testing.T) {
	selector := NewFilterSelector(nil)
	got := selector.Current(tetrapolToolID)
	if got.BandwidthHz != 12_500 {
		t.Fatalf("TETRAPOL filter = %d Hz; want 12500 Hz", got.BandwidthHz)
	}
}

func TestUnknownFilterModeFallsBackWithoutPanicking(t *testing.T) {
	selector := NewFilterSelector(nil)
	got := selector.Current("UNKNOWN MODE")
	if got.BandwidthHz <= 0 {
		t.Fatalf("fallback filter = %d Hz; want a valid bandwidth", got.BandwidthHz)
	}
}
