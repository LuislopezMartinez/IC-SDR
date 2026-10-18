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
	"sort"
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
	LineQuality                    []uint8
}

type SavedImage struct {
	Path, EnhancedPath, Mode, Reason, Quality                         string
	SavedAt                                                           time.Time
	Lines, Height, CorrectedLines, ReconstructedLines, DenoisedPixels int
	Partial                                                           bool
}

type Status struct {
	Running, Automatic, Candidates                      bool
	State, Mode, SelectedMode, LastSaved, Detail        string
	Width, Height, Lines, Progress, SyncPercent, Queued int
	TimeoutRemaining                                    int
	SyncRMS, ToneFrequency, ToneLevel                   float32
	Dropped                                             uint64
	CandidateModes                                      [candidateCount]string
	CandidateProgress, CandidateSync                    [candidateCount]int
	History                                             []SavedImage
}

type Decoder struct {
	executable, outputFolder string
	queue                    chan []float32
	pool                     sync.Pool
	running                  atomic.Bool
	dropped                  atomic.Uint64
	reconstructLines         atomic.Bool
	reduceNoise              atomic.Bool

	mu                sync.RWMutex
	enabled           bool
	automatic         bool
	selectedMode      string
	candidateModes    [candidateCount]string
	status            Status
	frames            [candidateCount + 1]Frame
	cmd               *exec.Cmd
	stdin             io.WriteCloser
	stop              chan struct{}
	generation        uint64
	restarting        bool
	visResetPending   bool
	visLeaderAt       time.Time
	visCandidateAt    time.Time
	pendingVISFrame   Frame
	pendingVISMode    string
	lastArchivedFrame string
	receptionActive   bool
	receptionStarted  time.Time
	lastProgressAt    time.Time
	lastProgressLines int
	receptionDeadline time.Time
}

func New(executable, outputFolder string) *Decoder {
	d := &Decoder{executable: executable, outputFolder: outputFolder, queue: make(chan []float32, 16), automatic: true, selectedMode: "R36", candidateModes: [4]string{"R36", "R72", "M1", "S1"}}
	d.pool.New = func() any { return make([]float32, 0, 4096) }
	d.reconstructLines.Store(true)
	d.reduceNoise.Store(true)
	d.status.State, d.status.Mode, d.status.ToneLevel = i18n.Source("text.38cca6bea010"), i18n.Source("text.80e4cf374a8a"), -120
	d.syncStatusLocked()
	return d
}

func (d *Decoder) SetReconstructLines(enabled bool) { d.reconstructLines.Store(enabled) }
func (d *Decoder) SetReduceNoise(enabled bool)      { d.reduceNoise.Store(enabled) }

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
	s.History = append([]SavedImage(nil), d.status.History...)
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
	result.LineQuality = append([]uint8(nil), source.LineQuality...)
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
	frame.LineQuality = append([]uint8(nil), frame.LineQuality...)
	d.mu.RUnlock()
	if frame.Width <= 0 || frame.Height <= 0 || len(frame.RGB) != frame.Width*frame.Height*3 {
		return "", fmt.Errorf("%s", i18n.Source("text.24ffe7ce2262"))
	}
	path, err := d.saveFramePNG(frame, "sstv-partial")
	if err == nil {
		enhancedPath, corrected, reconstructed, denoised, _ := d.saveEnhancedFramePNG(frame, path)
		d.mu.Lock()
		d.status.LastSaved = path
		d.recordSavedLocked(path, enhancedPath, frame, corrected, reconstructed, denoised, "Guardado manual", d.status.Mode)
		d.mu.Unlock()
	}
	return path, err
}

func (d *Decoder) saveFramePNG(frame Frame, prefix string) (string, error) {
	if frame.Width <= 0 || frame.Height <= 0 || len(frame.RGB) != frame.Width*frame.Height*3 {
		return "", fmt.Errorf("%s", i18n.Source("text.24ffe7ce2262"))
	}
	if err := os.MkdirAll(d.outputFolder, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(d.outputFolder, prefix+"-"+time.Now().Format("20060102-150405.000000000")+".png")
	return path, writeSSTVFrameWithMetadata(frame, path)
}

func writeSSTVFrameWithMetadata(frame Frame, path string) error {
	if err := writeSSTVFramePNG(frame, path); err != nil {
		return err
	}
	return writeSSTVFrameMetadata(path, frame)
}

func writeSSTVFramePNG(frame Frame, path string) error {
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
	return err
}

func (d *Decoder) saveEnhancedFramePNG(frame Frame, originalPath string) (string, int, int, int, error) {
	enhanced, corrected := alignSSTVRows(frame)
	reconstructed := 0
	if d.reconstructLines.Load() {
		enhanced, reconstructed = reconstructSSTVLines(enhanced, 6)
	}
	denoised := 0
	if d.reduceNoise.Load() {
		enhanced, denoised = reduceSSTVImpulseNoise(enhanced)
	}
	if corrected == 0 && reconstructed == 0 && denoised == 0 {
		return "", 0, 0, 0, nil
	}
	path := strings.TrimSuffix(originalPath, filepath.Ext(originalPath)) + "-mejorada.png"
	return path, corrected, reconstructed, denoised, writeSSTVFrameWithMetadata(enhanced, path)
}

func (d *Decoder) Close() { d.Configure(false) }

func (d *Decoder) runtimeArgs(forceStart bool) []string {
	args := []string{"--stream", "--sample-rate", "48000", "--output", d.outputFolder, "--weak", "--candidate-modes", strings.Join(d.candidateModes[:], ","), "--force-candidates"}
	if !d.automatic || forceStart {
		args = append(args, "--forced-mode", d.selectedMode)
	}
	if forceStart {
		args = append(args, "--force-start")
	}
	return args
}

func (d *Decoder) start(forceStart bool) error {
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
	args := d.runtimeArgs(forceStart)
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
	go d.watchReception(generation, stop)
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
	quality := make([]uint8, h)
	for row := 0; row < min(lines, h); row++ {
		quality[row] = 1
	}
	d.frames[channel] = Frame{Sequence: sequence, Width: w, Height: h, Lines: lines, RGB: append([]byte(nil), payload[16:]...), LineQuality: quality}
	if channel == 0 {
		d.status.Width, d.status.Height, d.status.Lines = w, h, lines
		d.noteProgressLocked(lines)
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
		f.LineQuality = make([]uint8, h)
	}
	f.Sequence, f.Width, f.Height, f.Lines = sequence, w, h, lines
	copy(f.RGB[first*w*3:(first+count)*w*3], payload[24:])
	for row := first; row < first+count; row++ {
		f.LineQuality[row] = 1
	}
	if channel == 0 {
		d.status.Width, d.status.Height, d.status.Lines = w, h, lines
		d.noteProgressLocked(lines)
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
	if strings.HasPrefix(value, "PRIMARY RESET ") {
		d.handlePrimaryReset(generation, strings.TrimSpace(strings.TrimPrefix(value, "PRIMARY RESET ")))
		return
	}
	if strings.HasPrefix(value, "MODE ") {
		d.handleMode(generation, value)
		return
	}
	if strings.HasPrefix(value, "WATCHDOG SAVE_PARTIAL") {
		d.handleReceptionTimeout(generation, "Señal perdida / timeout")
		return
	}
	if strings.HasPrefix(value, "COMPLETE ") {
		d.mu.Lock()
		if d.generation == generation {
			d.receptionActive = false
			d.status.TimeoutRemaining = 0
		}
		d.mu.Unlock()
		d.archiveCurrentFrame(generation, "sstv-complete", i18n.Source("text.894981ff0205"), "Completada")
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.generation != generation {
		return
	}
	switch {
	case strings.HasPrefix(value, "STATE "):
		raw := strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(value, i18n.Source("text.495cafe389de"))))
		now := time.Now()
		if raw == "LEADER LOCK" {
			d.visLeaderAt = now
		}
		if raw == "VIS" {
			if !d.visLeaderAt.IsZero() && now.Sub(d.visLeaderAt) <= 4*time.Second {
				d.visCandidateAt = now
			} else {
				d.visCandidateAt = time.Time{}
			}
		}
		d.status.State = normalizeState(raw)
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
	case strings.HasPrefix(value, "RESYNC "):
		d.status.State = i18n.Source("text.fb0fb383ce84")
	case strings.HasPrefix(value, "ERROR "):
		d.status.State, d.status.Detail = i18n.Source("text.d98ee0e5f939"), strings.TrimPrefix(value, i18n.Source("text.db7f988cb492"))
	}
}

func (d *Decoder) handleReceptionTimeout(generation uint64, reason string) {
	d.mu.Lock()
	if d.generation != generation {
		d.mu.Unlock()
		return
	}
	d.receptionActive = false
	d.status.TimeoutRemaining = 0
	d.status.State = "SEÑAL PERDIDA"
	d.status.Detail = reason + "; guardando parcial y reiniciando receptores"
	d.mu.Unlock()
	d.archiveCurrentFrame(generation, "sstv-partial-timeout", "SEÑAL PERDIDA", reason)
	// Restarting the child is intentionally the recovery boundary: its audio
	// queue, AUTO VIS decoder and all four manual candidates start empty.
	d.Restart(false)
}

func (d *Decoder) handlePrimaryReset(generation uint64, reason string) {
	d.mu.Lock()
	if d.generation != generation {
		d.mu.Unlock()
		return
	}
	if reason == "VIS_LOCKED" {
		now := time.Now()
		if d.visCandidateAt.IsZero() || now.Sub(d.visCandidateAt) > 3*time.Second {
			d.status.Detail = "VIS descartado: falta una secuencia leader/VIS válida"
			d.mu.Unlock()
			return
		}
		d.pendingVISFrame = d.bestFrameLocked()
		d.pendingVISFrame.RGB = append([]byte(nil), d.pendingVISFrame.RGB...)
		d.pendingVISFrame.LineQuality = append([]uint8(nil), d.pendingVISFrame.LineQuality...)
		d.pendingVISMode = d.status.Mode
		d.visResetPending = true
		d.status.State = "VALIDANDO NUEVO VIS"
		d.status.Detail = "Esperando la confirmación del modo antes de sustituir la imagen"
		d.mu.Unlock()
		return
	}
	d.clearFramesLocked()
	d.mu.Unlock()
}

func (d *Decoder) handleMode(generation uint64, value string) {
	parts := strings.Fields(value)
	if len(parts) < 4 {
		return
	}
	mode := strings.ToUpper(strings.Join(parts[1:len(parts)-2], " "))
	d.mu.Lock()
	if d.generation != generation {
		d.mu.Unlock()
		return
	}
	frame, oldMode := d.pendingVISFrame, d.pendingVISMode
	commit := d.visResetPending && !d.visCandidateAt.IsZero() && time.Since(d.visCandidateAt) <= 4*time.Second
	// A watchdog may already have archived this exact partial frame before a
	// later VIS confirms the next image. Preserve that knowledge until the old
	// frame has been assessed, then reset it for the new reception.
	archivePrevious := commit && frame.Lines > 0 && sstvFrameSignature(frame) != d.lastArchivedFrame
	if commit {
		d.clearFramesLocked()
		d.armReceptionWatchdogLocked(mode)
	}
	d.visResetPending = false
	d.pendingVISFrame, d.pendingVISMode = Frame{}, ""
	d.visLeaderAt, d.visCandidateAt = time.Time{}, time.Time{}
	d.lastArchivedFrame = ""
	d.status.Mode = mode
	d.status.State = i18n.Source("text.fb0fb383ce84")
	d.status.Detail = "VIS confirmado; iniciando una imagen limpia"
	d.mu.Unlock()
	if archivePrevious {
		d.archiveFrame(generation, frame, "sstv-partial-vis", "Interrumpida por nuevo VIS", oldMode)
	}
}

// watchReception is deliberately owned by the Go side of IC-SDR.  The
// external decoder has its own timeout, but this second guard makes sure a
// stalled child process cannot leave the panel indefinitely at SIGNAL LOST.
func (d *Decoder) watchReception(generation uint64, stop <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case now := <-ticker.C:
			reason := ""
			d.mu.Lock()
			if d.generation == generation && d.receptionActive {
				switch {
				case !d.receptionDeadline.IsZero() && !now.Before(d.receptionDeadline):
					reason = "Tiempo máximo del modo agotado"
				case !d.lastProgressAt.IsZero() && now.Sub(d.lastProgressAt) >= 15*time.Second:
					reason = "Sin líneas nuevas durante 15 s"
				}
				if reason != "" {
					d.receptionActive = false
					d.status.State = "SEÑAL PERDIDA"
					d.status.Detail = reason + "; guardando parcial y reiniciando receptores"
					d.status.TimeoutRemaining = 0
				} else if !d.receptionDeadline.IsZero() {
					d.status.TimeoutRemaining = max(0, int(time.Until(d.receptionDeadline).Seconds()))
				}
			}
			d.mu.Unlock()
			if reason != "" {
				d.handleReceptionTimeout(generation, reason)
				return
			}
		}
	}
}

func (d *Decoder) noteProgressLocked(lines int) {
	if !d.receptionActive || lines <= d.lastProgressLines {
		return
	}
	d.lastProgressLines = lines
	d.lastProgressAt = time.Now()
}

func (d *Decoder) armReceptionWatchdogLocked(mode string) {
	now := time.Now()
	d.receptionActive = true
	d.receptionStarted = now
	d.lastProgressAt = now
	d.lastProgressLines = 0
	d.receptionDeadline = now.Add(sstvExpectedDuration(mode) + 15*time.Second)
	d.status.TimeoutRemaining = max(0, int(time.Until(d.receptionDeadline).Seconds()))
}

// sstvExpectedDuration returns a conservative maximum image duration.  The
// extra grace period is applied by armReceptionWatchdogLocked, so a real
// transmission is not cut off for small clock or synchronization variations.
func sstvExpectedDuration(mode string) time.Duration {
	upper := strings.ToUpper(strings.TrimSpace(mode))
	switch {
	case strings.Contains(upper, "MARTIN 1"):
		return 114 * time.Second
	case strings.Contains(upper, "MARTIN 2"):
		return 58 * time.Second
	case strings.Contains(upper, "SCOTTIE S1"):
		return 110 * time.Second
	case strings.Contains(upper, "SCOTTIE S2"):
		return 71 * time.Second
	case strings.Contains(upper, "ROBOT 36"):
		return 36 * time.Second
	case strings.Contains(upper, "ROBOT 72"):
		return 72 * time.Second
	case strings.Contains(upper, "PD50"):
		return 50 * time.Second
	case strings.Contains(upper, "PD90"):
		return 90 * time.Second
	case strings.Contains(upper, "PD120"):
		return 120 * time.Second
	case strings.Contains(upper, "PD160"):
		return 160 * time.Second
	case strings.Contains(upper, "PD180"):
		return 180 * time.Second
	case strings.Contains(upper, "PD240"):
		return 240 * time.Second
	case strings.Contains(upper, "PD290"):
		return 290 * time.Second
	default:
		// Unknown modes remain bounded too, rather than hanging forever.
		return 120 * time.Second
	}
}

func (d *Decoder) archiveCurrentFrame(generation uint64, prefix, state, reason string) {
	d.mu.Lock()
	if d.generation != generation {
		d.mu.Unlock()
		return
	}
	frame := d.bestFrameLocked()
	frame.RGB = append([]byte(nil), frame.RGB...)
	frame.LineQuality = append([]uint8(nil), frame.LineQuality...)
	signature := sstvFrameSignature(frame)
	if frame.Lines <= 0 || frame.Width <= 0 || frame.Height <= 0 || signature == d.lastArchivedFrame {
		d.status.State = state
		d.mu.Unlock()
		return
	}
	d.lastArchivedFrame = signature
	d.status.State = state
	d.status.Detail = "Guardando la imagen recibida en PNG"
	mode := d.status.Mode
	d.mu.Unlock()
	d.archiveFrame(generation, frame, prefix, reason, mode)
	d.mu.Lock()
	if d.generation == generation {
		d.status.State = state
	}
	d.mu.Unlock()
}

func (d *Decoder) archiveFrame(generation uint64, frame Frame, prefix, reason, mode string) {
	path, err := d.saveFramePNG(frame, prefix)
	enhancedPath, corrected, reconstructed, denoised := "", 0, 0, 0
	if err == nil {
		enhancedPath, corrected, reconstructed, denoised, err = d.saveEnhancedFramePNG(frame, path)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.generation != generation {
		return
	}
	if err != nil {
		d.status.Detail = "No se pudo guardar la imagen recibida: " + err.Error()
		return
	}
	d.status.LastSaved = path
	d.recordSavedLocked(path, enhancedPath, frame, corrected, reconstructed, denoised, reason, mode)
	if frame.Lines < frame.Height {
		d.status.Detail = fmt.Sprintf("Imagen parcial guardada en PNG (%d/%d líneas)", frame.Lines, frame.Height)
	} else {
		d.status.Detail = "Imagen completa guardada en PNG"
	}
}

func (d *Decoder) recordSavedLocked(path, enhancedPath string, frame Frame, corrected, reconstructed, denoised int, reason, mode string) {
	item := SavedImage{
		Path: path, EnhancedPath: enhancedPath, Mode: mode, Reason: reason, Quality: sstvQualityLabel(frame, corrected, reconstructed), SavedAt: time.Now(),
		Lines: frame.Lines, Height: frame.Height, CorrectedLines: corrected, ReconstructedLines: reconstructed, DenoisedPixels: denoised, Partial: frame.Lines < frame.Height,
	}
	d.status.History = append([]SavedImage{item}, d.status.History...)
	if len(d.status.History) > 12 {
		d.status.History = d.status.History[:12]
	}
}

func sstvQualityLabel(frame Frame, corrected, reconstructed int) string {
	if frame.Height <= 0 {
		return "MUY INCOMPLETA"
	}
	received := min(max(frame.Lines, 0), frame.Height)
	ratio := float64(received) / float64(frame.Height)
	altered := corrected + reconstructed
	switch {
	case ratio >= .95 && altered <= max(4, frame.Height/20):
		return "BUENA"
	case ratio >= .75:
		return "ACEPTABLE"
	case ratio >= .40:
		return "DEGRADADA"
	default:
		return "MUY INCOMPLETA"
	}
}

// alignSSTVRows performs a deliberately conservative correction. A row is
// moved only when a nearby horizontal offset reduces its RGB difference to
// the preceding valid row by at least 30 percent. The received frame remains
// untouched; this function operates on a copy used for the enhanced PNG.
func alignSSTVRows(frame Frame) (Frame, int) {
	result := frame
	result.RGB = append([]byte(nil), frame.RGB...)
	result.LineQuality = append([]uint8(nil), frame.LineQuality...)
	if frame.Width < 24 || frame.Height < 2 || len(frame.RGB) != frame.Width*frame.Height*3 {
		return result, 0
	}
	quality := frame.LineQuality
	if len(quality) != frame.Height {
		quality = make([]uint8, frame.Height)
		for row := 0; row < min(frame.Lines, frame.Height); row++ {
			quality[row] = 1
		}
		result.LineQuality = append([]uint8(nil), quality...)
	}
	corrected := 0
	for row := 1; row < min(frame.Lines, frame.Height); row++ {
		if quality[row] == 0 || quality[row-1] == 0 {
			continue
		}
		previous := result.RGB[(row-1)*frame.Width*3 : row*frame.Width*3]
		current := frame.RGB[row*frame.Width*3 : (row+1)*frame.Width*3]
		baseline := sstvRowDifference(previous, current, frame.Width, 0)
		bestCost, bestShift := baseline, 0
		for shift := -6; shift <= 6; shift++ {
			if shift == 0 {
				continue
			}
			if cost := sstvRowDifference(previous, current, frame.Width, shift); cost < bestCost {
				bestCost, bestShift = cost, shift
			}
		}
		if bestShift == 0 || baseline < uint64(frame.Width*6) || bestCost*100 >= baseline*70 {
			continue
		}
		destination := result.RGB[row*frame.Width*3 : (row+1)*frame.Width*3]
		for x := 0; x < frame.Width; x++ {
			sourceX := min(max(x+bestShift, 0), frame.Width-1)
			copy(destination[x*3:x*3+3], current[sourceX*3:sourceX*3+3])
		}
		result.LineQuality[row] = 2
		corrected++
	}
	return result, corrected
}

// reconstructSSTVLines fills only short, bounded gaps between two genuinely
// received lines. Long gaps and missing image tails remain untouched so the
// enhanced image never invents a substantial part of the transmission.
func reconstructSSTVLines(frame Frame, maximumGap int) (Frame, int) {
	result := frame
	result.RGB = append([]byte(nil), frame.RGB...)
	result.LineQuality = append([]uint8(nil), frame.LineQuality...)
	if maximumGap <= 0 || frame.Width <= 0 || frame.Height < 3 || len(frame.LineQuality) != frame.Height {
		return result, 0
	}
	reconstructed := 0
	limit := min(frame.Lines, frame.Height)
	for row := 1; row < limit-1; {
		if frame.LineQuality[row] != 0 {
			row++
			continue
		}
		start := row
		for row < limit && frame.LineQuality[row] == 0 {
			row++
		}
		end := row
		gap := end - start
		if gap > maximumGap || end >= limit || frame.LineQuality[start-1] == 0 || frame.LineQuality[end] == 0 {
			continue
		}
		before := result.RGB[(start-1)*frame.Width*3 : start*frame.Width*3]
		after := result.RGB[end*frame.Width*3 : (end+1)*frame.Width*3]
		for missing := 0; missing < gap; missing++ {
			weightAfter := missing + 1
			weightBefore := gap + 1 - weightAfter
			destination := result.RGB[(start+missing)*frame.Width*3 : (start+missing+1)*frame.Width*3]
			for index := range destination {
				destination[index] = byte((int(before[index])*weightBefore + int(after[index])*weightAfter) / (gap + 1))
			}
			result.LineQuality[start+missing] = 3
			reconstructed++
		}
	}
	return result, reconstructed
}

// reduceSSTVImpulseNoise removes isolated salt-and-pepper pixels only when at
// least six neighbours agree around a very different centre pixel. Requiring
// that consensus preserves ordinary edges, thin lettering and image detail.
func reduceSSTVImpulseNoise(frame Frame) (Frame, int) {
	result := frame
	result.RGB = append([]byte(nil), frame.RGB...)
	if frame.Width < 3 || frame.Height < 3 || len(frame.RGB) != frame.Width*frame.Height*3 {
		return result, 0
	}
	limit := min(frame.Lines, frame.Height)
	changed := 0
	for y := 1; y < limit-1; y++ {
		if len(frame.LineQuality) == frame.Height && frame.LineQuality[y] == 0 {
			continue
		}
		for x := 1; x < frame.Width-1; x++ {
			var samples [3][9]int
			index := 0
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					pixel := ((y+dy)*frame.Width + x + dx) * 3
					for channel := 0; channel < 3; channel++ {
						samples[channel][index] = int(frame.RGB[pixel+channel])
					}
					index++
				}
			}
			median := [3]int{}
			for channel := 0; channel < 3; channel++ {
				sort.Ints(samples[channel][:])
				median[channel] = samples[channel][4]
			}
			centre := (y*frame.Width + x) * 3
			difference := 0
			for channel := 0; channel < 3; channel++ {
				difference += absInt(int(frame.RGB[centre+channel]) - median[channel])
			}
			if difference < 150 {
				continue
			}
			consensus, similarCentre := 0, 0
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					pixel := ((y+dy)*frame.Width + x + dx) * 3
					medianDistance, centreDistance := 0, 0
					for channel := 0; channel < 3; channel++ {
						medianDistance += absInt(int(frame.RGB[pixel+channel]) - median[channel])
						centreDistance += absInt(int(frame.RGB[pixel+channel]) - int(frame.RGB[centre+channel]))
					}
					if medianDistance <= 75 {
						consensus++
					}
					if centreDistance <= 75 {
						similarCentre++
					}
				}
			}
			if consensus < 6 || similarCentre > 1 {
				continue
			}
			for channel := 0; channel < 3; channel++ {
				result.RGB[centre+channel] = byte(median[channel])
			}
			changed++
		}
	}
	return result, changed
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func sstvRowDifference(previous, current []byte, width, shift int) uint64 {
	start, end := 6, width-6
	var difference uint64
	for x := start; x < end; x += 2 {
		sourceX := x + shift
		if sourceX < 0 || sourceX >= width {
			continue
		}
		for channel := 0; channel < 3; channel++ {
			a, b := int(previous[x*3+channel]), int(current[sourceX*3+channel])
			difference += uint64(max(a-b, b-a))
		}
	}
	return difference
}

func sstvFrameSignature(frame Frame) string {
	return fmt.Sprintf("%d:%d:%d:%d", frame.Sequence, frame.Width, frame.Height, frame.Lines)
}

func (d *Decoder) bestFrameLocked() Frame {
	frame := d.frames[0]
	if d.status.Candidates {
		for i := 1; i <= candidateCount; i++ {
			if d.frames[i].Lines > frame.Lines {
				frame = d.frames[i]
			}
		}
	}
	return frame
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
	d.receptionActive = false
	d.receptionStarted, d.lastProgressAt, d.receptionDeadline = time.Time{}, time.Time{}, time.Time{}
	d.lastProgressLines, d.status.TimeoutRemaining = 0, 0
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
	d.clearFramesLocked()
	d.mu.Unlock()
}
func (d *Decoder) clearFramesLocked() {
	for i := range d.frames {
		d.frames[i] = Frame{Sequence: d.frames[i].Sequence + 1}
	}
	d.status.Width, d.status.Height, d.status.Lines, d.status.Progress = 0, 0, 0, 0
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
