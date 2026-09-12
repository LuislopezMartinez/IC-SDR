package digitalvoice

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const inputSampleRate = 48_000

type Event struct {
	At, Protocol, Slot, Source, Target, Detail string
}

type Status struct {
	State, Detail, Protocol, Slot, Source, Target, CallType string
	ColorCode, NAC, RAN, Site, System                       string
	InputDBFS, SNR, BER                                     float32
	Encrypted, VoiceActive, Running, Available              bool
	StartedAt, LastVoice                                    time.Time
	Events                                                  []Event
}

type Decoder struct {
	executable     string
	onAudio        func([]float32)
	front          *frontend
	voiceRate      linearResampler
	outputChannels int

	mu       sync.RWMutex
	status   Status
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stop     chan struct{}
	done     chan struct{}
	pcm      chan []byte
	stopOnce sync.Once
}

func New(inputRate float64, executable string, onAudio func([]float32)) *Decoder {
	d := &Decoder{executable: executable, onAudio: onAudio, front: newFrontend(inputRate, 0, 12_500), outputChannels: 1}
	d.voiceRate.reset(8_000, 48_000)
	d.status = Status{State: "STOPPED", Detail: "Press START to enable detection", InputDBFS: -60}
	if info, err := os.Stat(executable); err == nil && !info.IsDir() {
		d.status.Available = true
	} else {
		d.status.Detail = "DSD-neo runtime is not installed"
	}
	return d
}

func (d *Decoder) Configure(offsetHz float64, bandwidth int) {
	if d.front != nil {
		d.front.reset(offsetHz, bandwidth)
	}
}

func (d *Decoder) Start(mode string) error {
	d.mu.Lock()
	if d.status.Running {
		d.mu.Unlock()
		return nil
	}
	if !d.status.Available {
		d.mu.Unlock()
		return fmt.Errorf("DSD-neo runtime is not available: %s", d.executable)
	}
	d.stop, d.done, d.pcm = make(chan struct{}), make(chan struct{}), make(chan []byte, 12)
	d.voiceRate.reset(8_000, 48_000)
	d.outputChannels = modeOutputChannels(mode)
	d.stopOnce = sync.Once{}
	cmd := exec.Command(d.executable, modeArgument(mode), "-i", "-", "-s", strconv.Itoa(inputSampleRate), "-o", "-")
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
	cmd.Dir = strings.TrimSuffix(d.executable, string(os.PathSeparator)+filepathBase(d.executable))
	if err := cmd.Start(); err != nil {
		d.mu.Unlock()
		return err
	}
	d.cmd, d.stdin = cmd, stdin
	d.status.State, d.status.Detail, d.status.Running = "SEARCHING", "Automatic detection is active", true
	d.status.StartedAt = time.Now()
	d.mu.Unlock()
	go d.runWriter()
	go d.runOutput(stdout)
	go d.runLog(stderr)
	go d.wait()
	return nil
}

func modeArgument(mode string) string {
	switch strings.ToUpper(strings.TrimSpace(mode)) {
	case "DMR":
		return "-fs"
	case "P25 I":
		return "-f1"
	case "P25 II":
		return "-f2"
	case "NXDN 48":
		return "-fi"
	case "NXDN 96":
		return "-fn"
	case "D-STAR":
		return "-fd"
	case "YSF":
		return "-fy"
	case "DPMR":
		return "-fm"
	case "PROVOICE":
		return "-fp"
	case "M17":
		return "-fz"
	case "X2-TDMA":
		return "-fx"
	default:
		return "-fa"
	}
}

// DSD-neo's CLI presets select the decoded-audio channel count. TDMA and
// automatic presets publish an interleaved stereo stream so both slots remain
// available; the other scoped presets publish mono.
func modeOutputChannels(mode string) int {
	switch strings.ToUpper(strings.TrimSpace(mode)) {
	case "AUTO · ALL", "AUTO · TODOS", "DMR", "P25 II", "X2-TDMA":
		return 2
	default:
		return 1
	}
}

// filepathBase avoids importing filepath only to derive the executable directory.
func filepathBase(path string) string {
	if i := strings.LastIndexAny(path, `/\\`); i >= 0 {
		return path[i+1:]
	}
	return path
}

func (d *Decoder) Stop() {
	d.stopOnce.Do(func() {
		d.mu.Lock()
		stop, stdin, cmd, running := d.stop, d.stdin, d.cmd, d.status.Running
		d.status.Running, d.status.VoiceActive = false, false
		d.status.State = "STOPPED"
		d.mu.Unlock()
		if !running {
			return
		}
		close(stop)
		_ = stdin.Close()
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})
}

func (d *Decoder) Close() { d.Stop() }

func (d *Decoder) ProcessIQ(iq []float32) {
	d.mu.RLock()
	running, queue := d.status.Running, d.pcm
	d.mu.RUnlock()
	if !running || len(iq) == 0 || d.front == nil {
		return
	}
	samples, inputDBFS := d.front.process(iq)
	if len(samples) == 0 {
		return
	}
	pcm := make([]byte, len(samples)*2)
	for i, sample := range samples {
		binary.LittleEndian.PutUint16(pcm[i*2:], uint16(sample))
	}
	d.mu.Lock()
	d.status.InputDBFS = inputDBFS
	d.mu.Unlock()
	select {
	case queue <- pcm:
	default:
	}
}

func (d *Decoder) Snapshot() Status {
	d.mu.RLock()
	s := d.status
	s.Events = append([]Event(nil), d.status.Events...)
	d.mu.RUnlock()
	if s.VoiceActive && !s.LastVoice.IsZero() && time.Since(s.LastVoice) > time.Second {
		s.VoiceActive = false
		if s.Running {
			s.State = "SEARCHING"
		}
	}
	return s
}

func (d *Decoder) runWriter() {
	for {
		select {
		case <-d.stop:
			return
		case pcm := <-d.pcm:
			if _, err := d.stdin.Write(pcm); err != nil {
				return
			}
		}
	}
}

func (d *Decoder) runOutput(reader io.Reader) {
	buffer := make([]byte, 4096)
	var carry []byte
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			data := append(carry, buffer[:n]...)
			frameBytes := max(d.outputChannels, 1) * 2
			complete := len(data) - len(data)%frameBytes
			carry = append(carry[:0], data[complete:]...)
			data = data[:complete]
			// DSD-neo's discriminator input runs at 48 kHz, but its digital
			// vocoders publish 8 kHz PCM. Convert that voice stream to the
			// application's fixed 48 kHz audio clock.
			decoded := make([]float32, 0, len(data)/frameBytes)
			power := float64(0)
			for offset := 0; offset < len(data); offset += frameBytes {
				sample := float32(int16(binary.LittleEndian.Uint16(data[offset:]))) / 32768
				if d.outputChannels == 2 {
					right := float32(int16(binary.LittleEndian.Uint16(data[offset+2:]))) / 32768
					sample = min(max(sample+right, -1), 1)
				}
				decoded = append(decoded, sample)
				power += float64(sample * sample)
			}
			voice := len(decoded) > 0 && math.Sqrt(power/float64(len(decoded))) > .001
			d.mu.RLock()
			running := d.status.Running
			d.mu.RUnlock()
			if voice && running && d.onAudio != nil {
				resampled := d.voiceRate.process(decoded)
				if len(resampled) > 0 {
					d.onAudio(resampled)
				}
			}
			if voice {
				d.mu.Lock()
				d.status.VoiceActive, d.status.State, d.status.LastVoice = true, "DECODING", time.Now()
				d.mu.Unlock()
			}
		}
		if err != nil {
			return
		}
	}
}

var (
	protocolPattern = regexp.MustCompile(`(?i)\b(DMR|P25(?:P1|P2|\s*(?:PHASE\s*)?[12])?|NXDN(?:48|96)?|D-?STAR|YSF|DPMR|PROVOICE|M17|X2-TDMA)\b`)
	slotPattern     = regexp.MustCompile(`(?i)\b(?:SLOT|TS)\s*[:=]?\s*([12])\b`)
	targetPattern   = regexp.MustCompile(`(?i)\b(?:TG|TARGET|DST)\s*[:=]?\s*(\d+)`)
	sourcePattern   = regexp.MustCompile(`(?i)\b(?:SRC|SOURCE|RADIO)\s*[:=]?\s*(\d+)`)
	ccPattern       = regexp.MustCompile(`(?i)\b(?:CC|COLOR\s*CODE)\s*[:=]?\s*(\d+)`)
	berPattern      = regexp.MustCompile(`(?i)\bBER\s*[:=]?\s*([\d.]+)`)
	snrPattern      = regexp.MustCompile(`(?i)\bSNR\s*[:=]?\s*([+-]?[\d.]+)`)
)

func (d *Decoder) runLog(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		d.parseLine(scanner.Text())
	}
}

func (d *Decoder) parseLine(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	upper := strings.ToUpper(line)
	// Auto hunting announces rejected candidates too. A failed CRC must not
	// replace the last positively identified protocol in the UI.
	if strings.Contains(upper, "CRC ERR") || strings.Contains(upper, "CRC FAIL") ||
		strings.Contains(upper, "M17 EOT") || (strings.Contains(upper, "M17 LSF") && !strings.Contains(upper, "CRC OK")) {
		d.status.Detail = line
		return
	}
	if match := protocolPattern.FindStringSubmatch(line); len(match) > 1 {
		d.status.Protocol = normalizeProtocol(match[1])
	}
	if match := slotPattern.FindStringSubmatch(line); len(match) > 1 {
		d.status.Slot = "SLOT " + match[1]
	}
	if match := targetPattern.FindStringSubmatch(line); len(match) > 1 {
		d.status.Target = match[1]
	}
	if match := sourcePattern.FindStringSubmatch(line); len(match) > 1 {
		d.status.Source = match[1]
	}
	if match := ccPattern.FindStringSubmatch(line); len(match) > 1 {
		d.status.ColorCode = match[1]
	}
	if match := berPattern.FindStringSubmatch(line); len(match) > 1 {
		if v, e := strconv.ParseFloat(match[1], 32); e == nil {
			d.status.BER = float32(v)
		}
	}
	if match := snrPattern.FindStringSubmatch(line); len(match) > 1 {
		if v, e := strconv.ParseFloat(match[1], 32); e == nil {
			d.status.SNR = float32(v)
		}
	}
	d.status.Encrypted = strings.Contains(strings.ToUpper(line), "ENCRYPT") || strings.Contains(strings.ToUpper(line), "CIPHER")
	d.status.Detail = line
	if d.status.Protocol != "" {
		d.status.Events = append(d.status.Events, Event{At: time.Now().Format("15:04:05"), Protocol: d.status.Protocol, Slot: d.status.Slot, Source: d.status.Source, Target: d.status.Target, Detail: line})
		if len(d.status.Events) > 80 {
			d.status.Events = d.status.Events[len(d.status.Events)-80:]
		}
	}
}

func normalizeProtocol(value string) string {
	v := strings.ToUpper(strings.ReplaceAll(value, "-", ""))
	switch {
	case strings.HasPrefix(v, "P25") && strings.Contains(v, "2"):
		return "P25 II"
	case strings.HasPrefix(v, "P25"):
		return "P25 I"
	case strings.HasPrefix(v, "NXDN48"):
		return "NXDN48"
	case strings.HasPrefix(v, "NXDN"):
		return "NXDN96"
	case v == "DSTAR":
		return "D-STAR"
	case v == "X2TDMA":
		return "X2"
	default:
		return strings.ToUpper(value)
	}
}

func (d *Decoder) wait() {
	err := d.cmd.Wait()
	d.mu.Lock()
	d.status.Running, d.status.VoiceActive = false, false
	d.status.State = "STOPPED"
	if err != nil {
		d.status.State, d.status.Detail = "ERROR", err.Error()
	}
	close(d.done)
	d.mu.Unlock()
}
