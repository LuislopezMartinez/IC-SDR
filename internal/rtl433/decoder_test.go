package rtl433

import (
	"bytes"
	"math"
	"testing"
)

func TestFrontendPreservesSignalAcrossIrregularBlocks(t *testing.T) {
	iq := make([]float32, 10_000*2)
	for n := 0; n < len(iq)/2; n++ {
		phase := 2 * math.Pi * 175_000 * float64(n) / 2_048_000
		iq[2*n], iq[2*n+1] = float32(.5*math.Cos(phase)), float32(.5*math.Sin(phase))
	}
	whole := newFrontend(2_048_000, 175_000, 250_000)
	want := whole.process(iq)
	split := newFrontend(2_048_000, 175_000, 250_000)
	var got []byte
	for start := 0; start < len(iq); {
		end := min(start+514, len(iq))
		// Compare the DSP independently of per-block gain adaptation.
		split.gain = 1
		got = append(got, split.process(iq[start:end])...)
		start = end
	}
	if !bytes.Equal(got, want) {
		t.Fatal("mixing or decimation changes at block boundaries")
	}
	for n := 100; n < len(want)/2; n++ {
		if want[2*n] < 190 || want[2*n] > 192 || want[2*n+1] < 127 || want[2*n+1] > 128 {
			t.Fatalf("translated tone is not stable DC at sample %d", n)
		}
	}
}

func BenchmarkWideFrontends(b *testing.B) {
	iq := make([]float32, 4096*2)
	for i := range iq {
		iq[i] = .1
	}
	var fronts []*frontend
	for _, hz := range multichannelCenters(0, 2_000_000) {
		fronts = append(fronts, newFrontend(2_048_000, float64(hz), 250_000))
	}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for _, f := range fronts {
			f.process(iq)
		}
	}
}

func TestParseEventPreservesOriginalFields(t *testing.T) {
	line := []byte(`{"time":"@0.017384s","protocol":59,"type":"TPMS","model":"Steelmate","id":"0x2980","pressure_kPa":43.75,"temperature_C":23,"battery_mV":2990,"mod":"FSK","freq1":433.923,"rssi":1.167,"snr":6.408}`)
	event, ok := ParseEvent(line)
	if !ok || event.Protocol != 59 || event.Model != "Steelmate" || event.ID != "0x2980" {
		t.Fatalf("unexpected event: %+v, ok=%v", event, ok)
	}
	if math.Abs(event.FreqMHz-433.923) > .000001 || event.Summary == "" || event.Raw != string(line) {
		t.Fatalf("event lost decoder data: %+v", event)
	}
}

func TestFrontendProducesCU8AtOutputRate(t *testing.T) {
	f := newFrontend(2_048_000, 250_000, 250_000)
	iq := make([]float32, 4096*2)
	for n := 0; n < 4096; n++ {
		phase := 2 * math.Pi * 250_000 * float64(n) / 2_048_000
		iq[n*2], iq[n*2+1] = float32(math.Cos(phase))*.1, float32(math.Sin(phase))*.1
	}
	out := f.process(iq)
	if len(out) != 4096/8*2 {
		t.Fatalf("CU8 bytes=%d, want %d", len(out), 4096/8*2)
	}
}

func TestFrontendSelectableBandwidthRates(t *testing.T) {
	cases := []struct{ width, rate, decimation int }{{250_000, 256_000, 8}, {500_000, 512_000, 4}, {1_000_000, 1_024_000, 2}, {2_000_000, 2_048_000, 1}}
	for _, test := range cases {
		front := newFrontend(2_048_000, 0, test.width)
		if front.outputRate != test.rate || front.decimation != test.decimation {
			t.Errorf("width %d: rate=%d decimation=%d", test.width, front.outputRate, front.decimation)
		}
	}
}

func TestWideCoverageKeepsCenterAndOverlapsNarrowChannels(t *testing.T) {
	for _, test := range []struct {
		width, count int
	}{{500_000, 3}, {1_000_000, 5}, {2_000_000, 11}} {
		centers := multichannelCenters(433_920_000, test.width)
		if len(centers) != test.count {
			t.Fatalf("width %d: channels=%d, want %d", test.width, len(centers), test.count)
		}
		for index := 1; index < len(centers); index++ {
			if centers[index]-centers[index-1] > 200_000 {
				t.Fatalf("width %d: filters do not overlap: %v", test.width, centers)
			}
		}
		if centers[len(centers)/2] != 433_920_000 {
			t.Fatalf("selected frequency has no centered channel: %v", centers)
		}
		coveredLow := centers[0] - 125_000
		coveredHigh := centers[len(centers)-1] + 125_000
		if coveredHigh-coveredLow != int64(test.width) {
			t.Fatalf("width %d: coverage %d..%d", test.width, coveredLow, coveredHigh)
		}
	}
}

func TestMultichannelDispatchIgnoresPreviousSingleQueue(t *testing.T) {
	d := New(2_048_000, "")
	d.queue = make(chan []float32, 1)
	d.manager = true
	child := New(2_048_000, "")
	child.queue = make(chan []float32, 1)
	child.running.Store(true)
	d.children = []*Decoder{child}
	d.running.Store(true)
	iq := []float32{.1, .2}
	d.ProcessIQ(iq)
	if len(d.queue) != 0 || len(child.queue) != 1 {
		t.Fatal("IQ must reach child receivers, not the obsolete single-channel queue")
	}
	iq[0] = .9
	if got := <-child.queue; got[0] != .1 {
		t.Fatal("queued IQ must not share the capture buffer")
	}
}
