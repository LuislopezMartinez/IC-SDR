package pocsag

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
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

const (
	InputRate  = 48_000
	OutputRate = 22_050
	maxEvents  = 2_000
)

type Protocol string

const (
	ProtocolPOCSAG Protocol = "POCSAG"
	ProtocolFLEX   Protocol = "FLEX"
)

type Event struct {
	Received    time.Time `json:"received"`
	Address     uint64    `json:"address"`
	Function    int       `json:"function"`
	Baud        int       `json:"baud"`
	Kind        string    `json:"kind"`
	Message     string    `json:"message"`
	Raw         string    `json:"raw,omitempty"`
	FrequencyHz int64     `json:"frequencyHz,omitempty"`
	Protocol    Protocol  `json:"protocol,omitempty"`
	SyncLevel   int       `json:"syncLevel,omitempty"`
	Phase       string    `json:"phase,omitempty"`
	Cycle       int       `json:"cycle,omitempty"`
	Frame       int       `json:"frame,omitempty"`
}

type Status struct {
	Running         bool
	Paused          bool
	State           string
	Error           string
	Events          []Event
	Queued, Dropped uint64
	LastBaud        int
	InputPeakDBFS   float64
	ProcessID       int
	Decoded         uint64
	PersistError    string
	Protocol        Protocol
	LastSyncLevel   int
	LastPhase       string
	LastCycle       int
	LastFrame       int
}

type Decoder struct {
	executable     string
	mu             sync.RWMutex
	state          string
	lastError      string
	events         []Event
	lastBaud       int
	paused         bool
	queue          chan []float32
	stop           chan struct{}
	done           chan struct{}
	cmd            *exec.Cmd
	releaseProcess func()
	generation     uint64
	stdin          io.WriteCloser
	running        atomic.Bool
	dropped        atomic.Uint64
	decoded        atomic.Uint64
	frequency      atomic.Int64
	inputPeak      atomic.Uint64
	historyPath    string
	protocol       Protocol
	lastSyncLevel  int
	lastPhase      string
	lastCycle      int
	lastFrame      int
}

func New(executable string, historyPath ...string) *Decoder {
	d := &Decoder{executable: executable, state: "DETENIDO", protocol: ProtocolPOCSAG}
	d.inputPeak.Store(math.Float64bits(-140))
	if len(historyPath) > 0 {
		d.historyPath = historyPath[0]
		d.loadHistory()
	}
	return d
}

func (d *Decoder) Configure(enabled bool) {
	if !enabled {
		d.Stop()
		return
	}
	if d.running.Load() {
		return
	}
	d.start()
}

func (d *Decoder) start() {
	d.mu.Lock()
	// Configure can be called by the UI and receiver lifecycle concurrently.
	// Holding the lock through process creation guarantees one owned runtime.
	if d.cmd != nil || d.running.Load() {
		d.mu.Unlock()
		return
	}
	if d.executable == "" {
		d.state, d.lastError = "ERROR", "runtime multimon-ng no configurado"
		d.mu.Unlock()
		return
	}
	if _, err := os.Stat(d.executable); err != nil {
		d.state, d.lastError = "ERROR", "runtime multimon-ng no encontrado: "+d.executable
		d.mu.Unlock()
		return
	}
	cmd := exec.Command(d.executable, decoderArguments(d.protocol)...)
	cmd.SysProcAttr = hiddenProcessAttributes()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		d.state, d.lastError = "ERROR", err.Error()
		d.mu.Unlock()
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		d.state, d.lastError = "ERROR", err.Error()
		d.mu.Unlock()
		return
	}
	stderr, _ := cmd.StderrPipe()
	if err = cmd.Start(); err != nil {
		_ = stdin.Close()
		d.state, d.lastError = "ERROR", err.Error()
		d.mu.Unlock()
		return
	}
	releaseProcess, lifetimeErr := attachProcessLifetime(cmd)
	if lifetimeErr != nil {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		d.state, d.lastError = "ERROR", "no se puede supervisar multimon-ng: "+lifetimeErr.Error()
		d.mu.Unlock()
		return
	}
	d.generation++
	generation := d.generation
	d.cmd, d.stdin = cmd, stdin
	d.releaseProcess = releaseProcess
	d.queue, d.stop, d.done = make(chan []float32, 12), make(chan struct{}), make(chan struct{})
	d.state, d.lastError = "ESCUCHANDO", ""
	d.mu.Unlock()
	d.running.Store(true)
	go d.writer(generation)
	go d.reader(stdout, generation)
	go d.readErrors(stderr)
	go d.wait(cmd, generation, releaseProcess)
}

func (d *Decoder) ProcessAudio(samples []float32) {
	if !d.running.Load() || len(samples) == 0 {
		return
	}
	peak := float32(0)
	for _, sample := range samples {
		if abs := float32(math.Abs(float64(sample))); abs > peak {
			peak = abs
		}
	}
	d.inputPeak.Store(math.Float64bits(20 * math.Log10(max(float64(peak), 0.0000001))))
	d.mu.RLock()
	paused, queue := d.paused, d.queue
	d.mu.RUnlock()
	if paused || queue == nil {
		return
	}
	copySamples := append([]float32(nil), samples...)
	select {
	case queue <- copySamples:
	default:
		d.dropped.Add(1)
	}
}

func (d *Decoder) writer(generation uint64) {
	defer close(d.done)
	d.mu.RLock()
	queue, stop, output := d.queue, d.stop, d.stdin
	d.mu.RUnlock()
	resampler := newResampler(InputRate, OutputRate)
	for {
		d.mu.RLock()
		current := d.generation == generation
		d.mu.RUnlock()
		if !current {
			return
		}
		select {
		case <-stop:
			_ = output.Close()
			return
		case samples := <-queue:
			pcm := resampler.process(samples)
			if len(pcm) == 0 {
				continue
			}
			payload := make([]byte, len(pcm)*2)
			for i, sample := range pcm {
				binary.LittleEndian.PutUint16(payload[i*2:], uint16(sample))
			}
			if _, err := output.Write(payload); err != nil {
				d.setError(err)
				return
			}
		}
	}
}

type jsonEvent struct {
	DemodName string  `json:"demod_name"`
	Address   *uint64 `json:"address"`
	Function  *int    `json:"function"`
	Numeric   string  `json:"numeric"`
	Alpha     string  `json:"alpha"`
	Timestamp string  `json:"timestamp"`
	Capcode   *uint64 `json:"capcode"`
	SyncBaud  int     `json:"sync_baud"`
	SyncLevel int     `json:"sync_level"`
	Phase     string  `json:"phase_number"`
	Cycle     int     `json:"cycle_number"`
	Frame     int     `json:"frame_number"`
	Message   string  `json:"message"`
	Tone      string  `json:"tone"`
	ToneLong  string  `json:"tone_long"`
	Baud      int     `json:"baud"`
}

func ParseEvent(line []byte) (Event, bool) {
	var raw jsonEvent
	if json.Unmarshal(line, &raw) != nil {
		return Event{}, false
	}
	name := strings.ToUpper(raw.DemodName)
	if strings.HasPrefix(name, "FLEX") {
		baud := raw.SyncBaud
		if baud == 0 {
			baud = raw.Baud
		}
		e := Event{Received: time.Now(), Function: -1, Raw: string(line), Protocol: ProtocolFLEX,
			Baud: baud, SyncLevel: raw.SyncLevel, Phase: raw.Phase, Cycle: raw.Cycle, Frame: raw.Frame}
		if raw.Capcode != nil {
			e.Address = *raw.Capcode
		} else if raw.Address != nil {
			e.Address = *raw.Address
		}
		switch {
		case strings.Contains(name, "ALPHANUMERIC"):
			e.Kind = "ALFA"
		case strings.Contains(name, "NUMERIC"):
			e.Kind = "NUM"
		case strings.Contains(name, "TONE"):
			e.Kind = "TONO"
		default:
			e.Kind = "FLEX"
		}
		e.Message = cleanMessage(raw.Message)
		if e.Message == "" {
			e.Message = cleanMessage(raw.Alpha)
		}
		if e.Message == "" {
			e.Message = cleanMessage(raw.Numeric)
		}
		if e.Message == "" {
			e.Message = cleanMessage(strings.TrimSpace(raw.Tone + " " + raw.ToneLong))
		}
		if e.Message == "" && e.Kind == "TONO" {
			e.Message = "TONO"
		}
		return e, e.Address > 0 || e.Message != ""
	}
	if !strings.HasPrefix(name, "POCSAG") || raw.Address == nil {
		return Event{}, false
	}
	e := Event{Received: time.Now(), Address: *raw.Address, Function: -1, Raw: string(line), Protocol: ProtocolPOCSAG}
	if raw.Function != nil {
		e.Function = *raw.Function
	}
	for _, baud := range []int{512, 1200, 2400} {
		if strings.Contains(name, strconv.Itoa(baud)) {
			e.Baud = baud
			break
		}
	}
	if raw.Alpha != "" {
		e.Kind, e.Message = "ALFA", cleanMessage(raw.Alpha)
	} else if raw.Numeric != "" {
		e.Kind, e.Message = "NUM", cleanMessage(raw.Numeric)
	} else {
		e.Kind, e.Message = "TONO", "TONO"
	}
	return e, true
}

func decoderArguments(protocol Protocol) []string {
	base := []string{"-q", "-c"}
	if protocol == ProtocolFLEX {
		base = append(base, "-a", "FLEX", "-a", "FLEX_NEXT")
	} else {
		base = append(base, "-a", "POCSAG512", "-a", "POCSAG1200", "-a", "POCSAG2400", "-f", "auto", "-b", "2", "-e")
	}
	return append(base, "--json", "-t", "raw", "-")
}

func cleanMessage(value string) string {
	for _, padding := range []string{"<NUL>", "<EOT>", "<ETX>"} {
		value = strings.ReplaceAll(value, padding, "")
	}
	return strings.TrimSpace(value)
}

func (d *Decoder) reader(reader io.Reader, generation uint64) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		if event, ok := ParseEvent(scanner.Bytes()); ok {
			event.FrequencyHz = d.frequency.Load()
			d.mu.Lock()
			if d.generation != generation {
				d.mu.Unlock()
				return
			}
			// Auto mode may emit numeric and alpha interpretations of one page.
			// Prefer the alpha interpretation when it immediately follows the
			// numeric candidate for the same codeword sequence.
			replaced := false
			if event.Protocol == ProtocolPOCSAG && event.Kind == "ALFA" && len(d.events) > 0 {
				latest := &d.events[0]
				if latest.Kind == "NUM" && latest.Address == event.Address && latest.Function == event.Function && latest.Baud == event.Baud && event.Received.Sub(latest.Received) < time.Second {
					*latest, replaced = event, true
				}
			}
			if !replaced && len(d.events) > 0 {
				latest := d.events[0]
				replaced = latest.Protocol == event.Protocol && latest.Address == event.Address && latest.Kind == event.Kind && latest.Message == event.Message && event.Received.Sub(latest.Received) < time.Second
			}
			if !replaced {
				d.events = append(d.events, Event{})
				copy(d.events[1:], d.events[:len(d.events)-1])
				d.events[0] = event
			}
			if len(d.events) > maxEvents {
				d.events = d.events[:maxEvents]
			}
			d.lastBaud, d.lastSyncLevel, d.lastPhase, d.lastCycle, d.lastFrame, d.state = event.Baud, event.SyncLevel, event.Phase, event.Cycle, event.Frame, "DECODIFICANDO"
			d.mu.Unlock()
			d.decoded.Add(1)
			d.appendHistory(event)
		}
	}
}

func (d *Decoder) readErrors(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "multimon-ng") || strings.HasPrefix(line, "Available demodulators") {
			continue
		}
		d.mu.Lock()
		d.lastError = line
		d.mu.Unlock()
	}
}

func (d *Decoder) wait(cmd *exec.Cmd, generation uint64, releaseProcess func()) {
	err := cmd.Wait()
	d.mu.Lock()
	if d.generation == generation && d.cmd == cmd {
		d.running.Store(false)
		stop := d.stop
		d.cmd, d.stdin, d.queue, d.stop, d.done, d.releaseProcess = nil, nil, nil, nil, nil, nil
		d.state = "DETENIDO"
		d.mu.Unlock()
		safeClose(stop)
		if err != nil {
			d.setError(err)
		}
	} else {
		d.mu.Unlock()
	}
	if releaseProcess != nil {
		releaseProcess()
	}
}

func (d *Decoder) Stop() {
	d.running.Store(false)
	d.mu.Lock()
	stop, done, cmd, releaseProcess := d.stop, d.done, d.cmd, d.releaseProcess
	d.stop, d.done, d.cmd, d.stdin, d.queue, d.releaseProcess = nil, nil, nil, nil, nil, nil
	d.state = "DETENIDO"
	d.mu.Unlock()
	safeClose(stop)
	if done != nil {
		select {
		case <-done:
		case <-time.After(750 * time.Millisecond):
		}
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	if releaseProcess != nil {
		releaseProcess()
	}
}

func safeClose(ch chan struct{}) {
	if ch == nil {
		return
	}
	defer func() { _ = recover() }()
	close(ch)
}

func (d *Decoder) Close() { d.Stop() }

func (d *Decoder) SetPaused(paused bool) {
	d.mu.Lock()
	d.paused = paused
	if paused && d.running.Load() {
		d.state = "PAUSADO"
	} else if d.running.Load() {
		d.state = "ESCUCHANDO"
	}
	d.mu.Unlock()
}

func (d *Decoder) SetFrequency(hz int64) { d.frequency.Store(hz) }

func (d *Decoder) SetProtocol(protocol Protocol) {
	if protocol != ProtocolPOCSAG && protocol != ProtocolFLEX {
		return
	}
	d.mu.RLock()
	current, restart := d.protocol, d.running.Load()
	d.mu.RUnlock()
	if current == protocol {
		return
	}
	if restart {
		d.Stop()
	}
	d.mu.Lock()
	d.protocol = protocol
	d.mu.Unlock()
	if restart {
		d.Configure(true)
	}
}

func (d *Decoder) Clear() {
	d.mu.Lock()
	d.events = nil
	d.mu.Unlock()
	if d.historyPath != "" {
		if err := os.MkdirAll(filepath.Dir(d.historyPath), 0755); err == nil {
			if err = os.WriteFile(d.historyPath, nil, 0644); err != nil {
				d.setPersistError(err)
			}
		} else {
			d.setPersistError(err)
		}
	}
}

func (d *Decoder) ClearProtocol(protocol Protocol) {
	d.mu.Lock()
	kept := d.events[:0]
	for _, event := range d.events {
		if event.Protocol != protocol {
			kept = append(kept, event)
		}
	}
	d.events = kept
	events := append([]Event(nil), kept...)
	d.mu.Unlock()
	if d.historyPath == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(d.historyPath), 0755); err != nil {
		d.setPersistError(err)
		return
	}
	f, err := os.Create(d.historyPath)
	if err != nil {
		d.setPersistError(err)
		return
	}
	encoder := json.NewEncoder(f)
	for index := len(events) - 1; index >= 0 && err == nil; index-- {
		err = encoder.Encode(events[index])
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		d.setPersistError(err)
	}
}

func (d *Decoder) Snapshot() Status {
	d.mu.RLock()
	queued, pid := uint64(0), 0
	if d.queue != nil {
		queued = uint64(len(d.queue))
	}
	if d.cmd != nil && d.cmd.Process != nil {
		pid = d.cmd.Process.Pid
	}
	status := Status{Running: d.running.Load(), Paused: d.paused, State: d.state, Error: d.lastError, Events: append([]Event(nil), d.events...), Queued: queued, Dropped: d.dropped.Load(), LastBaud: d.lastBaud, InputPeakDBFS: math.Float64frombits(d.inputPeak.Load()), ProcessID: pid, Decoded: d.decoded.Load(), Protocol: d.protocol, LastSyncLevel: d.lastSyncLevel, LastPhase: d.lastPhase, LastCycle: d.lastCycle, LastFrame: d.lastFrame}
	d.mu.RUnlock()
	return status
}

func (d *Decoder) appendHistory(event Event) {
	if d.historyPath == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(d.historyPath), 0755); err != nil {
		d.setPersistError(err)
		return
	}
	f, err := os.OpenFile(d.historyPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		d.setPersistError(err)
		return
	}
	data, err := json.Marshal(event)
	if err == nil {
		_, err = f.Write(append(data, '\n'))
	}
	_ = f.Close()
	if err != nil {
		d.setPersistError(err)
	}
}

func (d *Decoder) loadHistory() {
	f, err := os.Open(d.historyPath)
	if err != nil {
		return
	}
	defer f.Close()
	var loaded []Event
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 4096), 1024*1024)
	for s.Scan() {
		var e Event
		if json.Unmarshal(s.Bytes(), &e) == nil {
			if e.Protocol == "" {
				e.Protocol = ProtocolPOCSAG
			}
			loaded = append(loaded, e)
			if len(loaded) > maxEvents {
				loaded = loaded[1:]
			}
		}
	}
	for index := len(loaded) - 1; index >= 0; index-- {
		d.events = append(d.events, loaded[index])
	}
}

func (d *Decoder) setPersistError(err error) {
	d.mu.Lock()
	d.lastError = "historial: " + err.Error()
	d.mu.Unlock()
}

func (d *Decoder) setError(err error) {
	d.mu.Lock()
	d.state, d.lastError = "ERROR", err.Error()
	d.mu.Unlock()
}

type resampler struct {
	step, position float64
	previous       float32
	havePrevious   bool
}

func newResampler(inputRate, outputRate int) *resampler {
	return &resampler{step: float64(inputRate) / float64(outputRate)}
}

func (r *resampler) process(input []float32) []int16 {
	if len(input) == 0 {
		return nil
	}
	samples := input
	if r.havePrevious {
		samples = make([]float32, len(input)+1)
		samples[0] = r.previous
		copy(samples[1:], input)
	} else {
		r.havePrevious = true
	}
	out := make([]int16, 0, int(float64(len(samples))/r.step)+1)
	for r.position+1 < float64(len(samples)) {
		i := int(r.position)
		fraction := float32(r.position - float64(i))
		value := samples[i] + (samples[i+1]-samples[i])*fraction
		value = max(float32(-1), min(float32(1), value))
		out = append(out, int16(math.Round(float64(value*32767))))
		r.position += r.step
	}
	r.position -= float64(len(samples) - 1)
	r.previous = input[len(input)-1]
	return out
}
