package dab

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const OutputSampleRate = 48_000

// Status is the stable boundary between the bundled DAB runtime and the UI.
// Fields not supplied by an older runtime retain their zero value.
type Status struct {
	Available, Running, Synced, AudioActive bool
	State, Detail, Ensemble, Service        string
	DynamicLabel, AudioMode                 string
	ServiceID                               uint32
	SNR, FrequencyErrorHz                   float32
	InputQueued, InputCapacity              int
	Dropped                                 uint64
	LastAudio                               time.Time
	Services                                []Service
	AudioSampleRate, AudioChannels          int
}

type Service struct {
	ID   uint32
	Name string
}

type runtimeEvent struct {
	Type, State, Detail, Ensemble, Service string
	Label, Mode                            string
	ServiceID                              uint32   `json:"serviceId"`
	Synced                                 *bool    `json:"synced"`
	SNR                                    *float32 `json:"snr"`
	FrequencyErrorHz                       *float32 `json:"frequencyErrorHz"`
	SampleRate, Channels                   int
}

// Decoder owns the external, headless welle.io runtime. IQ enters as
// interleaved CF32 LE and decoded PCM leaves as float32 stereo or mono.
type Decoder struct {
	executable string
	inputRate  int
	onAudio    func([]float32)

	mu         sync.RWMutex
	status     Status
	serviceID  uint32
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	queue      chan []float32
	stop       chan struct{}
	generation uint64
	channels   atomic.Int32
	sampleRate atomic.Int32
	dropped    atomic.Uint64
	pool       sync.Pool
}

func New(inputRate float64, executable string, onAudio func([]float32)) *Decoder {
	rate := int(math.Round(inputRate))
	if rate <= 0 {
		rate = 2_048_000
	}
	d := &Decoder{executable: executable, inputRate: rate, onAudio: onAudio}
	d.status.State = "stopped"
	if info, err := os.Stat(executable); err == nil && !info.IsDir() {
		d.status.Available = true
	} else {
		d.status.Detail = "DAB runtime not installed"
	}
	d.channels.Store(2)
	d.sampleRate.Store(OutputSampleRate)
	d.pool.New = func() any { return make([]float32, 0, 16_384) }
	return d
}

func (d *Decoder) Configure(enabled bool, serviceID uint32) error {
	d.mu.Lock()
	d.serviceID = serviceID
	running := d.status.Running
	d.mu.Unlock()
	if !enabled {
		d.Stop()
		return nil
	}
	if running {
		return nil
	}
	return d.start()
}

func (d *Decoder) SelectService(serviceID uint32) error {
	d.mu.Lock()
	changed, running := d.serviceID != serviceID, d.status.Running
	d.serviceID = serviceID
	d.mu.Unlock()
	if !changed || !running {
		return nil
	}
	d.Stop()
	return d.start()
}

func (d *Decoder) start() error {
	d.mu.Lock()
	if d.status.Running {
		d.mu.Unlock()
		return nil
	}
	if !d.status.Available {
		d.mu.Unlock()
		return fmt.Errorf("DAB runtime not found: %s", d.executable)
	}
	abs, err := filepath.Abs(d.executable)
	if err != nil {
		d.mu.Unlock()
		return err
	}
	args := []string{"--stream", "--protocol", "1", "--input", "cf32le", "--sample-rate", strconv.Itoa(d.inputRate), "--audio", "f32le", "--audio-rate", strconv.Itoa(OutputSampleRate)}
	if d.serviceID != 0 {
		args = append(args, "--service-id", strconv.FormatUint(uint64(d.serviceID), 10))
	}
	cmd := exec.Command(abs, args...)
	cmd.Dir = filepath.Dir(abs)
	configureHiddenProcess(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		d.mu.Unlock()
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		d.mu.Unlock()
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		d.mu.Unlock()
		return err
	}
	if err = cmd.Start(); err != nil {
		d.status.State, d.status.Detail = "error", err.Error()
		d.mu.Unlock()
		return err
	}
	d.generation++
	generation := d.generation
	d.cmd, d.stdin = cmd, stdin
	d.queue, d.stop = make(chan []float32, 24), make(chan struct{})
	d.status.Running, d.status.State, d.status.Detail = true, "starting", ""
	d.status.Synced, d.status.AudioActive = false, false
	d.status.Ensemble, d.status.Service, d.status.DynamicLabel, d.status.AudioMode, d.status.ServiceID = "", "", "", "", 0
	d.status.Services = nil
	d.mu.Unlock()
	go d.writeLoop(generation)
	go d.audioLoop(stdout, generation)
	go d.eventLoop(stderr, generation)
	go d.waitLoop(cmd, generation)
	return nil
}

func (d *Decoder) ProcessIQ(iq []float32) {
	d.mu.RLock()
	running, queue := d.status.Running, d.queue
	d.mu.RUnlock()
	if !running || len(iq) == 0 {
		return
	}
	block := d.pool.Get().([]float32)
	if cap(block) < len(iq) {
		block = make([]float32, len(iq))
	} else {
		block = block[:len(iq)]
	}
	copy(block, iq)
	select {
	case queue <- block:
	default:
		d.pool.Put(block[:0])
		d.dropped.Add(1)
	}
}

func (d *Decoder) Snapshot() Status {
	d.mu.RLock()
	status := d.status
	status.Services = append([]Service(nil), d.status.Services...)
	if d.queue != nil {
		status.InputQueued, status.InputCapacity = len(d.queue), cap(d.queue)
	}
	d.mu.RUnlock()
	status.Dropped = d.dropped.Load()
	status.AudioSampleRate = int(d.sampleRate.Load())
	status.AudioChannels = int(d.channels.Load())
	if status.AudioActive && time.Since(status.LastAudio) > time.Second {
		status.AudioActive = false
	}
	return status
}

func (d *Decoder) Stop() {
	d.mu.Lock()
	if !d.status.Running {
		d.mu.Unlock()
		return
	}
	d.status.Running, d.status.Synced, d.status.AudioActive = false, false, false
	d.status.State = "stopped"
	stop, stdin, cmd, queue := d.stop, d.stdin, d.cmd, d.queue
	d.stop, d.stdin, d.cmd, d.queue = nil, nil, nil, nil
	close(stop)
	d.mu.Unlock()
	_ = stdin.Close()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	for {
		select {
		case block := <-queue:
			d.pool.Put(block[:0])
		default:
			return
		}
	}
}

func (d *Decoder) Close() { d.Stop() }

func (d *Decoder) writeLoop(generation uint64) {
	for {
		d.mu.RLock()
		if d.generation != generation {
			d.mu.RUnlock()
			return
		}
		stop, queue, writer := d.stop, d.queue, d.stdin
		d.mu.RUnlock()
		select {
		case <-stop:
			return
		case block := <-queue:
			data := make([]byte, len(block)*4)
			for i, sample := range block {
				binary.LittleEndian.PutUint32(data[i*4:], math.Float32bits(sample))
			}
			d.pool.Put(block[:0])
			if _, err := writer.Write(data); err != nil {
				return
			}
		}
	}
}

func (d *Decoder) audioLoop(reader io.Reader, generation uint64) {
	buffer, carry := make([]byte, 16_384), make([]byte, 0, 8)
	resampler := streamResampler{outputRate: OutputSampleRate}
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			data := append(carry, buffer[:n]...)
			channels := max(int(d.channels.Load()), 1)
			frameBytes := channels * 4
			complete := len(data) - len(data)%frameBytes
			carry = append(carry[:0], data[complete:]...)
			mono := decodePCMFloat32(data[:complete], channels)
			mono = resampler.Process(mono, int(d.sampleRate.Load()))
			if len(mono) > 0 && d.onAudio != nil {
				d.onAudio(mono)
				d.mu.Lock()
				if d.generation == generation {
					d.status.AudioActive, d.status.LastAudio = true, time.Now()
				}
				d.mu.Unlock()
			}
		}
		if err != nil {
			return
		}
	}
}

type streamResampler struct {
	buffer                []float32
	position              float64
	inputRate, outputRate int
}

func (r *streamResampler) Process(input []float32, inputRate int) []float32 {
	if len(input) == 0 || inputRate <= 0 || r.outputRate <= 0 {
		return nil
	}
	if inputRate == r.outputRate {
		r.buffer, r.position, r.inputRate = r.buffer[:0], 0, inputRate
		return input
	}
	if r.inputRate != inputRate {
		r.buffer, r.position, r.inputRate = r.buffer[:0], 0, inputRate
	}
	r.buffer = append(r.buffer, input...)
	if len(r.buffer) < 2 {
		return nil
	}
	ratio := float64(inputRate) / float64(r.outputRate)
	capacity := int((float64(len(r.buffer))-r.position)/ratio) + 1
	output := make([]float32, 0, max(capacity, 0))
	for r.position+1 < float64(len(r.buffer)) {
		base := int(r.position)
		fraction := float32(r.position - float64(base))
		output = append(output, r.buffer[base]+fraction*(r.buffer[base+1]-r.buffer[base]))
		r.position += ratio
	}
	consumed := min(int(r.position), len(r.buffer)-1)
	if consumed > 0 {
		copy(r.buffer, r.buffer[consumed:])
		r.buffer = r.buffer[:len(r.buffer)-consumed]
		r.position -= float64(consumed)
	}
	return output
}

func decodePCMFloat32(data []byte, channels int) []float32 {
	channels = max(channels, 1)
	frames := len(data) / (channels * 4)
	result := make([]float32, frames)
	for frame := 0; frame < frames; frame++ {
		var sum float32
		for channel := 0; channel < channels; channel++ {
			offset := (frame*channels + channel) * 4
			sum += math.Float32frombits(binary.LittleEndian.Uint32(data[offset:]))
		}
		result[frame] = min(max(sum/float32(channels), -1), 1)
	}
	return result
}

func (d *Decoder) eventLoop(reader io.Reader, generation uint64) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		var event runtimeEvent
		if json.Unmarshal(scanner.Bytes(), &event) != nil {
			continue
		}
		d.applyEvent(event, generation)
	}
}

func (d *Decoder) applyEvent(event runtimeEvent, generation uint64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.generation != generation {
		return
	}
	if event.State != "" {
		d.status.State = event.State
	}
	if event.Detail != "" {
		d.status.Detail = event.Detail
	}
	if event.Ensemble != "" {
		d.status.Ensemble = event.Ensemble
	}
	if event.Service != "" && event.Type != "service" {
		d.status.Service = event.Service
	}
	if event.ServiceID != 0 {
		d.status.ServiceID = event.ServiceID
	}
	if event.Type == "service" && event.ServiceID != 0 {
		found := false
		for index := range d.status.Services {
			if d.status.Services[index].ID == event.ServiceID {
				d.status.Services[index].Name = event.Service
				found = true
				break
			}
		}
		if !found {
			d.status.Services = append(d.status.Services, Service{ID: event.ServiceID, Name: event.Service})
		}
		if d.status.ServiceID == event.ServiceID && event.Service != "" {
			d.status.Service = event.Service
		}
	}
	if event.ServiceID != 0 && event.Type == "audio" {
		for _, service := range d.status.Services {
			if service.ID == event.ServiceID {
				d.status.Service = service.Name
				break
			}
		}
	}
	if event.Type == "label" {
		d.status.DynamicLabel = strings.TrimSpace(event.Label)
	}
	if event.Type == "audio" && event.Mode != "" {
		d.status.AudioMode = event.Mode
	}
	if event.Synced != nil {
		d.status.Synced = *event.Synced
	}
	if event.SNR != nil {
		d.status.SNR = *event.SNR
	}
	if event.FrequencyErrorHz != nil {
		d.status.FrequencyErrorHz = *event.FrequencyErrorHz
	}
	if event.Channels > 0 {
		d.channels.Store(int32(event.Channels))
	}
	if event.SampleRate > 0 {
		d.sampleRate.Store(int32(event.SampleRate))
	}
}

func (d *Decoder) waitLoop(cmd *exec.Cmd, generation uint64) {
	err := cmd.Wait()
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.generation != generation {
		return
	}
	d.status.Running, d.status.Synced, d.status.AudioActive = false, false, false
	if err != nil && d.status.State != "stopped" {
		d.status.State, d.status.Detail = "error", err.Error()
	}
}
