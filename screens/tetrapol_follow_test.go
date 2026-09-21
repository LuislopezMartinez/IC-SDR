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
