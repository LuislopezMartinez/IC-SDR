package screens

import (
	"testing"
	"time"

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

func TestPagingSelectionSurvivesNewMessages(t *testing.T) {
	selectedAt := time.Unix(100, 1)
	panel := &POCSAGPanel{
		selected:   0,
		selectedAt: selectedAt,
		status:     pocsag.Status{Events: []pocsag.Event{{Received: selectedAt, Address: 1}}},
	}
	panel.status.Events = append([]pocsag.Event{{Received: time.Unix(101, 1), Address: 2}}, panel.status.Events...)
	panel.reconcileSelection()
	if panel.selected != 1 || panel.status.Events[panel.selected].Address != 1 {
		t.Fatalf("selection moved to a new message: index=%d events=%+v", panel.selected, panel.status.Events)
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
