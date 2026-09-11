package aprs

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go-zero/internal/resources"
)

type Status struct {
	Running, KISS        bool
	State, Detail        string
	AudioLevel           int
	PacketCount, Dropped uint64
	Queued               int
	FrequencyErrorHz     float32
}
type Decoder struct {
	inputRate                                    float64
	executable, configTemplate, workingDirectory string
	mu                                           sync.RWMutex
	packets                                      []Packet
	state, detail                                string
	audioLevel                                   int
	frequencyError                               float32
	front                                        *frontend
	queue                                        chan []int16
	stop                                         chan struct{}
	cmd                                          *exec.Cmd
	stdin                                        io.WriteCloser
	kiss                                         net.Conn
	running                                      atomic.Bool
	session                                      atomic.Uint64
	dropped, packetCount                         atomic.Uint64
	configPath                                   string
	recentPackets                                map[string]time.Time
}

func New(inputRate float64, executable, configPath, workingDirectory string) *Decoder {
	if absolute, err := filepath.Abs(executable); err == nil {
		executable = absolute
	}
	if absolute, err := filepath.Abs(configPath); err == nil {
		configPath = absolute
	}
	if absolute, err := filepath.Abs(workingDirectory); err == nil {
		workingDirectory = absolute
	}
	return &Decoder{inputRate: inputRate, executable: executable, configTemplate: configPath, workingDirectory: workingDirectory, state: "STOPPED", audioLevel: -1, recentPackets: make(map[string]time.Time)}
}
func (d *Decoder) Configure(enabled bool, tuned, center int64, bandwidth int) {
	if !enabled {
		d.Stop()
		return
	}
	d.mu.RLock()
	same := d.running.Load() && d.front != nil && d.front.offsetHz == float64(tuned-center) && d.front.bandwidth == max(8500, bandwidth)
	d.mu.RUnlock()
	if same {
		return
	}
	d.Stop()
	d.mu.Lock()
	d.front = newFrontend(d.inputRate)
	d.front.reset(float64(tuned-center), bandwidth)
	d.mu.Unlock()
	d.start()
}
func (d *Decoder) start() {
	if d.executable == "" {
		d.setError(fmt.Errorf("Dire Wolf not configured"))
		return
	}
	port, err := freePort()
	if err != nil {
		d.setError(err)
		return
	}
	config, err := d.runtimeConfig(port)
	if err != nil {
		d.setError(err)
		return
	}
	d.configPath = config
	d.queue = make(chan []int16, 16)
	d.stop = make(chan struct{})
	d.cmd = exec.Command(d.executable, "-c", config, "-r", "48000", "-n", "1", "-b", "16", "-B", "1200", "-t", "0", "-q", "dx", "-")
	d.cmd.Dir = d.workingDirectory
	d.cmd.SysProcAttr = hiddenProcessAttributes()
	stdin, err := d.cmd.StdinPipe()
	if err != nil {
		d.setError(err)
		return
	}
	stdout, _ := d.cmd.StdoutPipe()
	stderr, _ := d.cmd.StderrPipe()
	if err = d.cmd.Start(); err != nil {
		d.setError(err)
		return
	}
	d.stdin = stdin
	session := d.session.Add(1)
	d.running.Store(true)
	d.mu.Lock()
	d.state, d.detail, d.audioLevel = "SEARCHING", "Connecting local KISS", -1
	d.mu.Unlock()
	go d.writer()
	go d.readLines(stdout)
	go d.readLines(stderr)
	go d.connectKISS(port, session)
}
func freePort() (int, error) {
	// Dire Wolf stores network ports in a signed 16-bit configuration field.
	// Windows' ephemeral allocator commonly returns >32767, which Dire Wolf
	// rejects even though it is a valid TCP port. Probe a private compatible
	// range instead.
	for port := 18101; port <= 18199; port++ {
		listener, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
		if err != nil {
			continue
		}
		_ = listener.Close()
		return port, nil
	}
	return 0, fmt.Errorf("no hay un puerto KISS libre entre 18101 y 18199")
}
func (d *Decoder) runtimeConfig(port int) (string, error) {
	data, err := os.ReadFile(d.configTemplate)
	if err != nil {
		return "", err
	}
	lines := []string{}
	for _, line := range strings.Split(string(data), "\n") {
		upper := strings.ToUpper(strings.TrimSpace(line))
		if strings.HasPrefix(upper, "KISSPORT") || strings.HasPrefix(upper, "AGWPORT") {
			continue
		}
		lines = append(lines, line)
	}
	lines = append(lines, "AGWPORT 0", "KISSPORT "+strconv.Itoa(port))
	directory := resources.WritablePath("cache", "aprs")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(directory, "direwolf-*.conf")
	if err != nil {
		return "", err
	}
	path := file.Name()
	_, err = file.WriteString(strings.Join(lines, "\n"))
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return path, err
}
func (d *Decoder) ProcessIQ(iq []float32) {
	if !d.running.Load() {
		return
	}
	d.mu.Lock()
	samples := d.front.process(iq)
	d.frequencyError = d.front.errorHz
	d.mu.Unlock()
	if len(samples) == 0 {
		return
	}
	select {
	case d.queue <- samples:
	default:
		d.dropped.Add(1)
	}
}
func (d *Decoder) writer() {
	buffer := make([]byte, 4096)
	for {
		select {
		case <-d.stop:
			return
		case samples := <-d.queue:
			offset := 0
			for _, sample := range samples {
				if offset+2 > len(buffer) {
					if _, err := d.stdin.Write(buffer[:offset]); err != nil {
						return
					}
					offset = 0
				}
				binary.LittleEndian.PutUint16(buffer[offset:], uint16(sample))
				offset += 2
			}
			if offset > 0 {
				if _, err := d.stdin.Write(buffer[:offset]); err != nil {
					return
				}
			}
		}
	}
}
func (d *Decoder) connectKISS(port int, session uint64) {
	address := "127.0.0.1:" + strconv.Itoa(port)
	for d.sessionActive(session) {
		connection, err := net.DialTimeout("tcp", address, 500*time.Millisecond)
		if err != nil {
			if !d.sessionActive(session) {
				return
			}
			time.Sleep(150 * time.Millisecond)
			continue
		}
		if !d.sessionActive(session) {
			_ = connection.Close()
			return
		}
		d.mu.Lock()
		d.kiss = connection
		d.detail = "KISS conectado · esperando AX.25"
		d.mu.Unlock()
		d.readKISS(connection, session)
		_ = connection.Close()
		d.mu.Lock()
		if d.kiss == connection {
			d.kiss = nil
		}
		d.mu.Unlock()
		if d.sessionActive(session) {
			time.Sleep(150 * time.Millisecond)
		}
	}
}
func (d *Decoder) sessionActive(session uint64) bool {
	return d.running.Load() && d.session.Load() == session
}
func (d *Decoder) readKISS(reader io.Reader, session uint64) {
	frame := make([]byte, 0, 512)
	buf := make([]byte, 2048)
	inside, escaped := false, false
	for d.sessionActive(session) {
		n, err := reader.Read(buf)
		if err != nil {
			return
		}
		for _, value := range buf[:n] {
			if !d.sessionActive(session) {
				return
			}
			if value == 0xc0 {
				if inside && len(frame) > 1 && frame[0]&15 == 0 {
					if packet, ok := DecodeAX25(frame[1:], d.audioLevel); ok {
						d.add(packet)
					}
				}
				frame = frame[:0]
				inside, escaped = true, false
				continue
			}
			if !inside {
				continue
			}
			if escaped {
				if value == 0xdc {
					value = 0xc0
				} else if value == 0xdd {
					value = 0xdb
				}
				escaped = false
			} else if value == 0xdb {
				escaped = true
				continue
			}
			if len(frame) < 4096 {
				frame = append(frame, value)
			} else {
				frame = frame[:0]
				inside = false
			}
		}
	}
}
func (d *Decoder) add(packet Packet) {
	d.mu.Lock()
	// More than one KISS client could briefly observe the same frame while an
	// older decoder session was shutting down. Suppress only byte-identical
	// copies arriving together; genuine APRS retransmissions remain visible.
	now := time.Now()
	if previous, exists := d.recentPackets[packet.Raw]; exists && now.Sub(previous) < 750*time.Millisecond {
		d.mu.Unlock()
		return
	}
	d.recentPackets[packet.Raw] = now
	for raw, seen := range d.recentPackets {
		if now.Sub(seen) > 5*time.Second {
			delete(d.recentPackets, raw)
		}
	}
	d.packets = append([]Packet{packet}, d.packets...)
	if len(d.packets) > 500 {
		d.packets = d.packets[:500]
	}
	d.state = "RECIBIENDO"
	d.detail = packet.Source + " > " + packet.Destination
	d.mu.Unlock()
	d.packetCount.Add(1)
}
func (d *Decoder) readLines(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			d.mu.Lock()
			if d.kiss == nil {
				d.detail = line
			}
			d.mu.Unlock()
		}
		if marker := strings.Index(line, " audio level = "); marker > 0 {
			tail := strings.TrimSpace(line[marker+15:])
			if end := strings.IndexByte(tail, '('); end > 0 {
				tail = strings.TrimSpace(tail[:end])
			}
			if value, err := strconv.Atoi(tail); err == nil {
				d.mu.Lock()
				d.audioLevel = value
				d.mu.Unlock()
			}
		}
	}
}
func (d *Decoder) Packets() []Packet {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return append([]Packet(nil), d.packets...)
}
func (d *Decoder) Clear() {
	d.mu.Lock()
	d.packets = nil
	d.recentPackets = make(map[string]time.Time)
	d.state = "SEARCHING"
	d.detail = "Waiting for AX.25 frames"
	d.mu.Unlock()
	d.packetCount.Store(0)
}
func (d *Decoder) Snapshot() Status {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return Status{Running: d.running.Load(), KISS: d.kiss != nil, State: d.state, Detail: d.detail, AudioLevel: d.audioLevel, PacketCount: d.packetCount.Load(), Dropped: d.dropped.Load(), Queued: len(d.queue), FrequencyErrorHz: d.frequencyError}
}
func (d *Decoder) setError(err error) {
	d.mu.Lock()
	d.state, d.detail = "ERROR", err.Error()
	d.mu.Unlock()
}
func (d *Decoder) Stop() {
	d.session.Add(1)
	if !d.running.Swap(false) {
		return
	}
	close(d.stop)
	if d.kiss != nil {
		_ = d.kiss.Close()
	}
	if d.stdin != nil {
		_ = d.stdin.Close()
	}
	if d.cmd != nil && d.cmd.Process != nil {
		_ = d.cmd.Process.Kill()
		_, _ = d.cmd.Process.Wait()
	}
	if d.configPath != "" && filepath.IsAbs(d.configPath) {
		_ = os.Remove(d.configPath)
	}
	d.mu.Lock()
	d.state, d.detail, d.audioLevel = "STOPPED", "", -1
	d.kiss = nil
	d.mu.Unlock()
}
