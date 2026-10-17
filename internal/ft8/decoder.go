package ft8

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
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const (
	InputRate   = 48_000
	DecoderRate = 12_000
	SlotSeconds = 15
	SlotSamples = DecoderRate * SlotSeconds
	// Audio callbacks may jitter, but a half-second hole is long enough to move
	// an FT8 symbol window materially out of place.
	continuityTolerance = 500 * time.Millisecond
)

type Decode struct {
	Slot       time.Time
	SNR        int
	DT         float64
	Frequency  float64
	Pass       int
	Corrected  int
	Message    string
	ReceivedAt time.Time
}

type Status struct {
	Running, Enabled bool
	Waiting          bool
	State, Detail    string
	Slot             time.Time
	WaitUntil        time.Time
	Progress         float32
	DecodeCount      int
	Dropped          uint64
	Decodes          []Decode
	Depth            string
	ClockOffset      float64
	ClockSamples     int
	ResyncCount      uint64
	LastResyncDrift  time.Duration
}

const (
	DepthFast     = "fast"
	DepthBalanced = "balanced"
	DepthDeep     = "deep"
)

type slotPacket struct {
	start   int64
	samples []int16
}

type runtimeEvent struct {
	Type, State, Message, Detail string
	Slot, Count, SNR, Pass       int64
	Corrected                    int
	DT, Frequency                float64
}

type Decoder struct {
	executable string
	queue      chan slotPacket
	running    atomic.Bool
	dropped    atomic.Uint64

	mu                sync.RWMutex
	enabled           bool
	status            Status
	cmd               *exec.Cmd
	stdin             io.WriteCloser
	stop              chan struct{}
	generation        uint64
	captureSlot       int64
	captureArmedAt    time.Time
	audioCursor       time.Time
	waitUntil         time.Time
	phase             int
	phaseSum          float64
	pcm               []int16
	depth             string
	lastInputAt       time.Time
	lastInputDuration time.Duration
}

func New(executable string) *Decoder {
	return &Decoder{executable: executable, queue: make(chan slotPacket, 2), depth: DepthBalanced, status: Status{State: "DETENIDO", Depth: DepthBalanced}}
}

func normalizeDepth(depth string) string {
	switch depth {
	case DepthFast, DepthDeep:
		return depth
	default:
		return DepthBalanced
	}
}

func depthBudget(depth string) string {
	switch normalizeDepth(depth) {
	case DepthFast:
		return "2"
	case DepthDeep:
		return "9"
	default:
		return "5"
	}
}

// SetDepth changes the time available to the multipass decoder. A running
// process is restarted immediately; captured and decoded history is retained.
func (d *Decoder) SetDepth(depth string) {
	depth = normalizeDepth(depth)
	d.mu.Lock()
	if d.depth == depth {
		d.mu.Unlock()
		return
	}
	d.depth, d.status.Depth = depth, depth
	running, enabled := d.running.Load(), d.enabled
	d.mu.Unlock()
	if running {
		d.stopProcess("REINICIANDO")
	}
	if enabled {
		_ = d.start()
	}
}

func (d *Decoder) Configure(enabled bool) {
	d.mu.Lock()
	d.enabled = enabled
	d.status.Enabled = enabled
	if enabled {
		d.resetCaptureLocked(time.Now())
	}
	running := d.running.Load()
	d.mu.Unlock()
	if enabled && !running {
		_ = d.start()
	} else if !enabled && running {
		d.stopProcess("DETENIDO")
	}
}

func (d *Decoder) ProcessAudio(samples []float32) {
	d.processAudioAt(samples, time.Now())
}

func (d *Decoder) processAudioAt(samples []float32, now time.Time) {
	if len(samples) == 0 || !d.running.Load() {
		return
	}
	d.mu.Lock()
	if !d.enabled {
		d.mu.Unlock()
		return
	}
	if drift, resync := inputDiscontinuity(d.lastInputAt, d.lastInputDuration, now); resync {
		d.resyncCaptureLocked(now, drift, "Pausa de audio detectada")
	}
	d.lastInputAt = now
	blockDuration := samplesDuration(len(samples), InputRate)
	d.lastInputDuration = blockDuration
	if drift, resync := cursorDiscontinuity(d.audioCursor, now); resync {
		d.resyncCaptureLocked(now, drift, "Desfase acumulado del audio detectado")
		d.lastInputAt = now
		d.lastInputDuration = blockDuration
	}
	blockStart := d.audioCursor
	if blockStart.IsZero() || d.status.Waiting {
		// A callback contains audio accumulated before it was delivered. Anchor
		// every waiting block to wall time. This prevents a rapidly drained stale
		// buffer from crossing the next UTC boundary before the clock does.
		blockStart = now.Add(-blockDuration)
	}
	d.audioCursor = blockStart.Add(blockDuration)
	start := 0
	if !d.captureArmedAt.IsZero() && blockStart.Before(d.captureArmedAt) {
		start = max(start, sampleIndexAt(blockStart, d.captureArmedAt, InputRate))
	}
	if !d.waitUntil.IsZero() && blockStart.Before(d.waitUntil) {
		start = max(start, sampleIndexAt(blockStart, d.waitUntil, InputRate))
	}
	start = min(start, len(samples))
	if start < len(samples) && d.status.Waiting {
		d.status.Waiting = false
		d.status.State = "ESCUCHANDO"
		d.status.Detail = "Ciclo UTC completo iniciado"
	}
	for _, sample := range samples[start:] {
		d.phaseSum += float64(sample)
		d.phase++
		if d.phase < InputRate/DecoderRate {
			continue
		}
		value := d.phaseSum / float64(d.phase)
		value = math.Max(-1, math.Min(1, value))
		d.pcm = append(d.pcm, int16(math.Round(value*32767)))
		d.phase, d.phaseSum = 0, 0
		if len(d.pcm) == SlotSamples {
			packet := slotPacket{start: d.captureSlot, samples: d.pcm}
			d.enqueueLatest(packet)
			d.captureSlot += SlotSeconds
			d.pcm = make([]int16, 0, SlotSamples)
		}
	}
	d.status.Slot = time.Unix(d.captureSlot, 0).UTC()
	d.status.Progress = float32(len(d.pcm)) / SlotSamples
	d.mu.Unlock()
}

func (d *Decoder) enqueueLatest(packet slotPacket) {
	select {
	case d.queue <- packet:
		return
	default:
	}
	// Decoding an old FT8 slot after a slowdown is less useful than decoding
	// the newest complete slot. Evict one stale packet and retry once.
	select {
	case <-d.queue:
		d.dropped.Add(1)
	default:
	}
	select {
	case d.queue <- packet:
	default:
		d.dropped.Add(1)
	}
}

func inputDiscontinuity(previous time.Time, previousDuration time.Duration, now time.Time) (time.Duration, bool) {
	if previous.IsZero() {
		return 0, false
	}
	drift := now.Sub(previous) - previousDuration
	if drift < 0 {
		// Buffered audio may be delivered faster than real time while the SDR
		// catches up. That is not a hole and must never trigger a reset.
		return drift, false
	}
	return drift, drift > continuityTolerance
}

func cursorDiscontinuity(cursor, now time.Time) (time.Duration, bool) {
	if cursor.IsZero() {
		return 0, false
	}
	drift := now.Sub(cursor)
	return drift, drift > continuityTolerance || drift < -continuityTolerance
}

func (d *Decoder) Snapshot() Status {
	d.mu.RLock()
	result := d.status
	result.Running, result.Dropped = d.running.Load(), d.dropped.Load()
	result.Decodes = append([]Decode(nil), d.status.Decodes...)
	result.ClockOffset, result.ClockSamples = estimateClockOffset(result.Decodes, 20)
	d.mu.RUnlock()
	return result
}

func estimateClockOffset(decodes []Decode, limit int) (float64, int) {
	values := make([]float64, 0, limit)
	for _, item := range decodes {
		if math.IsNaN(item.DT) || math.IsInf(item.DT, 0) || math.Abs(item.DT) > 2.5 {
			continue
		}
		values = append(values, item.DT)
		if len(values) == limit {
			break
		}
	}
	if len(values) == 0 {
		return 0, 0
	}
	sort.Float64s(values)
	middle := len(values) / 2
	if len(values)%2 == 0 {
		return (values[middle-1] + values[middle]) / 2, len(values)
	}
	return values[middle], len(values)
}

func (d *Decoder) Clear() {
	d.mu.Lock()
	d.status.Decodes = nil
	d.status.DecodeCount = 0
	d.mu.Unlock()
}

func (d *Decoder) Close() { d.Configure(false) }

func (d *Decoder) resetCaptureLocked(now time.Time) {
	next := now.UTC().Truncate(SlotSeconds * time.Second).Add(SlotSeconds * time.Second)
	d.captureSlot = next.Unix()
	d.captureArmedAt = now
	d.audioCursor = time.Time{}
	d.waitUntil = next
	d.phase, d.phaseSum = 0, 0
	d.pcm = make([]int16, 0, SlotSamples)
	d.status.Slot, d.status.WaitUntil, d.status.Progress = next, next, 0
	d.status.Waiting = true
	d.status.State = "ESPERANDO CICLO"
	d.status.Detail = "Descartando la trama parcial hasta el próximo límite UTC"
	d.lastInputAt, d.lastInputDuration = time.Time{}, 0
}

func (d *Decoder) resyncCaptureLocked(now time.Time, drift time.Duration, reason string) {
	d.resetCaptureLocked(now)
	d.status.ResyncCount++
	d.status.LastResyncDrift = drift
	d.status.State = "ESPERANDO CICLO"
	d.status.Detail = fmt.Sprintf("%s (%d ms); esperando un ciclo UTC completo", reason, drift.Milliseconds())
}

func samplesDuration(count, rate int) time.Duration {
	if count <= 0 || rate <= 0 {
		return 0
	}
	return time.Duration(float64(count) / float64(rate) * float64(time.Second))
}

func sampleIndexAt(blockStart, target time.Time, rate int) int {
	if !target.After(blockStart) || rate <= 0 {
		return 0
	}
	return int(math.Ceil(target.Sub(blockStart).Seconds() * float64(rate)))
}

func (d *Decoder) start() error {
	d.mu.Lock()
	if d.running.Load() || !d.enabled {
		d.mu.Unlock()
		return nil
	}
	executable, err := filepath.Abs(d.executable)
	if err == nil {
		_, err = os.Stat(executable)
	}
	if err != nil {
		d.status.State, d.status.Detail = "NO DISPONIBLE", "Runtime FT8 no encontrado: "+d.executable
		d.mu.Unlock()
		return err
	}
	cmd := exec.Command(executable, "--budget", depthBudget(d.depth))
	cmd.Dir = filepath.Dir(executable)
	cmd.SysProcAttr = hiddenProcessAttributes()
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
		d.status.State, d.status.Detail = "ERROR", err.Error()
		d.mu.Unlock()
		return err
	}
	d.generation++
	generation := d.generation
	d.cmd, d.stdin, d.stop = cmd, stdin, make(chan struct{})
	d.running.Store(true)
	if d.status.Waiting {
		d.status.State, d.status.Detail = "ESPERANDO CICLO", "Descartando la trama parcial hasta el próximo límite UTC"
	}
	d.mu.Unlock()
	go d.writer(generation, d.stop)
	go d.reader(stdout, generation)
	go d.readErrors(stderr, generation)
	go func() {
		err := cmd.Wait()
		var ended chan struct{}
		d.mu.Lock()
		if d.generation == generation {
			d.running.Store(false)
			ended, d.stop = d.stop, nil
			if d.enabled {
				d.status.State = "ERROR"
				if err != nil {
					d.status.Detail = err.Error()
				}
			}
		}
		d.mu.Unlock()
		if ended != nil {
			close(ended)
		}
	}()
	return nil
}

func (d *Decoder) writer(generation uint64, stop <-chan struct{}) {
	for {
		var packet slotPacket
		select {
		case <-stop:
			return
		case packet = <-d.queue:
		}
		d.mu.RLock()
		stdin, current := d.stdin, d.generation
		d.mu.RUnlock()
		if current != generation || stdin == nil {
			return
		}
		header := make([]byte, 16)
		copy(header, "FT8S")
		binary.LittleEndian.PutUint64(header[4:12], uint64(packet.start))
		binary.LittleEndian.PutUint32(header[12:16], uint32(len(packet.samples)))
		if _, err := stdin.Write(header); err != nil {
			d.fail(generation, err)
			return
		}
		data := make([]byte, len(packet.samples)*2)
		for i, value := range packet.samples {
			binary.LittleEndian.PutUint16(data[i*2:], uint16(value))
		}
		if _, err := stdin.Write(data); err != nil {
			d.fail(generation, err)
			return
		}
	}
}

func (d *Decoder) reader(reader io.Reader, generation uint64) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 128*1024)
	for scanner.Scan() {
		var event runtimeEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}
		d.mu.Lock()
		if d.generation != generation {
			d.mu.Unlock()
			return
		}
		switch event.Type {
		case "slot":
			if event.State == "decoding" {
				d.status.State, d.status.Detail = "DECODIFICANDO", "Analizando señales débiles y cancelando interferencias"
			} else {
				d.status.State = "ESCUCHANDO"
				d.status.Detail = fmt.Sprintf("Ciclo completado: %d mensajes", event.Count)
			}
		case "decode":
			item := Decode{Slot: time.Unix(event.Slot, 0).UTC(), SNR: int(event.SNR), DT: event.DT, Frequency: event.Frequency, Pass: int(event.Pass), Corrected: event.Corrected, Message: event.Message, ReceivedAt: time.Now()}
			d.status.Decodes = append([]Decode{item}, d.status.Decodes...)
			// Preserve enough history for the session map to remember a station's
			// earlier locator when later reports omit it.
			if len(d.status.Decodes) > 2_000 {
				d.status.Decodes = d.status.Decodes[:2_000]
			}
			d.status.DecodeCount++
		}
		d.mu.Unlock()
	}
}

func (d *Decoder) readErrors(reader io.Reader, generation uint64) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		d.mu.Lock()
		if d.generation == generation {
			d.status.Detail = scanner.Text()
		}
		d.mu.Unlock()
	}
}

func (d *Decoder) fail(generation uint64, err error) {
	d.mu.Lock()
	if d.generation == generation {
		d.status.State, d.status.Detail = "ERROR", err.Error()
	}
	d.mu.Unlock()
}

func (d *Decoder) stopProcess(state string) {
	d.mu.Lock()
	if !d.running.Load() {
		d.status.State = state
		d.mu.Unlock()
		return
	}
	d.generation++
	cmd, stdin, stop := d.cmd, d.stdin, d.stop
	d.cmd, d.stdin = nil, nil
	d.running.Store(false)
	d.status.State, d.status.Detail = state, ""
	d.mu.Unlock()
	if stop != nil {
		close(stop)
	}
	for {
		select {
		case <-d.queue:
		default:
			goto drained
		}
	}
drained:
	if stdin != nil {
		_ = stdin.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
