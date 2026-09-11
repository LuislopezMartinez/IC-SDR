package screens

import "testing"

func TestNormalizeLegacyDMRMemoryMode(t *testing.T) {
	panel := &MemoryPanel{memories: []MemoryEntry{{Name: "DMR BCN 01", Mode: "NFM", FilterBandwidthHz: 11_500}}}
	if !panel.normalizeMemoryModes() {
		t.Fatal("expected legacy DMR memory to be repaired")
	}
	if got := panel.memories[0].Mode; got != "DMR BETA" {
		t.Fatalf("mode=%q, want DMR BETA", got)
	}
	if got := panel.memories[0].FilterBandwidthHz; got != 11_500 {
		t.Fatalf("bandwidth=%d, expected compatible stored bandwidth", got)
	}
}

func TestNormalizeExplicitDMRUsesSafeBandwidth(t *testing.T) {
	panel := &MemoryPanel{memories: []MemoryEntry{{Name: "Digital channel", Mode: "DMR", FilterBandwidthHz: 2400}}}
	panel.normalizeMemoryModes()
	if panel.memories[0].Mode != "DMR BETA" || panel.memories[0].FilterBandwidthHz != 12_500 {
		t.Fatalf("unexpected normalized memory: %+v", panel.memories[0])
	}
}
