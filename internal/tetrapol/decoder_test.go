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
