package screens

import "testing"

func TestTETRAPOLUHFChannelFrequency(t *testing.T) {
	got, err := tetrapolChannelFrequency("UHF", 390_000_000, 2882)
	if err != nil || got != 394_425_000 {
		t.Fatalf("channel 2882 = %d, %v; want 394425000", got, err)
	}
	if _, err := tetrapolChannelFrequency("VHF", 80_000_000, 100); err == nil {
		t.Fatal("deployment-specific VHF plan was guessed")
	}
	if _, err := tetrapolChannelFrequency("UHF", 390_000_000, 4096); err == nil {
		t.Fatal("invalid channel was accepted")
	}
}

func TestTETRAPOLTenKHzChannelPlan(t *testing.T) {
	got, err := tetrapolChannelFrequencyForPlan(tetrapolPlanUHF10, 390_000_000, 2882)
	if err != nil || got != 397_465_000 {
		t.Fatalf("10 kHz channel 2882 = %d, %v; want 397465000", got, err)
	}
	if got, err = tetrapolChannelFrequencyForPlan(tetrapolPlanUHF125, 390_000_000, 2882); err != nil || got != 394_425_000 {
		t.Fatalf("12.5 kHz channel 2882 = %d, %v; want 394425000", got, err)
	}
}
