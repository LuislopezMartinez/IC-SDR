package dsp

import (
	"math"
	"testing"
)

func TestFFTPutsComplexToneAtExpectedCenteredBin(t *testing.T) {
	const size = 1024
	const toneBin = 123
	iq := make([]float32, size*2)
	for index := 0; index < size; index++ {
		phase := 2 * math.Pi * toneBin * float64(index) / size
		iq[index*2] = float32(math.Cos(phase))
		iq[index*2+1] = float32(math.Sin(phase))
	}
	spectrum := make([]float32, size)
	NewFFT(size).Process(iq, spectrum)
	maximum := 0
	for index := range spectrum {
		if spectrum[index] > spectrum[maximum] {
			maximum = index
		}
	}
	want := size/2 + toneBin
	if maximum != want {
		t.Fatalf("peak bin = %d, want %d", maximum, want)
	}
}

func TestFFTDCSpikeRemovalKeepsOffsetTone(t *testing.T) {
	const size = 1024
	const toneBin = 80
	iq := make([]float32, size*2)
	for index := 0; index < size; index++ {
		phase := 2 * math.Pi * toneBin * float64(index) / size
		iq[index*2] = 0.45 + 0.25*float32(math.Cos(phase))
		iq[index*2+1] = 0.35 + 0.25*float32(math.Sin(phase))
	}
	raw := make([]float32, size)
	cleaned := make([]float32, size)
	fft := NewFFT(size)
	fft.Process(iq, raw)
	fft.ProcessDC(iq, cleaned, true)
	if raw[size/2] <= raw[size/2+toneBin] {
		t.Fatalf("expected a DC spike before removal: dc=%.2f tone=%.2f", raw[size/2], raw[size/2+toneBin])
	}
	if cleaned[size/2+toneBin] <= cleaned[size/2] {
		t.Fatalf("DC spike remained after removal: dc=%.2f tone=%.2f", cleaned[size/2], cleaned[size/2+toneBin])
	}
}

func TestFFTFullScaleComplexToneHasZeroDBFSPeak(t *testing.T) {
	const size = 1024
	const toneBin = 64
	iq := make([]float32, size*2)
	for index := 0; index < size; index++ {
		phase := 2 * math.Pi * toneBin * float64(index) / size
		iq[index*2] = float32(math.Cos(phase))
		iq[index*2+1] = float32(math.Sin(phase))
	}
	spectrum := make([]float32, size)
	NewFFT(size).Process(iq, spectrum)
	peak := spectrum[size/2+toneBin]
	if math.Abs(float64(peak)) > .02 {
		t.Fatalf("full-scale FFT peak = %.4f dBFS, want 0 dBFS", peak)
	}
}
