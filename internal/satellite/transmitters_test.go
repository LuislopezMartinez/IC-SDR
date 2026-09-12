package satellite

import (
	"strings"
	"testing"
)

func TestParseSatnogsTransmittersFillsAmateurDownlinks(t *testing.T) {
	data := []byte(`[
		{"description":"FM Voice","status":"active","mode":"FM","norad_cat_id":27607,"downlink_low":436795000,"alive":true},
		{"description":"Dead","status":"invalid","mode":"FM","norad_cat_id":27607,"downlink_low":145900000,"alive":false},
		{"description":"APT","status":"active","mode":"AFSK","norad_cat_id":25338,"downlink_low":137620000,"alive":true},
		{"description":"NB transponder","status":"active","mode":"USB","norad_cat_id":43700,"downlink_low":10489500000,"downlink_high":10490000000,"alive":true}
	]`)
	table, err := parseSatnogsTransmitters(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(table[27607]) != 1 || table[27607][0].DownlinkHz != 436795000 {
		t.Fatalf("SO-50: %+v", table[27607])
	}
	if len(table[25338]) != 1 || table[25338][0].DownlinkHz != 137620000 {
		t.Fatalf("NOAA 15: %+v", table[25338])
	}
	if len(table[43700]) != 1 || table[43700][0].DownlinkHz != 10489750000 {
		t.Fatalf("QO-100 midpoint: %+v", table[43700])
	}
}

func TestAttachSignalsMergesKnownAndCatalogued(t *testing.T) {
	tracker := &Tracker{transmitters: builtinTransmitters(), satellites: []Satellite{{Name: "NOAA 15", NORAD: 25338}, {Name: "ISS (ZARYA)", NORAD: 25544}}}
	tracker.attachSignalsLocked()
	if len(tracker.satellites[0].Signals) == 0 || tracker.satellites[0].Signals[0].DownlinkHz != 137620000 {
		t.Fatalf("NOAA 15 missing APT: %+v", tracker.satellites[0].Signals)
	}
	if len(tracker.satellites[1].Signals) < 2 {
		t.Fatalf("ISS missing transponders: %+v", tracker.satellites[1].Signals)
	}
}

func TestDemodForSignal(t *testing.T) {
	if DemodForSignal("SSB/CW") != "USB" {
		t.Fatal(DemodForSignal("SSB/CW"))
	}
	if DemodForSignal("APT") != "AM" {
		t.Fatal(DemodForSignal("APT"))
	}
	if DemodForSignal("FM") != "NFM" {
		t.Fatal(DemodForSignal("FM"))
	}
}

func TestFormatSignalLabelIncludesFrequency(t *testing.T) {
	label := FormatSignalLabel(Signal{Name: "Voice / SSTV", Mode: "FM", DownlinkHz: 145800000})
	if !strings.Contains(label, "145.800") || !strings.Contains(label, "FM") {
		t.Fatal(label)
	}
}
