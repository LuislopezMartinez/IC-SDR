package screens

import (
	"go-zero/internal/i18n"

	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	shinemp3 "github.com/braheezy/shine-mp3/pkg/mp3"
	"go-zero/internal/resources"
)

const (
	recorderFormatWAV = "WAV"
	recorderFormatMP3 = "MP3"
)

type recorderChunk struct {
	session     uint64
	samples     []int16
	stop        bool
	frequencyHz int64
	band, mode  string
	format      string
}

type RecorderState struct {
	Recording, Paused, WaitingForSquelch bool
	SkipSquelchSilence                   bool
	DurationSeconds                      uint64
	DroppedChunks                        uint64
	PeakDBFS                             float32
	RecentFiles                          []string
	Format                               string
	Encoding                             bool
	LastError                            string
}

type AudioRecorder struct {
	mu                                      sync.Mutex
	queue                                   chan recorderChunk
	done                                    chan struct{}
	directory                               string
	recording, paused, skipSilence, waiting bool
	squelchCapture                          bool
	tailRemaining                           int
	preRoll                                 []int16
	preRollWrite, preRollCount              int
	session                                 uint64
	frequencyHz                             int64
	band, mode, format, activeFormat        string
	recordedSamples, dropped                uint64
	peakDBFS                                float32
	recent                                  []string
	encoding                                bool
	lastError                               string
}

func NewAudioRecorder() *AudioRecorder {
	directory := recorderDirectory()
	return newAudioRecorder(directory)
}

func recorderDirectory() string {
	return resources.WritablePath("recordings")
}

func newAudioRecorder(directory string) *AudioRecorder {
	_ = os.MkdirAll(directory, 0o755)
	r := &AudioRecorder{queue: make(chan recorderChunk, 256), done: make(chan struct{}), directory: directory, format: recorderFormatMP3,
		preRoll: make([]int16, audioSampleRate*250/1000), peakDBFS: -60}
	go r.writeLoop()
	return r
}

func (r *AudioRecorder) SetFormat(format string) {
	format = strings.ToUpper(format)
	if format != recorderFormatWAV && format != recorderFormatMP3 {
		return
	}
	r.mu.Lock()
	if !r.recording {
		r.format = format
	}
	r.mu.Unlock()
}

func (r *AudioRecorder) Configure(frequencyHz int64, band, mode string) {
	r.mu.Lock()
	r.frequencyHz, r.band, r.mode = frequencyHz, safeFilePart(band), safeFilePart(mode)
	r.mu.Unlock()
}

func (r *AudioRecorder) Start() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.recording {
		return
	}
	r.session++
	r.recording, r.paused = true, false
	r.activeFormat = r.format
	r.recordedSamples, r.peakDBFS = 0, -60
	r.lastError = ""
	r.resetSquelchLocked()
}

func (r *AudioRecorder) Stop() {
	r.mu.Lock()
	if !r.recording {
		r.mu.Unlock()
		return
	}
	session := r.session
	r.recording, r.paused = false, false
	r.resetSquelchLocked()
	r.mu.Unlock()
	r.enqueue(recorderChunk{session: session, stop: true})
}

func (r *AudioRecorder) TogglePause() {
	r.mu.Lock()
	if r.recording {
		r.paused = !r.paused
		r.resetSquelchLocked()
	}
	r.mu.Unlock()
}

func (r *AudioRecorder) SetSkipSquelchSilence(enabled bool) {
	r.mu.Lock()
	if r.skipSilence != enabled {
		r.skipSilence = enabled
		r.resetSquelchLocked()
	}
	r.mu.Unlock()
}

func (r *AudioRecorder) Submit(samples []float32, squelchEnabled, squelchOpen bool) {
	if len(samples) == 0 {
		return
	}
	pcm := make([]int16, len(samples))
	peak := float32(0)
	for i, sample := range samples {
		sample = min(max(sample, -.98), .98)
		pcm[i] = int16(math.Round(float64(sample * 32767)))
		peak = max(peak, absFloat32(sample))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.recording || r.paused {
		return
	}
	if peak > 0 {
		r.peakDBFS = 20 * float32(math.Log10(float64(peak)))
	} else {
		r.peakDBFS = -60
	}
	if !r.skipSilence || !squelchEnabled {
		r.enqueueLocked(pcm)
		r.waiting = false
		r.resetCaptureLocked()
		return
	}
	if squelchOpen {
		if !r.squelchCapture {
			r.flushPreRollLocked()
		}
		r.squelchCapture, r.waiting = true, false
		r.tailRemaining = audioSampleRate * 200 / 1000
		r.enqueueLocked(pcm)
		return
	}
	r.waiting = true
	offset := 0
	if r.squelchCapture && r.tailRemaining > 0 {
		count := min(len(pcm), r.tailRemaining)
		r.enqueueLocked(pcm[:count])
		offset = count
		r.tailRemaining -= count
	}
	if r.tailRemaining <= 0 {
		r.squelchCapture = false
	}
	for _, sample := range pcm[offset:] {
		r.preRoll[r.preRollWrite] = sample
		r.preRollWrite = (r.preRollWrite + 1) % len(r.preRoll)
		r.preRollCount = min(r.preRollCount+1, len(r.preRoll))
	}
}

func (r *AudioRecorder) State() RecorderState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return RecorderState{
		Recording: r.recording, Paused: r.paused, WaitingForSquelch: r.waiting,
		SkipSquelchSilence: r.skipSilence, DurationSeconds: r.recordedSamples / audioSampleRate,
		DroppedChunks: r.dropped, PeakDBFS: r.peakDBFS, RecentFiles: append([]string(nil), r.recent...),
		Format: r.format, Encoding: r.encoding, LastError: r.lastError,
	}
}
func (r *AudioRecorder) Directory() string { return r.directory }

func (r *AudioRecorder) DeleteFile(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(r.directory, absolute)
	if err != nil || relative == ".." || len(relative) >= 3 && relative[:3] == ".."+string(os.PathSeparator) {
		return fmt.Errorf("%s", i18n.Source("text.7f169d39fbfd"))
	}
	if err = os.Remove(absolute); err != nil {
		return err
	}
	r.mu.Lock()
	for i, recent := range r.recent {
		if recent == absolute {
			r.recent = append(r.recent[:i], r.recent[i+1:]...)
			break
		}
	}
	r.mu.Unlock()
	return nil
}

func (r *AudioRecorder) Close() { r.Stop(); close(r.queue); <-r.done }

func (r *AudioRecorder) enqueueLocked(samples []int16) {
	copySamples := append([]int16(nil), samples...)
	select {
	case r.queue <- recorderChunk{session: r.session, samples: copySamples, frequencyHz: r.frequencyHz, band: r.band, mode: r.mode, format: r.activeFormat}:
		r.recordedSamples += uint64(len(copySamples))
	default:
		r.dropped++
	}
}
func (r *AudioRecorder) enqueue(chunk recorderChunk) {
	select {
	case r.queue <- chunk:
	default:
		r.mu.Lock()
		r.dropped++
		r.mu.Unlock()
	}
}
func (r *AudioRecorder) flushPreRollLocked() {
	if r.preRollCount == 0 {
		return
	}
	samples := make([]int16, r.preRollCount)
	start := (r.preRollWrite - r.preRollCount + len(r.preRoll)) % len(r.preRoll)
	for i := range samples {
		samples[i] = r.preRoll[(start+i)%len(r.preRoll)]
	}
	r.enqueueLocked(samples)
	r.preRollCount, r.preRollWrite = 0, 0
}
func (r *AudioRecorder) resetCaptureLocked() {
	r.preRollCount, r.preRollWrite, r.tailRemaining = 0, 0, 0
	r.squelchCapture = false
}
func (r *AudioRecorder) resetSquelchLocked() { r.resetCaptureLocked(); r.waiting = false }

func (r *AudioRecorder) writeLoop() {
	defer close(r.done)
	var file *os.File
	var session uint64
	var dataBytes uint32
	var path, finalPath, format string
	closeFile := func() {
		if file == nil {
			return
		}
		_, _ = file.Seek(0, 0)
		_ = writeWAVHeader(file, dataBytes)
		_ = file.Close()
		completedPath := path
		if format == recorderFormatMP3 {
			r.mu.Lock()
			r.encoding = true
			r.mu.Unlock()
			if err := encodeWAVToMP3(path, finalPath); err != nil {
				fallback := strings.TrimSuffix(path, ".part")
				if renameErr := os.Rename(path, fallback); renameErr == nil {
					completedPath = fallback
				}
				r.mu.Lock()
				r.lastError = i18n.Source("text.792fa167e3c4") + err.Error()
				r.mu.Unlock()
			} else {
				completedPath = finalPath
				_ = os.Remove(path)
			}
		}
		r.mu.Lock()
		r.encoding = false
		r.recent = append([]string{completedPath}, r.recent...)
		if len(r.recent) > 30 {
			r.recent = r.recent[:30]
		}
		r.mu.Unlock()
		file = nil
		dataBytes = 0
	}
	for chunk := range r.queue {
		if chunk.stop {
			if chunk.session == session {
				closeFile()
			}
			continue
		}
		if file == nil || chunk.session != session {
			closeFile()
			session = chunk.session
			format = chunk.format
			finalPath = r.uniquePath(chunk)
			path = finalPath
			if format == recorderFormatMP3 {
				path += ".wav.part"
			}
			file, _ = os.Create(path)
			if file != nil {
				_ = writeWAVHeader(file, 0)
			}
		}
		if file == nil {
			continue
		}
		_ = binary.Write(file, binary.LittleEndian, chunk.samples)
		dataBytes += uint32(len(chunk.samples) * 2)
	}
	closeFile()
}

func (r *AudioRecorder) uniquePath(chunk recorderChunk) string {
	stamp := time.Now().Format("20060102-150405")
	mhz := fmt.Sprintf("%.6fMHz", float64(chunk.frequencyHz)/1e6)
	extension := "." + strings.ToLower(chunk.format)
	name := fmt.Sprintf("%s_%s_%s_%s%s", stamp, chunk.band, safeFilePart(mhz), chunk.mode, extension)
	path := filepath.Join(r.directory, name)
	for suffix := 2; ; suffix++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path
		}
		path = filepath.Join(r.directory, fmt.Sprintf("%s_%s_%s_%s-%d%s", stamp, chunk.band, safeFilePart(mhz), chunk.mode, suffix, extension))
	}
}

func encodeWAVToMP3(wavPath, mp3Path string) (err error) {
	input, err := os.Open(wavPath)
	if err != nil {
		return err
	}
	defer input.Close()
	if _, err = input.Seek(44, io.SeekStart); err != nil {
		return err
	}
	output, err := os.Create(mp3Path)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := output.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(mp3Path)
		}
	}()

	encoder := shinemp3.NewEncoder(audioSampleRate, 1)
	pcmBytes := make([]byte, shinemp3.SHINE_MAX_SAMPLES*2)
	samples := make([]int16, shinemp3.SHINE_MAX_SAMPLES)
	for {
		n, readErr := io.ReadFull(input, pcmBytes)
		if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
			return readErr
		}
		if n == 0 {
			break
		}
		for i := range samples {
			samples[i] = 0
		}
		for i := 0; i < n/2; i++ {
			samples[i] = int16(binary.LittleEndian.Uint16(pcmBytes[i*2:]))
		}
		encoded, written := encoder.EncodeBufferInterleaved(samples)
		if _, err = output.Write(encoded[:written]); err != nil {
			return err
		}
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}
	}
	return nil
}

var unsafeFilePart = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

func safeFilePart(value string) string {
	value = unsafeFilePart.ReplaceAllString(value, "-")
	if value == "" {
		return "unknown"
	}
	return value
}
func writeWAVHeader(file *os.File, bytes uint32) error {
	header := struct {
		RIFF             [4]byte
		Size             uint32
		WAVE             [4]byte
		FMT              [4]byte
		FmtSize          uint32
		Format, Channels uint16
		Rate, ByteRate   uint32
		Align, Bits      uint16
		Data             [4]byte
		DataSize         uint32
	}{[4]byte{'R', 'I', 'F', 'F'}, 36 + bytes, [4]byte{'W', 'A', 'V', 'E'}, [4]byte{'f', 'm', 't', ' '}, 16, 1, 1, audioSampleRate, audioSampleRate * 2, 2, 16, [4]byte{'d', 'a', 't', 'a'}, bytes}
	return binary.Write(file, binary.LittleEndian, &header)
}
