package sdr

import (
	"slices"
	"testing"
)

func TestTETRAPOLRuntimeArgumentsPreserveVHFUplink(t *testing.T) {
	channel, band, direction, args := tetrapolRuntimeArguments("CCH", "VHF", "UP")
	if channel != "CCH" || band != "VHF" || direction != "UP" {
		t.Fatalf("normalized config = %s/%s/%s", channel, band, direction)
	}
	want := []string{"-b", "VHF", "-t", "CCH", "-d", "UP"}
	if !slices.Equal(args, want) {
		t.Fatalf("runtime args = %v; want %v", args, want)
	}
}

func TestTETRAPOLRuntimeArgumentsUseSafeDefaults(t *testing.T) {
	_, band, direction, args := tetrapolRuntimeArguments("invalid", "invalid", "invalid")
	if band != "UHF" || direction != "DOWN" || !slices.Equal(args, []string{"-b", "UHF", "-t", "TCH", "-d", "DOWN"}) {
		t.Fatalf("default runtime config = %s/%s %v", band, direction, args)
	}
}
