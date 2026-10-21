package tetrapol

import (
	"math"
	"testing"
)

func TestMonitorMeasuresGMSKLikeCarrier(t *testing.T) {
	d := New(2_048_000)
	d.Configure(true)
	iq := make([]float32, 65_536)
	for n := 0; n < len(iq)/2; n++ {
		p := 2 * math.Pi * 1000 * float64(n) / 2_048_000
		iq[2*n], iq[2*n+1] = float32(math.Cos(p)), float32(math.Sin(p))
	}
	d.ProcessIQ(iq[:len(iq)/2])
	d.ProcessIQ(iq[len(iq)/2:])
	s := d.Snapshot()
	if !s.Running || s.Symbols == 0 || math.Abs(s.FrequencyDeviationHz-1000) > 20 || s.LevelDBFS < -0.1 {
		t.Fatalf("%+v", s)
	}
}

func TestMonitorProducesHardSymbolsAcrossIQBlocks(t *testing.T) {
	const rate = 2_048_000.0
	pattern := []byte{1, 0, 1, 1, 0, 0, 1, 0}
	const samplesPerSymbol = 256
	iq := make([]float32, len(pattern)*8*samplesPerSymbol*2)
	phase := 0.0
	for symbol := 0; symbol < len(pattern)*8; symbol++ {
		bit := pattern[symbol%len(pattern)]
		frequency := -1_000.0
		if bit != 0 {
			frequency = 1_000
		}
		for sample := 0; sample < samplesPerSymbol; sample++ {
			index := symbol*samplesPerSymbol + sample
			phase += 2 * math.Pi * frequency / rate
			iq[2*index], iq[2*index+1] = float32(math.Cos(phase)), float32(math.Sin(phase))
		}
	}
	decode := func(split bool) []byte {
		d := New(rate)
		var got []byte
		d.SetBitSink(func(bits []byte) { got = append(got, bits...) })
		d.Configure(true)
		if split {
			d.ProcessIQ(iq[:7334])
			d.ProcessIQ(iq[7334:])
		} else {
			d.ProcessIQ(iq)
		}
		return got
	}
	oneBlock, splitBlocks := decode(false), decode(true)
	if len(oneBlock) < 50 || len(oneBlock) != len(splitBlocks) {
		t.Fatalf("symbols one-block=%d split=%d", len(oneBlock), len(splitBlocks))
	}
	for index := range oneBlock {
		if oneBlock[index] != splitBlocks[index] {
			t.Fatalf("split stream differs at symbol %d", index)
		}
	}
}

func TestMonitorAFCTracksCarrierOffset(t *testing.T) {
	const (
		rate          = 2_048_000.0
		carrierOffset = 725.0
		deviation     = 1_000.0
		samples       = 256
	)
	pattern := []byte{1, 0, 1, 1, 0, 0, 1, 0}
	iq := make([]float32, len(pattern)*80*samples*2)
	phase := 0.0
	for symbol := 0; symbol < len(pattern)*80; symbol++ {
		frequency := carrierOffset - deviation
		if pattern[symbol%len(pattern)] != 0 {
			frequency = carrierOffset + deviation
		}
		for sample := 0; sample < samples; sample++ {
			index := symbol*samples + sample
			phase += 2 * math.Pi * frequency / rate
			iq[index*2], iq[index*2+1] = float32(math.Cos(phase)), float32(math.Sin(phase))
		}
	}
	d := New(rate)
	d.Configure(true)
	d.ProcessIQ(iq)
	status := d.Snapshot()
	if status.AFCState != "LOCKED" {
		t.Fatalf("AFC state = %s, want LOCKED (%+v)", status.AFCState, status)
	}
	if math.Abs(status.CarrierOffsetHz-carrierOffset) > 80 {
		t.Fatalf("carrier offset = %.1f Hz, want %.1f±80", status.CarrierOffsetHz, carrierOffset)
	}
	if math.Abs(status.AFCErrorHz) > 80 {
		t.Fatalf("AFC residual = %.1f Hz", status.AFCErrorHz)
	}
}

func TestMonitorCanDisableAFC(t *testing.T) {
	d := New(2_048_000)
	d.SetAFCEnabled(false)
	d.Configure(true)
	if got := d.Snapshot(); got.AFCState != "OFF" || got.CarrierOffsetHz != 0 {
		t.Fatalf("disabled AFC status = %+v", got)
	}
}

func TestIQSinkUsesNativeChannelRateWithoutIntermediateResampling(t *testing.T) {
	const (
		inputRate    = 2_048_000.0
		inputSamples = 8_192
	)
	d := New(inputRate)
	var received []float32
	d.SetIQSink(func(iq []float32) { received = append(received, iq...) })
	d.Configure(true)
	d.ProcessIQ(make([]float32, inputSamples*2))

	if got, want := d.ChannelSampleRate(), 32_000.0; got != want {
		t.Fatalf("native channel rate = %.0f, want %.0f", got, want)
	}
	wantComplexSamples := inputSamples / 64
	if got := len(received) / 2; got != wantComplexSamples {
		t.Fatalf("native IQ samples = %d, want %d (direct decimated stream)", got, wantComplexSamples)
	}
}
