package sstv

import (
	"go-zero/internal/i18n"

	"bufio"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
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

const candidateCount = 4

var ValidModes = []string{"M1", "M2", "S1", "S2", i18n.Source("text.542213f49bef"), "R36", "R72", "PD50", "PD90", "PD120", "PD160", "PD180", "PD240", "PD290"}

type Frame struct {
	Sequence, Width, Height, Lines int
	RGB                            []byte
}

type Status struct {
	Running, Automatic, Candidates                      bool
	State, Mode, SelectedMode, LastSaved, Detail        string
	Width, Height, Lines, Progress, SyncPercent, Queued int
	SyncRMS, ToneFrequency, ToneLevel                   float32
	Dropped                                             uint64
	CandidateModes                                      [candidateCount]string
	CandidateProgress, CandidateSync                    [candidateCount]int
}

type Decoder struct {
	executable, outputFolder string
	queue                    chan []float32
	pool                     sync.Pool
	running                  atomic.Bool
	dropped                  atomic.Uint64

	mu             sync.RWMutex
	enabled        bool
	automatic      bool
	selectedMode   string
	candidateModes [candidateCount]string
	status         Status
	frames         [candidateCount + 1]Frame
	cmd            *exec.Cmd
	stdin          io.WriteCloser
	stop           chan struct{}
	generation     uint64
	restarting     bool
}

func New(executable, outputFolder string) *Decoder {
	d := &Decoder{executable: executable, outputFolder: outputFolder, queue: make(chan []float32, 16), automatic: true, selectedMode: "R36", candidateModes: [4]string{"R36", "R72", "M1", "S1"}}
	d.pool.New = func() any { return make([]float32, 0, 4096) }
	d.status.State, d.status.Mode, d.status.ToneLevel = i18n.Source("text.38cca6bea010"), i18n.Source("text.80e4cf374a8a"), -120
	d.syncStatusLocked()
	return d
}

func (d *Decoder) Configure(enabled bool) {
	d.mu.Lock()
	d.enabled = enabled
	running := d.running.Load()
	d.mu.Unlock()
	if enabled && !running {
		_ = d.start(false)
	} else if !enabled && running {
		d.stopProcess(i18n.Source("text.38cca6bea010"))
	}
}

func (d *Decoder) ProcessAudio(samples []float32) {
	if !d.running.Load() || len(samples) == 0 {
		return
	}
	block := d.pool.Get().([]float32)
	if cap(block) < len(samples) {
		block = make([]float32, len(samples))
	} else {
		block = block[:len(samples)]
	}
	copy(block, samples)
	select {
	case d.queue <- block:
	default:
		d.pool.Put(block[:0])
		d.dropped.Add(1)
	}
}

func (d *Decoder) SetAutomatic(value bool) {
	d.mu.Lock()
	changed := d.automatic != value
	d.automatic = value
	d.syncStatusLocked()
	d.mu.Unlock()
	if changed {
		d.Restart(false)
	}
}

func (d *Decoder) SetSelectedMode(mode string) {
	if !validMode(mode) {
		return
	}
	d.mu.Lock()
	changed := d.selectedMode != mode
	d.selectedMode = mode
	automatic := d.automatic
	d.syncStatusLocked()
	d.mu.Unlock()
	if changed && !automatic {
		d.Restart(false)
	}
}

func (d *Decoder) SetCandidateMode(index int, mode string) {
	if index < 0 || index >= candidateCount || !validMode(mode) {
		return
	}
	d.mu.Lock()
	changed := d.candidateModes[index] != mode
	d.candidateModes[index] = mode
	d.syncStatusLocked()
	d.mu.Unlock()
	if changed {
		d.Restart(false)
	}
}

func (d *Decoder) ForceReceive() { d.Restart(true) }
func (d *Decoder) StopReceive() {
	d.mu.Lock()
	d.automatic = true
	d.mu.Unlock()
	d.Restart(false)
}

func (d *Decoder) Restart(forceCandidates bool) bool {
	d.mu.Lock()
	if !d.enabled || d.restarting {
		d.mu.Unlock()
		return false
	}
	d.restarting = true
	d.mu.Unlock()
	go func() {
		d.stopProcess(i18n.Source("text.56f21695a650"))
		d.clearFrames()
		_ = d.start(forceCandidates)
		d.mu.Lock()
		d.restarting = false
		d.mu.Unlock()
	}()
	return true
}

func (d *Decoder) Snapshot() Status {
	d.mu.RLock()
	s := d.status
	s.Running, s.Queued, s.Dropped = d.running.Load(), len(d.queue), d.dropped.Load()
	d.mu.RUnlock()
	return s
}

func (d *Decoder) Frame(channel, previousSequence int) (Frame, bool) {
	if channel < 0 || channel > candidateCount {
		return Frame{}, false
	}
	d.mu.RLock()
	source := d.frames[channel]
	if source.Sequence == previousSequence {
		d.mu.RUnlock()
		return Frame{}, false
	}
	result := source
	result.RGB = append([]byte(nil), source.RGB...)
	d.mu.RUnlock()
	return result, true
}

func (d *Decoder) OutputFolder() string { return d.outputFolder }

func (d *Decoder) SavePartial() (string, error) {
	d.mu.RLock()
	frame := d.frames[0]
	if d.status.Candidates {
		best := 1
		for i := 2; i <= candidateCount; i++ {
			if d.frames[i].Lines > d.frames[best].Lines {
				best = i
			}
		}
		frame = d.frames[best]
	}
	frame.RGB = append([]byte(nil), frame.RGB...)
	d.mu.RUnlock()
	if frame.Width <= 0 || frame.Height <= 0 || len(frame.RGB) != frame.Width*frame.Height*3 {
		return "", fmt.Errorf("%s", i18n.Source("text.24ffe7ce2262"))
	}
	if err := os.MkdirAll(d.outputFolder, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(d.outputFolder, "sstv-partial-"+time.Now().Format("20060102-150405")+".png")
	img := image.NewRGBA(image.Rect(0, 0, frame.Width, frame.Height))
	for y := 0; y < frame.Height; y++ {
		for x := 0; x < frame.Width; x++ {
			i := (y*frame.Width + x) * 3
			img.SetRGBA(x, y, color.RGBA{R: frame.RGB[i], G: frame.RGB[i+1], B: frame.RGB[i+2], A: 255})
		}
	}
	file, err := os.Create(path)
	if err == nil {
		err = png.Encode(file, img)
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}
	if err == nil {
		d.mu.Lock()
		d.status.LastSaved = path
		d.mu.Unlock()
	}
	return path, err
}

func (d *Decoder) Close() { d.Configure(false) }

func (d *Decoder) start(forceCandidates bool) error {
	d.mu.Lock()
	if d.running.Load() || !d.enabled {
		d.mu.Unlock()
		return nil
	}
	if err := os.MkdirAll(d.outputFolder, 0o755); err != nil {
		d.setErrorLocked(err)
		d.mu.Unlock()
		return err
	}
	executable, err := filepath.Abs(d.executable)
	if err != nil {
		d.setErrorLocked(err)
		d.mu.Unlock()
		return err
	}
	// Keep the automatic primary decoder and the manual candidates running
	// together. The UI exposes RX1 as AUTO and RX2-RX4 as independently
	// selectable manual decoders.
	args := []string{"--stream", "--sample-rate", "48000", "--output", d.outputFolder, "--weak", "--candidate-modes", strings.Join(d.candidateModes[:], ","), "--force-candidates"}
	if !d.automatic {
		args = append(args, "--forced-mode", d.selectedMode)
	}
	_ = forceCandidates
	cmd := exec.Command(executable, args...)
	cmd.Dir = filepath.Dir(executable)
	cmd.SysProcAttr = hiddenProcessAttributes()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		d.setErrorLocked(err)
		d.mu.Unlock()
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		d.setErrorLocked(err)
		d.mu.Unlock()
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		d.setErrorLocked(err)
		d.mu.Unlock()
		return err
	}
	if err = cmd.Start(); err != nil {
		d.setErrorLocked(err)
		d.mu.Unlock()
		return err
	}
	d.cmd, d.stdin, d.stop = cmd, stdin, make(chan struct{})
	d.generation++
	generation, stop := d.generation, d.stop
	d.running.Store(true)
	d.status.State, d.status.Detail = i18n.Source("text.56f21695a650"), i18n.Source("text.100814c34a99")
	d.syncStatusLocked()
	d.mu.Unlock()
	go d.writer(generation, stop)
	go d.readFrames(stdout, generation)
	go d.readStatus(stderr, generation)
	go func() {
		err := cmd.Wait()
		d.mu.Lock()
		if d.generation == generation {
			d.running.Store(false)
			if d.enabled && !d.restarting {
				d.status.State = i18n.Source("text.d98ee0e5f939")
				d.status.Detail = i18n.Source("text.ce8993627228")
				if err != nil {
					d.status.Detail += ": " + err.Error()
				}
			}
		}
		d.mu.Unlock()
	}()
	return nil
}

func (d *Decoder) writer(generation uint64, stop <-chan struct{}) {
	writer := bufio.NewWriterSize(d.stdin, 16*1024)
	bytes := make([]byte, 8192)
	for {
		select {
		case <-stop:
			_ = writer.Flush()
			return
		case samples := <-d.queue:
			offset := 0
			for _, sample := range samples {
				if offset+2 > len(bytes) {
					if _, err := writer.Write(bytes[:offset]); err != nil {
						d.fail(generation, err)
						d.pool.Put(samples[:0])
						return
					}
					offset = 0
				}
				value := int16(math.Round(float64(min(max(sample, -1), 1) * 32767)))
				binary.LittleEndian.PutUint16(bytes[offset:], uint16(value))
				offset += 2
			}
			if offset > 0 {
				_, _ = writer.Write(bytes[:offset])
			}
			d.pool.Put(samples[:0])
			if err := writer.Flush(); err != nil {
				d.fail(generation, err)
				return
			}
		}
	}
}

func (d *Decoder) readFrames(reader io.Reader, generation uint64) {
	input := bufio.NewReaderSize(reader, 64*1024)
	for {
		header := make([]byte, 16)
		if _, err := io.ReadFull(input, header); err != nil {
			return
		}
		if string(header[:4]) != i18n.Source("text.820d4685bc9d") || header[4] != 1 {
			d.fail(generation, fmt.Errorf("%s", i18n.Source("text.a2a740851591")))
			return
		}
		typeID, channel := header[5], int(binary.LittleEndian.Uint16(header[6:8]))
		payloadSize := int(binary.LittleEndian.Uint32(header[8:12]))
		sequence := int(binary.LittleEndian.Uint32(header[12:16]))
		if channel < 0 || channel > candidateCount || payloadSize < 0 || payloadSize > 2_000_000 {
			d.fail(generation, fmt.Errorf("%s", i18n.Source("text.fe04e15ce616")))
			return
		}
		payload := make([]byte, payloadSize)
		if _, err := io.ReadFull(input, payload); err != nil {
			return
		}
		d.mu.Lock()
		if d.generation != generation {
			d.mu.Unlock()
			return
		}
		switch typeID {
		case 1:
			d.applyFullLocked(channel, sequence, payload)
		case 2:
			if len(payload) == 8 {
				d.status.ToneFrequency = math.Float32frombits(binary.LittleEndian.Uint32(payload[:4]))
				d.status.ToneLevel = math.Float32frombits(binary.LittleEndian.Uint32(payload[4:]))
			}
		case 3:
			d.applyRowsLocked(channel, sequence, payload)
		}
		d.mu.Unlock()
	}
}

func (d *Decoder) applyFullLocked(channel, sequence int, payload []byte) {
	if len(payload) < 16 {
		return
	}
	w, h, lines := int(binary.LittleEndian.Uint32(payload[4:8])), int(binary.LittleEndian.Uint32(payload[8:12])), int(binary.LittleEndian.Uint32(payload[12:16]))
	if w <= 0 || h <= 0 || len(payload)-16 != w*h*3 {
		return
	}
	d.frames[channel] = Frame{Sequence: sequence, Width: w, Height: h, Lines: lines, RGB: append([]byte(nil), payload[16:]...)}
	if channel == 0 {
		d.status.Width, d.status.Height, d.status.Lines = w, h, lines
	}
}

func (d *Decoder) applyRowsLocked(channel, sequence int, payload []byte) {
	if len(payload) < 24 {
		return
	}
	w, h := int(binary.LittleEndian.Uint32(payload[4:8])), int(binary.LittleEndian.Uint32(payload[8:12]))
	first, count, lines := int(binary.LittleEndian.Uint32(payload[12:16])), int(binary.LittleEndian.Uint32(payload[16:20])), int(binary.LittleEndian.Uint32(payload[20:24]))
	if w <= 0 || h <= 0 || first < 0 || count <= 0 || first+count > h || len(payload)-24 != w*count*3 {
		return
	}
	f := &d.frames[channel]
	if f.Width != w || f.Height != h || len(f.RGB) != w*h*3 {
		f.RGB = make([]byte, w*h*3)
	}
	f.Sequence, f.Width, f.Height, f.Lines = sequence, w, h, lines
	copy(f.RGB[first*w*3:(first+count)*w*3], payload[24:])
	if channel == 0 {
		d.status.Width, d.status.Height, d.status.Lines = w, h, lines
	}
}

func (d *Decoder) readStatus(reader io.Reader, generation uint64) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024), 64*1024)
	for scanner.Scan() {
		d.parseStatus(generation, scanner.Text())
	}
}

func (d *Decoder) parseStatus(generation uint64, line string) {
	if !strings.HasPrefix(line, i18n.Source("text.4c476f9afd36")) {
		return
	}
	value := strings.TrimPrefix(line, i18n.Source("text.4c476f9afd36"))
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.generation != generation {
		return
	}
	switch {
	case strings.HasPrefix(value, "STATE "):
		d.status.State = normalizeState(strings.TrimPrefix(value, i18n.Source("text.495cafe389de")))
	case strings.HasPrefix(value, "MODE "):
		parts := strings.Fields(value)
		if len(parts) >= 4 {
			d.status.Mode = strings.ToUpper(strings.Join(parts[1:len(parts)-2], " "))
		}
	case strings.HasPrefix(value, "PROGRESS "):
		parts := strings.Fields(value)
		if len(parts) >= 8 {
			d.status.Progress, _ = strconv.Atoi(parts[1])
			d.status.SyncPercent, _ = strconv.Atoi(parts[5])
			d.status.SyncRMS = parseFloat(parts[7])
		}
	case strings.HasPrefix(value, "CANDIDATE "):
		parts := strings.Fields(value)
		if len(parts) >= 10 {
			i, _ := strconv.Atoi(parts[1])
			if i >= 1 && i <= 4 {
				d.status.CandidateProgress[i-1], _ = strconv.Atoi(parts[3])
				d.status.CandidateSync[i-1], _ = strconv.Atoi(parts[7])
			}
		}
	case strings.HasPrefix(value, "CANDIDATES ACTIVE"):
		d.status.Candidates, d.status.State = true, i18n.Source("text.483d04905f7c")
	case strings.HasPrefix(value, "CANDIDATES INACTIVE"):
		d.status.Candidates = false
	case strings.HasPrefix(value, "WATCHDOG SAVE_PARTIAL"):
		d.status.State = i18n.Source("text.764a55c35afa")
	case strings.HasPrefix(value, "RESYNC "):
		d.status.State = i18n.Source("text.fb0fb383ce84")
	case strings.HasPrefix(value, "COMPLETE "):
		d.status.State, d.status.LastSaved = i18n.Source("text.894981ff0205"), strings.TrimPrefix(value, i18n.Source("text.e5e55040c5e2"))
	case strings.HasPrefix(value, "ERROR "):
		d.status.State, d.status.Detail = i18n.Source("text.d98ee0e5f939"), strings.TrimPrefix(value, i18n.Source("text.db7f988cb492"))
	}
}

func (d *Decoder) stopProcess(state string) {
	d.mu.Lock()
	d.running.Store(false)
	d.generation++
	if d.stop != nil {
		close(d.stop)
		d.stop = nil
	}
	if d.stdin != nil {
		_ = d.stdin.Close()
	}
	if d.cmd != nil && d.cmd.Process != nil {
		_ = d.cmd.Process.Kill()
	}
	d.stdin, d.cmd = nil, nil
	d.status.State = state
	d.mu.Unlock()
	d.drainQueue()
}

func (d *Decoder) fail(generation uint64, err error) {
	d.mu.Lock()
	if d.generation == generation {
		d.status.State, d.status.Detail = i18n.Source("text.d98ee0e5f939"), err.Error()
		if d.cmd != nil && d.cmd.Process != nil {
			_ = d.cmd.Process.Kill()
		}
	}
	d.mu.Unlock()
}

func (d *Decoder) clearFrames() {
	d.mu.Lock()
	for i := range d.frames {
		d.frames[i] = Frame{Sequence: d.frames[i].Sequence + 1}
	}
	d.status.Width, d.status.Height, d.status.Lines, d.status.Progress = 0, 0, 0, 0
	d.mu.Unlock()
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
func (d *Decoder) setErrorLocked(err error) {
	d.status.State, d.status.Detail = i18n.Source("text.d98ee0e5f939"), err.Error()
}
func (d *Decoder) syncStatusLocked() {
	d.status.Automatic, d.status.SelectedMode, d.status.CandidateModes = d.automatic, d.selectedMode, d.candidateModes
}
func validMode(value string) bool {
	for _, mode := range ValidModes {
		if value == mode {
			return true
		}
	}
	return false
}
func normalizeState(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "HUNTING":
		return i18n.Source("text.56f21695a650")
	case "LEADER LOCK":
		return i18n.Source("text.7dcad6823810")
	case "IMAGE RX":
		return i18n.Source("text.fb0fb383ce84")
	case "DONE":
		return i18n.Source("text.894981ff0205")
	}
	return strings.ToUpper(strings.TrimSpace(value))
}
func parseFloat(value string) float32 {
	parsed, _ := strconv.ParseFloat(value, 32)
	return float32(parsed)
}
