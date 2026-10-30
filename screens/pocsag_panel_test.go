package screens

import (
	"testing"

	"go-zero/internal/pocsag"
)

func TestPOCSAGPresetRecentersAndUsesNarrowFM(t *testing.T) {
	screen := &MainScreen{frequencyHz: 14_261_000, centerFrequencyHz: 14_261_000, spanHz: 100_000}
	panel := NewPOCSAGPanel(screen)
	panel.selectFrequency(169_625_000)
	if screen.frequencyHz != 169_625_000 || screen.centerFrequencyHz != 169_625_000 {
		t.Fatalf("preset did not retune and recenter: frequency=%d center=%d", screen.frequencyHz, screen.centerFrequencyHz)
	}
	if screen.demodBandwidthHz != 12_500 {
		t.Fatalf("unexpected POCSAG bandwidth: %d", screen.demodBandwidthHz)
	}
	if screen.spanHz < 500_000 {
		t.Fatalf("preset did not expose enough spectrum: %d", screen.spanHz)
	}
}

func TestPagingProtocolSelection(t *testing.T) {
	panel := NewPOCSAGPanel(&MainScreen{})
	panel.selectProtocol(pocsag.ProtocolFLEX)
	if panel.protocol != pocsag.ProtocolFLEX || len(panel.protocolButtons) != 2 {
		t.Fatalf("FLEX protocol was not selected: %s", panel.protocol)
	}
}

func TestPagingProtocolsHaveIndependentPresetDefaults(t *testing.T) {
	pocsagPresets := defaultPagingPresets(pocsag.ProtocolPOCSAG)
	flexPresets := defaultPagingPresets(pocsag.ProtocolFLEX)
	if len(pocsagPresets) != 5 || len(flexPresets) != 5 || pocsagPresets[0].FrequencyHz == flexPresets[0].FrequencyHz {
		t.Fatalf("pager preset families are not independent: POCSAG=%+v FLEX=%+v", pocsagPresets, flexPresets)
	}
}
