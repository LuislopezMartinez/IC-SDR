// Package iqcapture writes narrow-band IQ WAV files for external decoders.
package iqcapture

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
)

const Rate = 128000

type Metadata struct {
	CenterHz, ChannelHz    int64  `json:"centerHz"`
	SourceRate, OutputRate int    `json:"sourceRate"`
	Format                 string `json:"format"`
}
type Writer struct {
	mu                sync.Mutex
	file              *os.File
	path              string
	center, channel   int64
	inputRate, factor int
	phase             float64
	samples           uint32
	decimation        int
	history           []complex128
	historyAt         int
	kernel            []float64
}

func New(path string, center, channel int64, inputRate float64) (*Writer, error) {
	factor := int(math.Round(inputRate / Rate))
	if factor < 1 || math.Abs(inputRate-float64(factor*Rate)) > .01 {
		return nil, fmt.Errorf("la tasa IQ %.0f no permite exportar a %d sps", inputRate, Rate)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	w := &Writer{file: f, path: path, center: center, channel: channel, inputRate: int(math.Round(inputRate)), factor: factor, history: make([]complex128, 129)}
	w.kernel = lowPassKernel(len(w.history), 20_000/float64(w.inputRate))
	if _, err = f.Write(make([]byte, 44)); err != nil {
		f.Close()
		return nil, err
	}
	return w, nil
}
func (w *Writer) Write(iq []float32) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return
	}
	step := 2 * math.Pi * float64(w.channel-w.center) / float64(w.inputRate)
	out := make([]byte, 0, len(iq)*2/w.factor)
	for n := 0; n+1 < len(iq); n += 2 {
		c, s := math.Cos(w.phase), math.Sin(w.phase)
		sample := complex(float64(iq[n])*c+float64(iq[n+1])*s, float64(iq[n+1])*c-float64(iq[n])*s)
		w.history[w.historyAt] = sample
		w.historyAt = (w.historyAt + 1) % len(w.history)
		w.phase += step
		w.decimation++
		if w.decimation == w.factor {
			w.decimation = 0
			var filtered complex128
			for tap, coefficient := range w.kernel {
				filtered += w.history[(w.historyAt-1-tap+len(w.history))%len(w.history)] * complex(coefficient, 0)
			}
			out = append16(out, real(filtered))
			out = append16(out, imag(filtered))
			w.samples++
		}
	}
	_, _ = w.file.Write(out)
}
func lowPassKernel(length int, cutoff float64) []float64 {
	result := make([]float64, length)
	middle := float64(length-1) / 2
	sum := 0.0
	for index := range result {
		x := float64(index) - middle
		value := 2 * cutoff
		if math.Abs(x) > 1e-12 {
			value = math.Sin(2*math.Pi*cutoff*x) / (math.Pi * x)
		}
		value *= .42 - .5*math.Cos(2*math.Pi*float64(index)/float64(length-1)) + .08*math.Cos(4*math.Pi*float64(index)/float64(length-1))
		result[index] = value
		sum += value
	}
	for index := range result {
		result[index] /= sum
	}
	return result
}
func append16(b []byte, v float64) []byte {
	v = math.Max(-1, math.Min(1, v))
	return binary.LittleEndian.AppendUint16(b, uint16(int16(math.Round(v*32767))))
}
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	data := w.samples * 4
	h := make([]byte, 44)
	copy(h, "RIFF")
	binary.LittleEndian.PutUint32(h[4:], 36+data)
	copy(h[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(h[16:], 16)
	binary.LittleEndian.PutUint16(h[20:], 1)
	binary.LittleEndian.PutUint16(h[22:], 2)
	binary.LittleEndian.PutUint32(h[24:], Rate)
	binary.LittleEndian.PutUint32(h[28:], Rate*4)
	binary.LittleEndian.PutUint16(h[32:], 4)
	binary.LittleEndian.PutUint16(h[34:], 16)
	copy(h[36:], "data")
	binary.LittleEndian.PutUint32(h[40:], data)
	_, err := w.file.WriteAt(h, 0)
	if e := w.file.Close(); err == nil {
		err = e
	}
	meta := Metadata{w.center, w.channel, w.inputRate, Rate, "PCM S16LE stereo: I=left, Q=right"}
	if b, e := json.MarshalIndent(meta, "", "  "); err == nil && e == nil {
		err = os.WriteFile(w.path+".json", b, 0644)
	}
	w.file = nil
	return err
}
