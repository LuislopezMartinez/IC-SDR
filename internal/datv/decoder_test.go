package datv

import (
	"reflect"
	"testing"
)

func TestArguments(t *testing.T) {
	got := Arguments(2048000, Settings{Standard: "DVB-S", SymbolRate: 333000, OffsetHz: -12000, FEC: "3/4", Inverted: true})
	want := []string{"--f32", "--sr", "333000", "--sampler", "rrc", "--standard", "DVB-S", "--derotate", "-12000", "--drift", "--fastlock", "--fd-info", "2", "--fd-const", "2", "--json", "-f", "2048000", "--cnr", "--cr", "3/4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestSymbolPoints(t *testing.T) {
	points, ok := parseSymbolPoints(`SYMBOLS [[100,-100],[-50,25]]`)
	if !ok || len(points) != 2 || points[0].I != 1 || points[0].Q != -1 || points[1].I != -.5 || points[1].Q != .25 {
		t.Fatalf("unexpected points: %#v ok=%v", points, ok)
	}
}

func TestAFCRequiresLockAndConverges(t *testing.T) {
	front := newChannelizer(2_048_000, 125_000, 10_000)
	d := &Decoder{channelizer: front, symbolRate: 125_000}
	d.status.FrequencyErrorHz = 1000
	for range 4 {
		d.updateAFCLocked()
	}
	if d.status.AFCCorrectionHz != 0 {
		t.Fatal("AFC corrected without lock")
	}
	d.status.Locked = true
	for range 4 {
		d.updateAFCLocked()
	}
	if d.status.AFCCorrectionHz <= 0 || d.status.AFCState != "corrigiendo" {
		t.Fatalf("correction=%f state=%q", d.status.AFCCorrectionHz, d.status.AFCState)
	}
}

func TestAutoPlanStaysOnSelectedChannel(t *testing.T) {
	original := Settings{Standard: "AUTO", SymbolRate: 250_000, BandwidthHz: 400_000, OffsetHz: 123_456, Inverted: false}
	plan := autoPlan(original)
	if len(plan) != 8 {
		t.Fatalf("attempts=%d want=8", len(plan))
	}
	for index, candidate := range plan {
		if candidate.OffsetHz != original.OffsetHz {
			t.Fatalf("attempt %d changed selected channel", index)
		}
		if candidate.Standard == "AUTO" {
			t.Fatalf("attempt %d retained AUTO standard", index)
		}
	}
	if plan[0].Standard != "DVB-S2" || plan[0].SymbolRate != 250_000 || plan[0].Inverted {
		t.Fatalf("unexpected first attempt: %#v", plan[0])
	}
	// 400 kHz / 1.2 is closest to the common 333 kS/s DATV rate.
	if plan[4].SymbolRate != 333_000 {
		t.Fatalf("estimated rate=%d", plan[4].SymbolRate)
	}
}

func TestAutoPlanIncludesRateEstimatedFromSelectionWidth(t *testing.T) {
	plan := autoPlan(Settings{Standard: "AUTO", SymbolRate: 333_000, BandwidthHz: 360_000})
	found := false
	for _, candidate := range plan {
		found = found || candidate.SymbolRate == 300_000
	}
	if !found {
		t.Fatal("missing 300 kS/s rate estimated from 360 kHz selection")
	}
}

func TestLeanDVBTelemetry(t *testing.T) {
	d := &Decoder{generation: 3}
	d.applyTelemetry(`STANDARD "DVB-S2"`, 3)
	d.applyTelemetry("MER 8.7", 3)
	d.applyTelemetry("LOCK 1", 3)
	s := d.Snapshot()
	if s.Standard != "DVB-S2" || s.MER != 8.7 || !s.Locked {
		t.Fatalf("unexpected status: %#v", s)
	}
}
func TestTransportSync(t *testing.T) {
	b := make([]byte, 10+188*3)
	b[10], b[198], b[386] = 0x47, 0x47, 0x47
	if ok, off := transportSync(b); !ok || off != 10 {
		t.Fatalf("ok=%v offset=%d", ok, off)
	}
}
