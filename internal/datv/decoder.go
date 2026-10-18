package datv

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Settings struct {
	Standard      string
	SymbolRate    int
	BandwidthHz   int64
	OffsetHz      int64
	FEC           string
	Inverted      bool
	StartPlayer   bool
	RecordingPath string
}

type Point struct{ I, Q float32 }

type Status struct {
	Available, PlayerAvailable, Running, Locked, TSValid, PlayerRunning, Recording    bool
	State, Detail, Standard, Constellation, FEC, Service, Video, Audio, RecordingPath string
	MER, CNR, FrequencyErrorHz                                                        float32
	AFCCorrectionHz                                                                   float32
	AFCState                                                                          string
	Points                                                                            []Point
	Bitrate                                                                           uint64
	InputQueued, InputCapacity                                                        int
	InputRate, DecoderRate                                                            int
	SymbolRate, AutoAttempt, AutoAttempts                                             int
	SpectrumInverted, AutoMode                                                        bool
	Dropped                                                                           uint64
}

type Decoder struct {
	executable, player string
	inputRate          int
	mu                 sync.RWMutex
	status             Status
	cmd                *exec.Cmd
	stdin              io.WriteCloser
	playerOut          net.Conn
	recording          *os.File
	queue              chan []float32
	stop, done         chan struct{}
	releaseProcess     func()
	generation         uint64
	inverted           bool
	channelizer        *channelizer
	dropped            atomic.Uint64
	pool               sync.Pool
	symbolRate         int
	afcValid           int
}

func New(inputRate float64, executable, player string) *Decoder {
	d := &Decoder{executable: executable, player: resolvePlayer(player), inputRate: int(math.Round(inputRate))}
	if d.inputRate <= 0 {
		d.inputRate = 2_048_000
	}
	d.status.State = "stopped"
	d.status.Available = regularFile(executable)
	d.status.PlayerAvailable = regularFile(d.player)
	if !d.status.Available {
		d.status.Detail = "LeanDVB runtime not installed"
	}
	d.pool.New = func() any { return make([]float32, 0, 16_384) }
	return d
}

func regularFile(path string) bool { info, err := os.Stat(path); return err == nil && !info.IsDir() }

func resolvePlayer(path string) string {
	if candidate := filepath.Join(filepath.Dir(path), "libvlc.dll"); regularFile(candidate) {
		return candidate
	}
	for _, candidate := range []string{`C:\Program Files\VideoLAN\VLC\libvlc.dll`, `C:\Program Files (x86)\VideoLAN\VLC\libvlc.dll`} {
		if regularFile(candidate) {
			return candidate
		}
	}
	return path
}

func Arguments(inputRate int, s Settings) []string {
	standard := strings.ToUpper(strings.TrimSpace(s.Standard))
	if standard == "" || standard == "AUTO" {
		standard = "DVB-S2"
	}
	args := []string{"--f32", "--sr", strconv.Itoa(max(s.SymbolRate, 1)), "--sampler", "rrc", "--standard", standard, "--derotate", strconv.FormatInt(s.OffsetHz, 10), "--drift", "--fastlock", "--fd-info", "2", "--fd-const", "2", "--json"}
	if inputRate > 0 {
		args = append(args, "-f", strconv.Itoa(inputRate))
	}
	if inputRate > 3*s.SymbolRate {
		args = append(args, "--cnr")
	}
	if standard == "DVB-S" && s.FEC != "" && strings.ToUpper(s.FEC) != "AUTO" {
		args = append(args, "--cr", s.FEC)
	}
	return args
}

func (d *Decoder) Start(s Settings) error {
	plan := []Settings{s}
	if strings.EqualFold(strings.TrimSpace(s.Standard), "AUTO") || strings.TrimSpace(s.Standard) == "" {
		plan = autoPlan(s)
	}
	return d.startAttempt(plan, 0)
}

func (d *Decoder) startAttempt(plan []Settings, attempt int) error {
	d.Stop()
	if attempt < 0 || attempt >= len(plan) {
		return fmt.Errorf("invalid DATV automatic attempt")
	}
	s := plan[attempt]
	d.mu.Lock()
	if !d.status.Available {
		d.mu.Unlock()
		return fmt.Errorf("LeanDVB runtime not found: %s", d.executable)
	}
	abs, err := filepath.Abs(d.executable)
	if err != nil {
		d.mu.Unlock()
		return err
	}
	frontend := newChannelizer(d.inputRate, s.SymbolRate, s.OffsetHz)
	decoderSettings := s
	decoderSettings.OffsetHz = 0 // The channelizer has already moved it to DC.
	cmd := exec.Command(abs, Arguments(frontend.outputRate, decoderSettings)...)
	if (strings.EqualFold(s.Standard, "DVB-S2") || strings.EqualFold(s.Standard, "AUTO") || strings.TrimSpace(s.Standard) == "") && runtime.GOOS != "windows" {
		helper := filepath.Join(filepath.Dir(abs), "ldpc_tool.exe")
		if regularFile(helper) {
			cmd.Args = append(cmd.Args, "--ldpc-helper", helper)
		}
	} else if strings.EqualFold(s.Standard, "DVB-S2") || strings.EqualFold(s.Standard, "AUTO") || strings.TrimSpace(s.Standard) == "" {
		// Upstream's external helper path deliberately aborts on Windows. Its
		// in-process bit-flipping decoder is the supported Windows correction path.
		cmd.Args = append(cmd.Args, "--ldpc-bf", "15")
	}
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
	release, err := attachProcessLifetime(cmd)
	if err != nil {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		d.mu.Unlock()
		return err
	}
	d.generation++
	gen := d.generation
	d.cmd, d.stdin, d.queue, d.stop, d.done, d.releaseProcess = cmd, stdin, make(chan []float32, 32), make(chan struct{}), make(chan struct{}), release
	d.inverted = s.Inverted
	d.channelizer = frontend
	d.symbolRate, d.afcValid = s.SymbolRate, 0
	d.status.InputRate, d.status.DecoderRate = d.inputRate, frontend.outputRate
	d.status.SymbolRate, d.status.SpectrumInverted = s.SymbolRate, s.Inverted
	d.status.AutoMode, d.status.AutoAttempt, d.status.AutoAttempts = len(plan) > 1, attempt+1, len(plan)
	d.status.FrequencyErrorHz, d.status.AFCCorrectionHz, d.status.AFCState = 0, 0, "esperando lock"
	d.status.Points = nil
	d.status.Running, d.status.State, d.status.Detail = true, "starting", ""
	d.status.Locked, d.status.TSValid, d.status.PlayerRunning, d.status.Recording = false, false, false, false
	d.status.Standard = strings.ToUpper(s.Standard)
	if s.RecordingPath != "" {
		d.openRecordingLocked(s.RecordingPath)
	}
	if s.StartPlayer {
		d.startPlayerLocked()
	}
	d.mu.Unlock()
	done := d.done
	go d.writeLoop(gen)
	go d.transportLoop(stdout, gen)
	go d.eventLoop(stderr, gen)
	go d.waitLoop(cmd, gen, done)
	if len(plan) > attempt+1 {
		go d.autoFallback(gen, plan, attempt)
	}
	return nil
}

var commonSymbolRates = []int{33_000, 66_000, 125_000, 250_000, 333_000, 500_000, 1_000_000, 1_500_000}

func autoPlan(s Settings) []Settings {
	rates := []int{max(s.SymbolRate, 1)}
	if s.BandwidthHz > 0 {
		estimated := float64(s.BandwidthHz) / 1.20
		estimatedRate := int(math.Round(estimated/1000)) * 1000
		if estimatedRate >= 20_000 && math.Abs(float64(estimatedRate-rates[0]))/float64(rates[0]) >= .01 {
			rates = append(rates, estimatedRate)
		}
		best := commonSymbolRates[0]
		for _, rate := range commonSymbolRates[1:] {
			if math.Abs(float64(rate)-estimated) < math.Abs(float64(best)-estimated) {
				best = rate
			}
		}
		alreadyIncluded := false
		for _, rate := range rates {
			alreadyIncluded = alreadyIncluded || rate == best
		}
		if !alreadyIncluded {
			rates = append(rates, best)
		}
	}
	plan := make([]Settings, 0, len(rates)*4)
	for _, rate := range rates {
		for _, variant := range []struct {
			standard string
			inverted bool
		}{{"DVB-S2", s.Inverted}, {"DVB-S", s.Inverted}, {"DVB-S2", !s.Inverted}, {"DVB-S", !s.Inverted}} {
			candidate := s
			candidate.Standard, candidate.SymbolRate, candidate.Inverted = variant.standard, rate, variant.inverted
			plan = append(plan, candidate)
		}
	}
	return plan
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

func (d *Decoder) SetRecording(path string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.recording != nil {
		_ = d.recording.Close()
		d.recording = nil
	}
	d.status.Recording, d.status.RecordingPath = false, ""
	if strings.TrimSpace(path) == "" {
		return nil
	}
	return d.openRecordingLocked(path)
}

func (d *Decoder) openRecordingLocked(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	d.recording, d.status.Recording, d.status.RecordingPath = f, true, path
	return nil
}

func (d *Decoder) Snapshot() Status {
	d.mu.RLock()
	s := d.status
	s.Points = append([]Point(nil), d.status.Points...)
	if d.queue != nil {
		s.InputQueued, s.InputCapacity = len(d.queue), cap(d.queue)
	}
	d.mu.RUnlock()
	s.Dropped = d.dropped.Load()
	return s
}

func (d *Decoder) Stop() {
	d.mu.Lock()
	if !d.status.Running {
		if d.recording != nil {
			_ = d.recording.Close()
			d.recording = nil
		}
		d.status.Recording = false
		d.mu.Unlock()
		return
	}
	d.status.Running, d.status.Locked, d.status.TSValid, d.status.PlayerRunning, d.status.Recording = false, false, false, false, false
	d.status.State = "stopped"
	d.status.FrequencyErrorHz, d.status.AFCCorrectionHz, d.status.AFCState = 0, 0, "detenido"
	d.status.Points = nil
	d.status.Bitrate, d.status.Video, d.status.Audio = 0, "", ""
	stop, stdin, cmd, playerOut, rec, done := d.stop, d.stdin, d.cmd, d.playerOut, d.recording, d.done
	release, queue := d.releaseProcess, d.queue
	d.stop, d.stdin, d.cmd, d.playerOut, d.recording, d.done, d.queue, d.channelizer = nil, nil, nil, nil, nil, nil, nil, nil
	d.releaseProcess = nil
	close(stop)
	d.mu.Unlock()
	_ = stdin.Close()
	if playerOut != nil {
		_ = playerOut.Close()
	}
	if rec != nil {
		_ = rec.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	if done != nil {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}
	if release != nil {
		release()
	}
	for queue != nil {
		select {
		case block := <-queue:
			d.pool.Put(block[:0])
		default:
			return
		}
	}
}

func (d *Decoder) Close() { d.Stop() }

func (d *Decoder) writeLoop(gen uint64) {
	for {
		d.mu.RLock()
		if d.generation != gen {
			d.mu.RUnlock()
			return
		}
		stop, q, w := d.stop, d.queue, d.stdin
		d.mu.RUnlock()
		select {
		case <-stop:
			return
		case block := <-q:
			d.mu.RLock()
			frontend, inverted := d.channelizer, d.inverted
			d.mu.RUnlock()
			processed := block
			if frontend != nil {
				processed = frontend.process(block)
			}
			// Invert after frequency translation. Conjugating the original wideband
			// stream would also mirror the selected offset around the SDR centre.
			if inverted {
				for i := 1; i < len(processed); i += 2 {
					processed[i] = -processed[i]
				}
			}
			data := make([]byte, len(processed)*4)
			for i, v := range processed {
				binary.LittleEndian.PutUint32(data[i*4:], math.Float32bits(v))
			}
			d.pool.Put(block[:0])
			if _, err := w.Write(data); err != nil {
				return
			}
		}
	}
}

func (d *Decoder) transportLoop(r io.Reader, gen uint64) {
	buf := make([]byte, 64*1024)
	var bytes uint64
	started := time.Now()
	framer := transportFramer{}
	analyzer := transportAnalyzer{}
	for {
		n, err := r.Read(buf)
		if n > 0 {
			data := framer.push(buf[:n])
			if len(data) > 0 {
				bytes += uint64(len(data))
				video, audio := analyzer.consume(data)
				d.mu.Lock()
				if d.generation == gen {
					d.status.TSValid = true
					d.status.Video, d.status.Audio = video, audio
					if d.status.State == "starting" {
						d.status.State = "receiving"
					}
				}
				d.mu.Unlock()
				d.mu.RLock()
				rec, playerOut := d.recording, d.playerOut
				d.mu.RUnlock()
				if rec != nil {
					_, _ = rec.Write(data)
				}
				if playerOut != nil {
					if e := writeTransportUDP(playerOut, data); e != nil {
						d.playerFailed(gen, e)
					}
				}
			}
			elapsed := time.Since(started).Seconds()
			if elapsed > 0 {
				d.mu.Lock()
				if d.generation == gen {
					d.status.Bitrate = uint64(float64(bytes*8) / elapsed)
				}
				d.mu.Unlock()
			}
		}
		if err != nil {
			return
		}
	}
}

func writeTransportUDP(writer io.Writer, data []byte) error {
	const payload = 7 * 188
	for len(data) > 0 {
		count := min(len(data), payload)
		if _, err := writer.Write(data[:count]); err != nil {
			return err
		}
		data = data[count:]
	}
	return nil
}

func transportSync(data []byte) (bool, int) {
	for off := 0; off < min(188, len(data)); off++ {
		if len(data) > off+376 && data[off] == 0x47 && data[off+188] == 0x47 && data[off+376] == 0x47 {
			return true, off
		}
	}
	return false, 0
}

var numberRE = regexp.MustCompile(`[-+]?\d+(?:\.\d+)?`)

func (d *Decoder) eventLoop(r io.Reader, gen uint64) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		d.applyTelemetry(scanner.Text(), gen)
	}
}
func (d *Decoder) applyTelemetry(line string, gen uint64) {
	if points, ok := parseSymbolPoints(line); ok {
		d.mu.Lock()
		if d.generation == gen {
			d.status.Points = points
		}
		d.mu.Unlock()
		return
	}
	values := map[string]any{}
	_ = json.Unmarshal([]byte(line), &values)
	lower := strings.ToLower(line)
	fields := strings.Fields(line)
	if len(fields) >= 2 {
		key, value := strings.ToUpper(fields[0]), strings.Trim(strings.Join(fields[1:], " "), "\"")
		switch key {
		case "LOCK", "FRAMELOCK":
			values["lock"] = value
		case "MER":
			values["mer"] = value
		case "CNR":
			values["cnr"] = value
		case "FREQ":
			values["frequencyErrorHz"] = value
		case "STANDARD":
			values["standard"] = value
		case "CONSTELLATION":
			values["constellation"] = value
		case "CR":
			values["fec"] = value
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.generation != gen {
		return
	}
	frequencyUpdated := false
	for k, v := range values {
		key := strings.ToLower(k)
		switch key {
		case "lock", "locked":
			d.status.Locked = asBool(v)
		case "mer":
			d.status.MER = asFloat(v)
		case "cnr", "snr":
			d.status.CNR = asFloat(v)
		case "frequencyerrorhz", "freq_error", "freq":
			d.status.FrequencyErrorHz = asFloat(v)
			frequencyUpdated = true
		case "standard":
			d.status.Standard = fmt.Sprint(v)
		case "constellation", "modulation":
			d.status.Constellation = fmt.Sprint(v)
		case "fec", "coderate":
			d.status.FEC = fmt.Sprint(v)
		case "service", "program":
			d.status.Service = fmt.Sprint(v)
		case "video":
			d.status.Video = fmt.Sprint(v)
		case "audio":
			d.status.Audio = fmt.Sprint(v)
		}
	}
	if strings.Contains(lower, "lock") {
		if strings.Contains(lower, "lock=1") || strings.Contains(lower, "lock: 1") || strings.Contains(lower, "locked") {
			d.status.Locked = true
		}
	}
	if d.status.Locked {
		d.status.State = "locked"
	}
	if strings.Contains(lower, "mer") && d.status.MER == 0 {
		if s := numberRE.FindString(line); s != "" {
			if v, e := strconv.ParseFloat(s, 32); e == nil {
				d.status.MER = float32(v)
			}
		}
	}
	if frequencyUpdated {
		d.updateAFCLocked()
	}
}

func parseSymbolPoints(line string) ([]Point, bool) {
	if !strings.HasPrefix(strings.TrimSpace(line), "SYMBOLS ") {
		return nil, false
	}
	start := strings.IndexByte(line, '[')
	if start < 0 {
		return nil, false
	}
	var raw [][]float32
	if json.Unmarshal([]byte(line[start:]), &raw) != nil || len(raw) == 0 {
		return nil, false
	}
	peak := float32(0)
	for _, pair := range raw {
		if len(pair) >= 2 {
			peak = max(peak, max(abs32(pair[0]), abs32(pair[1])))
		}
	}
	if peak <= 0 {
		peak = 1
	}
	points := make([]Point, 0, min(len(raw), 128))
	for _, pair := range raw {
		if len(pair) >= 2 && len(points) < 128 {
			points = append(points, Point{I: pair[0] / peak, Q: pair[1] / peak})
		}
	}
	return points, true
}

// updateAFCLocked nudges the external channelizer only after repeated locked
// measurements. LeanDVB keeps handling rapid carrier tracking; this loop merely
// removes slow LNB/receiver drift so its internal PLL stays near the centre.
func (d *Decoder) updateAFCLocked() {
	if d.channelizer == nil || !d.status.Locked {
		d.afcValid = 0
		d.status.AFCState = "esperando lock"
		return
	}
	errorHz := d.status.FrequencyErrorHz
	limit := float32(max(d.symbolRate/5, 500))
	if abs32(errorHz) > limit {
		d.afcValid = 0
		d.status.AFCState = "error fuera de rango"
		return
	}
	d.afcValid++
	if d.afcValid < 3 {
		d.status.AFCState = "confirmando"
		return
	}
	deadband := float32(max(d.symbolRate/2000, 20))
	if abs32(errorHz) <= deadband {
		d.status.AFCState = "centrado"
		return
	}
	maxStep := float32(max(d.symbolRate/500, 25))
	step := min(max(errorHz*.12, -maxStep), maxStep)
	maxCorrection := float32(max(d.symbolRate/4, 1500))
	d.status.AFCCorrectionHz = min(max(d.status.AFCCorrectionHz+step, -maxCorrection), maxCorrection)
	d.channelizer.setCorrection(float64(d.status.AFCCorrectionHz))
	d.status.AFCState = "corrigiendo"
}

func abs32(value float32) float32 {
	if value < 0 {
		return -value
	}
	return value
}

func (d *Decoder) autoFallback(gen uint64, plan []Settings, attempt int) {
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	<-timer.C
	d.mu.RLock()
	shouldRetry := d.generation == gen && d.status.Running && !d.status.Locked && !d.status.TSValid
	recordingPath := d.status.RecordingPath
	d.mu.RUnlock()
	if shouldRetry {
		next := attempt + 1
		plan[next].RecordingPath = recordingPath
		_ = d.startAttempt(plan, next)
	}
}

func asBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case string:
		return x == "1" || strings.EqualFold(x, "true")
	}
	return false
}
func asFloat(v any) float32 {
	switch x := v.(type) {
	case float64:
		return float32(x)
	case string:
		n, _ := strconv.ParseFloat(x, 32)
		return float32(n)
	}
	return 0
}

func (d *Decoder) startPlayerLocked() {
	conn, err := net.Dial("udp", "127.0.0.1:19540")
	if err != nil {
		d.status.Detail = "vídeo local: " + err.Error()
		return
	}
	d.playerOut = conn
	d.status.PlayerRunning = true
}
func (d *Decoder) playerFailed(gen uint64, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.generation == gen {
		d.status.PlayerRunning = false
		if d.playerOut != nil {
			_ = d.playerOut.Close()
			d.playerOut = nil
		}
		d.status.Detail = "VLC: " + err.Error()
	}
}
func (d *Decoder) waitLoop(cmd *exec.Cmd, gen uint64, done chan struct{}) {
	err := cmd.Wait()
	close(done)
	d.mu.Lock()
	if d.generation == gen {
		d.status.Running = false
		if d.status.State != "stopped" {
			d.status.State = "error"
			if err != nil {
				d.status.Detail = err.Error()
			}
		}
		d.done = nil
	}
	d.mu.Unlock()
}
