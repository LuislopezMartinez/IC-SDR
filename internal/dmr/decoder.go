package dmr

import (
	"go-zero/internal/i18n"

	"bufio"
	"encoding/binary"
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

const outputRate = 48_000

type Status struct {
	State, Detail, AFCState, Slot1, Slot2, AudioSlot string
	ColorCode, InputLevel, SyncQuality               int
	PLLLocked, SignalActive                          bool
	FrequencyErrorHz, AFCCorrectionHz                float32
	Queued, Capacity                                 int
	Dropped                                          uint64
}

type Decoder struct {
	inputRate float64
	exe       string
	onAudio   func([]float32)
	queue     chan []float32
	stop      chan struct{}
	done      chan struct{}
	enabled   atomic.Bool
	dropped   atomic.Uint64
	pool      sync.Pool

	mu                   sync.RWMutex
	status               Status
	offsetHz             float64
	bandwidth            int
	autoCenter           bool
	audioSlot            string
	desiredEnabled       bool
	resyncing            bool
	restartAttempts      int
	process              *exec.Cmd
	stdin                io.WriteCloser
	generation           uint64
	rawState             string
	confirmed            bool
	confirmationStreak   int
	candidateSince       time.Time
	lastTrustedTelemetry time.Time
	lastTelemetry        time.Time
	afcErrorSum          float64
	afcErrorSamples      int
	validAFCWindows      int
	lastValidAFC         time.Time
	pendingAudio         []float32

	front *frontend
}

func New(inputRate float64, executable string, onAudio func([]float32)) *Decoder {
	d := &Decoder{inputRate: inputRate, exe: executable, onAudio: onAudio, queue: make(chan []float32, 32), audioSlot: i18n.Source("text.6ea56fae9eac"), bandwidth: 12_500}
	d.pool.New = func() any { return make([]float32, 0, 16_384) }
	d.status = Status{State: i18n.Source("text.38cca6bea010"), AFCState: i18n.Source("text.38cca6bea010"), Slot1: "--", Slot2: "--", AudioSlot: i18n.Source("text.6ea56fae9eac"), ColorCode: -1, Capacity: cap(d.queue)}
	return d
}

func (d *Decoder) Configure(enabled bool, offsetHz float64, bandwidth int) {
	d.mu.Lock()
	d.desiredEnabled = enabled
	bandwidth = min(max(bandwidth, 8_000), 18_000)
	offsetChanged := math.Abs(d.offsetHz-offsetHz) >= 1
	changed := offsetChanged || d.bandwidth != bandwidth
	d.offsetHz, d.bandwidth = offsetHz, bandwidth
	if offsetChanged {
		d.resetAFCLocked()
	}
	if changed && d.front != nil {
		d.front.reset(d.offsetHz, d.bandwidth)
	}
	running, resyncing := d.enabled.Load(), d.resyncing
	d.mu.Unlock()
	if enabled && !running && !resyncing {
		_ = d.start()
	}
	if !enabled && running {
		d.Stop()
	}
}

func (d *Decoder) SetAutoCenter(enabled bool) {
	d.mu.Lock()
	d.autoCenter = enabled
	if !enabled {
		d.resetAFCLocked()
	} else if d.status.AFCState == i18n.Source("text.38cca6bea010") {
		d.status.AFCState = i18n.Source("text.01a3981a1797")
	}
	front := d.front
	d.mu.Unlock()
	if !enabled && front != nil {
		front.setCorrection(0)
	}
}

func (d *Decoder) SetAudioSlot(slot string) {
	if slot != "TS1" && slot != "TS2" {
		slot = i18n.Source("text.6ea56fae9eac")
	}
	d.mu.Lock()
	changed := slot != d.audioSlot
	d.audioSlot, d.status.AudioSlot = slot, slot
	d.mu.Unlock()
	if changed && d.enabled.Load() {
		d.Resync()
	}
}

func (d *Decoder) ProcessIQ(iq []float32) {
	if !d.enabled.Load() {
		return
	}
	copyBlock := d.pool.Get().([]float32)
	if cap(copyBlock) < len(iq) {
		copyBlock = make([]float32, len(iq))
	} else {
		copyBlock = copyBlock[:len(iq)]
	}
	copy(copyBlock, iq)
	select {
	case d.queue <- copyBlock:
	default:
		d.pool.Put(copyBlock[:0])
		d.dropped.Add(1)
	}
}

func (d *Decoder) Snapshot() Status {
	d.mu.RLock()
	result := d.status
	result.SignalActive = d.confirmed
	d.mu.RUnlock()
	result.Queued, result.Capacity, result.Dropped = len(d.queue), cap(d.queue), d.dropped.Load()
	return result
}

func (d *Decoder) Resync() bool {
	d.mu.Lock()
	if !d.desiredEnabled || !d.enabled.Load() || d.resyncing {
		d.mu.Unlock()
		return false
	}
	d.resyncing = true
	d.restartAttempts = 0
	d.stopLocked(false)
	d.mu.Unlock()
	d.drainQueue()
	go func() {
		_ = d.start()
		d.mu.Lock()
		d.resyncing = false
		d.mu.Unlock()
	}()
	return true
}

func (d *Decoder) start() error {
	d.mu.Lock()
	if d.enabled.Load() || !d.desiredEnabled {
		d.mu.Unlock()
		return nil
	}
	exePath, err := filepath.Abs(d.exe)
	if err != nil {
		d.mu.Unlock()
		return err
	}
	if _, err = os.Stat(exePath); err != nil {
		d.status.State, d.status.Detail = i18n.Source("text.d98ee0e5f939"), i18n.Source("text.3580796643aa")
		d.mu.Unlock()
		return err
	}
	slot := strings.ToLower(d.audioSlot)
	cmd := exec.Command(exePath, "--stream", "--slot", slot)
	cmd.Dir = filepath.Dir(exePath)
	cmd.Env = append(os.Environ(), i18n.Source("text.c243c1091a83")+filepath.Dir(exePath)+string(os.PathListSeparator)+os.Getenv(i18n.Source("text.d4db71eecc1a")))
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
		d.status.State, d.status.Detail = i18n.Source("text.d98ee0e5f939"), err.Error()
		d.mu.Unlock()
		return err
	}
	d.process, d.stdin = cmd, stdin
	d.stop, d.done = make(chan struct{}), make(chan struct{})
	d.front = newFrontend(d.inputRate, d.offsetHz, d.bandwidth)
	d.resetSessionLocked()
	d.enabled.Store(true)
	d.generation++
	generation := d.generation
	d.mu.Unlock()
	stop := d.stop
	go d.processingLoop(generation, stop)
	go d.audioLoop(stdout, generation)
	go d.statusLoop(stderr, generation)
	go func() {
		err := cmd.Wait()
		d.mu.Lock()
		if d.generation != generation {
			d.mu.Unlock()
			return
		}
		d.enabled.Store(false)
		if d.stop != nil {
			close(d.stop)
			d.stop = nil
		}
		d.process, d.stdin, d.front = nil, nil, nil
		d.status.State = i18n.Source("text.d98ee0e5f939")
		if err != nil {
			d.status.Detail = i18n.Source("text.364f5bde1bb1") + err.Error()
		} else {
			d.status.Detail = i18n.Source("text.8b07a1a9373f")
		}
		shouldRestart := d.desiredEnabled && d.restartAttempts < 3 && !d.resyncing
		restartAttempt := 0
		if shouldRestart {
			d.restartAttempts++
			restartAttempt = d.restartAttempts
		}
		d.mu.Unlock()
		d.drainQueue()
		if shouldRestart {
			time.Sleep(time.Duration(restartAttempt) * 500 * time.Millisecond)
			_ = d.start()
		}
	}()
	return nil
}

func (d *Decoder) Stop() {
	d.mu.Lock()
	d.desiredEnabled = false
	d.stopLocked(true)
	d.mu.Unlock()
	d.drainQueue()
}

func (d *Decoder) stopLocked(showOff bool) {
	d.enabled.Store(false)
	d.generation++
	if d.stop != nil {
		close(d.stop)
		d.stop = nil
	}
	if d.stdin != nil {
		_ = d.stdin.Close()
	}
	if d.process != nil && d.process.Process != nil {
		_ = d.process.Process.Kill()
	}
	d.process, d.stdin, d.front = nil, nil, nil
	d.confirmed, d.rawState = false, ""
	if showOff {
		d.status.State, d.status.Detail = i18n.Source("text.38cca6bea010"), ""
	}
}

func (d *Decoder) drainQueue() {
	for {
		select {
		case block := <-d.queue:
			d.pool.Put(block[:0])
		default:
			return
		}
	}
}

func (d *Decoder) processingLoop(generation uint64, stop <-chan struct{}) {
	var buffered *bufio.Writer
	pendingBytes := 0
	for {
		var block []float32
		select {
		case <-stop:
			if buffered != nil {
				_ = buffered.Flush()
			}
			return
		case block = <-d.queue:
		}
		d.mu.RLock()
		active := d.enabled.Load() && d.generation == generation
		front, writer := d.front, d.stdin
		d.mu.RUnlock()
		if !active || front == nil || writer == nil {
			d.pool.Put(block[:0])
			return
		}
		if buffered == nil {
			buffered = bufio.NewWriterSize(writer, 8192)
		}
		pcm, measured := front.process(block)
		d.pool.Put(block[:0])
		d.updateAFC(measured, len(pcm))
		if len(pcm) > 0 {
			if err := binary.Write(buffered, binary.LittleEndian, pcm); err != nil {
				d.failGeneration(generation, err)
				return
			}
			pendingBytes += len(pcm) * 2
		}
		if pendingBytes >= 2048 {
			if err := buffered.Flush(); err != nil {
				d.failGeneration(generation, err)
				return
			}
			pendingBytes = 0
		}
	}
}

func (d *Decoder) failGeneration(generation uint64, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.generation != generation || !d.enabled.Load() {
		return
	}
	d.status.State, d.status.Detail = i18n.Source("text.d98ee0e5f939"), i18n.Source("text.35f9b0b1a599")+err.Error()
	if d.stdin != nil {
		_ = d.stdin.Close()
	}
	if d.process != nil && d.process.Process != nil {
		_ = d.process.Process.Kill()
	}
}

func decodePCM16LE(reader io.Reader, emit func([]float32)) error {
	buffer := make([]byte, 4096)
	samples := make([]float32, 0, 1920)
	pendingLowByte := -1
	for {
		n, err := reader.Read(buffer)
		index := 0
		if pendingLowByte >= 0 && n > 0 {
			word := uint16(pendingLowByte) | uint16(buffer[index])<<8
			samples = append(samples, float32(int16(word))/32768)
			pendingLowByte, index = -1, 1
		}
		for index+1 < n {
			word := binary.LittleEndian.Uint16(buffer[index : index+2])
			samples = append(samples, float32(int16(word))/32768)
			index += 2
		}
		if index < n {
			pendingLowByte = int(buffer[index])
		}
		if len(samples) > 0 {
			emit(samples)
			samples = samples[:0]
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func (d *Decoder) audioLoop(reader io.Reader, generation uint64) {
	_ = decodePCM16LE(reader, func(samples []float32) {
		d.mu.Lock()
		if !d.enabled.Load() || d.generation != generation {
			d.mu.Unlock()
			return
		}
		// DSDcc can produce the first vocoder block before the telemetry guard
		// confirms DMR. Retain that candidate audio so the beginning of the
		// transmission is not lost; SEARCH clears it if the candidate was false.
		if !d.confirmed {
			d.pendingAudio = append(d.pendingAudio, samples...)
			if excess := len(d.pendingAudio) - outputRate; excess > 0 {
				copy(d.pendingAudio, d.pendingAudio[excess:])
				d.pendingAudio = d.pendingAudio[:outputRate]
			}
			d.mu.Unlock()
			return
		}
		output := make([]float32, 0, len(d.pendingAudio)+len(samples))
		output = append(output, d.pendingAudio...)
		output = append(output, samples...)
		d.pendingAudio = d.pendingAudio[:0]
		d.mu.Unlock()
		if len(output) > 0 && d.onAudio != nil {
			d.onAudio(output)
		}
	})
}

func (d *Decoder) statusLoop(reader io.Reader, generation uint64) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024), 64*1024)
	for scanner.Scan() {
		line := scanner.Text()
		d.mu.Lock()
		if d.generation != generation {
			d.mu.Unlock()
			return
		}
		switch {
		case strings.HasPrefix(line, "EVENT "):
			fields := strings.Fields(line)
			if len(fields) > 1 {
				d.handleRawStateLocked(fields[1], time.Now())
			}
		case strings.HasPrefix(line, "STATUS "):
			d.status.Detail = strings.TrimSpace(strings.TrimPrefix(line, i18n.Source("text.d553aebdff7d")))
		case strings.HasPrefix(line, "TELEMETRY "):
			now := time.Now()
			d.lastTelemetry = now
			for _, field := range strings.Fields(strings.TrimPrefix(line, i18n.Source("text.f10ca5baa704"))) {
				parts := strings.SplitN(field, "=", 2)
				if len(parts) != 2 {
					continue
				}
				value := strings.ReplaceAll(parts[1], "_", " ")
				switch parts[0] {
				case "CC":
					d.status.ColorCode, _ = strconv.Atoi(value)
				case "LEVEL":
					d.status.InputLevel, _ = strconv.Atoi(value)
				case "QUALITY":
					d.status.SyncQuality, _ = strconv.Atoi(value)
				case "PLL":
					d.status.PLLLocked = value == "1"
				case "TS1":
					d.status.Slot1 = value
				case "TS2":
					d.status.Slot2 = value
				}
			}
			d.updateConfirmationLocked(now)
			if d.confirmed {
				d.restartAttempts = 0
			}
		}
		d.mu.Unlock()
	}
}

func (d *Decoder) handleRawStateLocked(next string, now time.Time) {
	d.rawState = next
	if next == i18n.Source("text.a430e6d293d0") || next == i18n.Source("text.c97c29c7a71b") {
		if d.confirmed {
			d.status.State = next
		} else {
			if d.candidateSince.IsZero() {
				d.candidateSince = now
			}
			d.status.State = i18n.Source("text.6306148e33dd")
		}
		return
	}
	d.confirmationStreak = 0
	if next == i18n.Source("text.56f21695a650") && d.confirmed {
		d.status.State = i18n.Source("text.aacf94b7be62")
		return
	}
	if next == i18n.Source("text.56f21695a650") && !d.confirmed {
		d.pendingAudio = d.pendingAudio[:0]
	}
	d.confirmed = false
	d.candidateSince = time.Time{}
	d.status.State = next
}

func (d *Decoder) updateConfirmationLocked(now time.Time) {
	rawDMR := d.rawState == i18n.Source("text.a430e6d293d0") || d.rawState == i18n.Source("text.c97c29c7a71b")
	trustworthy := rawDMR && d.status.PLLLocked && d.status.SyncQuality > 0 && d.status.InputLevel > 0
	if trustworthy {
		d.lastTrustedTelemetry = now
		d.confirmationStreak++
		if !d.confirmed && d.confirmationStreak >= 2 && !d.candidateSince.IsZero() && now.Sub(d.candidateSince) >= 200*time.Millisecond {
			d.confirmed = true
			d.status.State = d.rawState
		} else if d.confirmed {
			d.status.State = d.rawState
		}
		return
	}
	d.confirmationStreak = 0
	if d.confirmed && now.Sub(d.lastTrustedTelemetry) > 700*time.Millisecond {
		d.confirmed = false
		if rawDMR {
			d.candidateSince = now
			d.status.State = i18n.Source("text.6306148e33dd")
		} else {
			d.candidateSince = time.Time{}
			d.status.State = i18n.Source("text.56f21695a650")
		}
	} else if !d.confirmed && d.rawState == i18n.Source("text.56f21695a650") {
		d.candidateSince = time.Time{}
		d.status.State = i18n.Source("text.56f21695a650")
		d.pendingAudio = d.pendingAudio[:0]
	}
}

func (d *Decoder) updateAFC(measured float32, sampleCount int) {
	if sampleCount <= 0 {
		return
	}
	d.mu.Lock()
	d.afcErrorSum += float64(measured) * float64(sampleCount)
	d.afcErrorSamples += sampleCount
	if d.afcErrorSamples < outputRate/2 {
		d.mu.Unlock()
		return
	}
	windowMeasurement := float32(d.afcErrorSum / float64(d.afcErrorSamples))
	d.afcErrorSum, d.afcErrorSamples = 0, 0
	now := time.Now()
	trusted := d.confirmed && (d.status.State == i18n.Source("text.a430e6d293d0") || d.status.State == i18n.Source("text.c97c29c7a71b")) &&
		d.status.PLLLocked && d.status.SyncQuality > 0 && d.status.InputLevel > 0 &&
		!d.lastTelemetry.IsZero() && now.Sub(d.lastTelemetry) <= 750*time.Millisecond
	if !d.autoCenter {
		d.status.AFCState = i18n.Source("text.38cca6bea010")
		d.validAFCWindows = 0
		d.mu.Unlock()
		return
	}
	if !trusted {
		d.validAFCWindows = 0
		d.status.FrequencyErrorHz *= .8
		if !d.lastValidAFC.IsZero() && now.Sub(d.lastValidAFC) > 5*time.Second && abs32(d.status.AFCCorrectionHz) > .5 {
			d.status.AFCCorrectionHz += min(max(-d.status.AFCCorrectionHz, -10), 10)
		}
		if abs32(d.status.AFCCorrectionHz) > .5 {
			d.status.AFCState = i18n.Source("text.aacf94b7be62")
		} else {
			d.status.AFCState = i18n.Source("text.01a3981a1797")
		}
		correction, front := d.status.AFCCorrectionHz, d.front
		d.mu.Unlock()
		if front != nil {
			front.setCorrection(float64(correction))
		}
		return
	}
	d.lastValidAFC = now
	d.status.FrequencyErrorHz = .85*d.status.FrequencyErrorHz + .15*windowMeasurement
	d.validAFCWindows++
	if d.validAFCWindows < 3 {
		d.status.AFCState = i18n.Source("text.848cd7228796")
		d.mu.Unlock()
		return
	}
	if abs32(d.status.FrequencyErrorHz) <= 90 {
		d.status.AFCState = i18n.Source("text.bfc160483cb0")
		d.mu.Unlock()
		return
	}
	step := min(max(d.status.FrequencyErrorHz*.06, -30), 30)
	d.status.AFCCorrectionHz = min(max(d.status.AFCCorrectionHz+step, -1500), 1500)
	if abs32(d.status.AFCCorrectionHz) >= 1499 {
		d.status.AFCState = i18n.Source("text.18ba5c477fc5")
	} else {
		d.status.AFCState = i18n.Source("text.e433860031a8")
	}
	correction, front := d.status.AFCCorrectionHz, d.front
	d.mu.Unlock()
	if front != nil {
		front.setCorrection(float64(correction))
	}
}

func (d *Decoder) resetAFCLocked() {
	d.afcErrorSum, d.afcErrorSamples, d.validAFCWindows = 0, 0, 0
	d.lastValidAFC = time.Time{}
	d.status.FrequencyErrorHz, d.status.AFCCorrectionHz = 0, 0
	if d.autoCenter {
		d.status.AFCState = i18n.Source("text.01a3981a1797")
	} else {
		d.status.AFCState = i18n.Source("text.38cca6bea010")
	}
}

func (d *Decoder) resetSessionLocked() {
	d.resetAFCLocked()
	d.rawState, d.confirmed, d.confirmationStreak = i18n.Source("text.56f21695a650"), false, 0
	d.pendingAudio = d.pendingAudio[:0]
	d.candidateSince, d.lastTrustedTelemetry, d.lastTelemetry = time.Time{}, time.Time{}, time.Time{}
	d.status.State, d.status.Detail = i18n.Source("text.56f21695a650"), i18n.Source("text.7330f4b9b4ca")
	d.status.ColorCode, d.status.InputLevel, d.status.SyncQuality = -1, 0, 0
	d.status.PLLLocked, d.status.Slot1, d.status.Slot2 = false, "--", "--"
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func (s Status) String() string {
	return fmt.Sprintf(i18n.Source("text.17f6383c65ea"), s.State, s.ColorCode, s.Slot1, s.Slot2)
}
