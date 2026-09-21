package iqcapture

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestWavecomWriterProducesStereo128kWAV(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.wav")
	w, e := New(p, 100000000, 100000000, 2048000)
	if e != nil {
		t.Fatal(e)
	}
	iq := make([]float32, 16*8*2)
	for i := 0; i < len(iq); i += 2 {
		iq[i] = .5
	}
	w.Write(iq)
	if e = w.Close(); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	if string(b[:4]) != "RIFF" || string(b[8:12]) != "WAVE" || binary.LittleEndian.Uint32(b[24:]) != Rate || binary.LittleEndian.Uint16(b[22:]) != 2 || len(b) != 44+8*4 {
		t.Fatalf("invalid wave: %d", len(b))
	}
	if _, e = os.Stat(p + ".json"); e != nil {
		t.Fatal(e)
	}
}

func TestWavecomWriterRejectsOutOfBandEnergy(t *testing.T) {
	measure := func(hz float64) float64 {
		p := filepath.Join(t.TempDir(), "tone.wav")
		w, err := New(p, 100_000_000, 100_000_000, 2_048_000)
		if err != nil {
			t.Fatal(err)
		}
		iq := make([]float32, 2_048*2)
		for n := 0; n < len(iq)/2; n++ {
			phase := 2 * math.Pi * hz * float64(n) / 2_048_000
			iq[2*n], iq[2*n+1] = float32(math.Cos(phase)), float32(math.Sin(phase))
		}
		w.Write(iq)
		if err = w.Close(); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var sum float64
		count := 0
		for at := 44 + 128*4; at+3 < len(data); at += 4 {
			v := float64(int16(binary.LittleEndian.Uint16(data[at:]))) / 32767
			sum += v * v
			count++
		}
		return math.Sqrt(sum / float64(count))
	}
	if wanted, rejected := measure(8_000), measure(300_000); wanted < .4 || rejected > wanted*.02 {
		t.Fatalf("filter pass/reject=%f/%f", wanted, rejected)
	}
}
