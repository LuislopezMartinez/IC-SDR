package dsp

import (
	"go-zero/internal/i18n"

	"math"
	"sync"
)

// FFT converts one complex IQ block into a centered dBFS-like spectrum.
// Size must be a power of two.
type FFT struct {
	size       int
	levels     int
	window     []float64
	real       []float64
	imag       []float64
	mu         sync.Mutex
	windowType string
}

func NewFFT(size int) *FFT {
	if size < 2 || size&(size-1) != 0 {
		panic(i18n.Source("text.b61ed9896340"))
	}
	levels := 0
	for value := size; value > 1; value >>= 1 {
		levels++
	}
	fft := &FFT{
		size: size, levels: levels,
		window:     make([]float64, size),
		real:       make([]float64, size),
		imag:       make([]float64, size),
		windowType: i18n.Source("text.26701b540b4b"),
	}
	for index := range fft.window {
		fft.window[index] = .5 - .5*math.Cos(2*math.Pi*float64(index)/float64(size-1))
	}
	return fft
}

func (fft *FFT) Size() int { return fft.size }

func (fft *FFT) SetWindowType(windowType string) {
	fft.mu.Lock()
	defer fft.mu.Unlock()
	fft.windowType = windowType
	for index := range fft.window {
		phase := 2 * math.Pi * float64(index) / float64(fft.size-1)
		switch windowType {
		case "BLACKMAN-HARRIS":
			fft.window[index] = .35875 - .48829*math.Cos(phase) + .14128*math.Cos(2*phase) - .01168*math.Cos(3*phase)
		case "FLAT TOP":
			fft.window[index] = .21557895 - .41663158*math.Cos(phase) + .277263158*math.Cos(2*phase) - .083578947*math.Cos(3*phase) + .006947368*math.Cos(4*phase)
		case "RECTANGULAR":
			fft.window[index] = 1
		default:
			fft.window[index] = .5 - .5*math.Cos(phase)
		}
	}
}

func (fft *FFT) Process(interleavedIQ []float32, output []float32) {
	fft.mu.Lock()
	defer fft.mu.Unlock()
	if len(interleavedIQ) < fft.size*2 || len(output) < fft.size {
		return
	}
	for index := 0; index < fft.size; index++ {
		fft.real[index] = float64(interleavedIQ[index*2]) * fft.window[index]
		fft.imag[index] = float64(interleavedIQ[index*2+1]) * fft.window[index]
	}
	fft.transform()

	normalization := 2 / float64(fft.size)
	half := fft.size / 2
	for displayBin := 0; displayBin < fft.size; displayBin++ {
		fftBin := (displayBin + half) & (fft.size - 1)
		real := fft.real[fftBin] * normalization
		imag := fft.imag[fftBin] * normalization
		power := real*real + imag*imag
		output[displayBin] = float32(10 * math.Log10(power+1e-14))
	}
}

func (fft *FFT) transform() {
	for index := 0; index < fft.size; index++ {
		reversed := reverseBits(index, fft.levels)
		if reversed > index {
			fft.real[index], fft.real[reversed] = fft.real[reversed], fft.real[index]
			fft.imag[index], fft.imag[reversed] = fft.imag[reversed], fft.imag[index]
		}
	}
	for transformSize := 2; transformSize <= fft.size; transformSize <<= 1 {
		half := transformSize >> 1
		for start := 0; start < fft.size; start += transformSize {
			for offset := 0; offset < half; offset++ {
				angle := -2 * math.Pi * float64(offset) / float64(transformSize)
				wr, wi := math.Cos(angle), math.Sin(angle)
				even, odd := start+offset, start+offset+half
				real := wr*fft.real[odd] - wi*fft.imag[odd]
				imag := wr*fft.imag[odd] + wi*fft.real[odd]
				fft.real[odd] = fft.real[even] - real
				fft.imag[odd] = fft.imag[even] - imag
				fft.real[even] += real
				fft.imag[even] += imag
			}
		}
	}
}

func reverseBits(value, count int) int {
	result := 0
	for index := 0; index < count; index++ {
		result = result<<1 | value&1
		value >>= 1
	}
	return result
}
