// Package radiosonde feeds the shared SDR IQ stream to the RS command-line decoders.
package radiosonde

import (
	"go-zero/internal/i18n"

	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var Families = []string{"RS41", i18n.Source("text.5945e8331ba7"), "M10/M20"}
var Modes = []string{i18n.Source("text.6ea56fae9eac"), "RS41", i18n.Source("text.5945e8331ba7"), "M10/M20"}

func ValidFamily(f string) bool {
	return f == i18n.Source("text.6ea56fae9eac") || f == "RS41" || f == i18n.Source("text.5945e8331ba7") || f == "M10/M20"
}

type Event struct {
	Received          time.Time       `json:"received"`
	Type              string          `json:"type"`
	ID                string          `json:"id"`
	Frame             int64           `json:"frame"`
	Datetime          string          `json:"datetime"`
	Lat               *float64        `json:"lat,omitempty"`
	Lon               *float64        `json:"lon,omitempty"`
	Alt               *float64        `json:"alt,omitempty"`
	Speed             *float64        `json:"vel_h,omitempty"`
	Climb             *float64        `json:"vel_v,omitempty"`
	Heading           *float64        `json:"heading,omitempty"`
	Temperature       *float64        `json:"temp,omitempty"`
	Humidity          *float64        `json:"humidity,omitempty"`
	Pressure          *float64        `json:"pressure,omitempty"`
	PositionReference string          `json:"ref_position,omitempty"`
	DatetimeReference string          `json:"ref_datetime,omitempty"`
	FrequencyHz       int64           `json:"frequency_hz"`
	Raw               json.RawMessage `json:"raw"`
}

func ParseEvent(line []byte) (Event, bool) {
	var e Event
	if json.Unmarshal(line, &e) != nil || e.ID == "" || e.Type == "" {
		return Event{}, false
	}
	if (e.Lat != nil && (*e.Lat < -90 || *e.Lat > 90)) || (e.Lon != nil && (*e.Lon < -180 || *e.Lon > 180)) {
		return Event{}, false
	}
	e.Received = time.Now().UTC()
	e.Raw = append(json.RawMessage(nil), line...)
	return e, true
}

type Status struct {
	Running                      bool
	State, Error, Detail, Family string
	FrequencyHz                  int64
	Dropped                      uint64
	Events                       int
}
type session struct {
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	queue      chan []float32
	stop, done chan struct{}
	cancelOnce sync.Once
	writerDone chan struct{}
}

func (s *session) cancel() { s.cancelOnce.Do(func() { close(s.stop) }) }

type Decoder struct {
	lifecycle  sync.Mutex
	mu         sync.RWMutex
	directory  string
	rate       float64
	center     int64
	status     Status
	events     []Event
	current    *session
	children   []*Decoder
	eventSink  func(Event)
	generation uint64
	detections map[string]autoDetection
}

func New(rate float64, directory string) *Decoder {
	return &Decoder{rate: rate, directory: directory, status: Status{State: i18n.Source("text.7dc7253c376a")}}
}

// Arguments lets RS perform channel translation, filtering and decimation itself.
// The input is interleaved little-endian float32 I,Q, independent of listening audio.
func Arguments(family string, rate float64, frequency, center int64) (string, []string, error) {
	names := map[string]string{"RS41": "rs41mod", i18n.Source("text.5945e8331ba7"): "dfm09mod", "M10/M20": "m10m20mod"}
	name, ok := names[family]
	if !ok {
		return "", nil, fmt.Errorf(i18n.Source("text.1af366a09eac"), family)
	}
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 96000 || rate > 20e6 || frequency <= 0 || math.Abs(float64(frequency-center))+24000 >= rate/2 {
		return "", nil, fmt.Errorf("%s", i18n.Source("text.a47da8c7c95d"))
	}
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	args := []string{"--json", "--ptu"}
	if family == i18n.Source("text.5945e8331ba7") || family == "RS41" {
		args = append(args, "--auto")
	}
	args = append(args, "--IQ", strconv.FormatFloat(float64(frequency-center)/rate, 'f', 9, 64), "--lpIQ", "--jsn_cfq", strconv.FormatInt(center, 10), "-", strconv.Itoa(int(math.Round(rate))), "32")
	return name, args, nil
}

func (d *Decoder) Configure(enabled bool, family string, frequency, center int64) {
	d.lifecycle.Lock()
	defer d.lifecycle.Unlock()
	d.mu.RLock()
	same := d.status.Running && d.status.Family == family && d.status.FrequencyHz == frequency && d.center == center
	d.mu.RUnlock()
	if same && family == i18n.Source("text.6ea56fae9eac") {
		same = d.Snapshot().Running
	}
	if enabled && same {
		return
	}
	d.stop()
	if !enabled {
		return
	}
	if family == i18n.Source("text.6ea56fae9eac") {
		d.startAutomatic(frequency, center)
		return
	}
	name, args, err := Arguments(family, d.rate, frequency, center)
	d.mu.Lock()
	d.status.Family = family
	d.status.FrequencyHz = frequency
	d.center = center
	d.status.Error = ""
	d.status.Detail = ""
	d.mu.Unlock()
	if err != nil {
		d.fail(err)
		return
	}
	cmd := exec.Command(filepath.Join(d.directory, name), args...)
	cmd.SysProcAttr = hiddenProcessAttributes()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		d.fail(err)
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		d.fail(err)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		d.fail(err)
		return
	}
	if err = cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		d.fail(err)
		return
	}
	s := &session{cmd: cmd, stdin: stdin, queue: make(chan []float32, 16), stop: make(chan struct{}), done: make(chan struct{}), writerDone: make(chan struct{})}
	d.mu.Lock()
	d.current = s
	d.status.Running = true
	d.status.State = i18n.Source("text.237eec4f6068")
	d.mu.Unlock()
	go func() {
		defer close(s.writerDone)
		for {
			select {
			case <-s.stop:
				return
			case iq := <-s.queue:
				payload := make([]byte, len(iq)*4)
				for i, v := range iq {
					binary.LittleEndian.PutUint32(payload[i*4:], math.Float32bits(v))
				}
				if _, err := s.stdin.Write(payload); err != nil {
					_ = cmd.Process.Kill()
					return
				}
			}
		}
	}()
	var readers sync.WaitGroup
	readers.Add(2)
	go func() {
		defer readers.Done()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 4096), 1024*1024)
		for scanner.Scan() {
			e, ok := ParseEvent(scanner.Bytes())
			if !ok {
				continue
			}
			e.FrequencyHz = frequency
			d.mu.Lock()
			d.events = append(d.events, e)
			if len(d.events) > 2000 {
				copy(d.events, d.events[len(d.events)-2000:])
				d.events = d.events[:2000]
			}
			if d.current == s {
				d.status.State = i18n.Source("text.3ec713946605")
			}
			d.mu.Unlock()
			if d.eventSink != nil {
				d.eventSink(e)
			}
		}
	}()
	go func() {
		defer readers.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			d.mu.Lock()
			d.status.Detail = strings.TrimSpace(scanner.Text())
			d.mu.Unlock()
		}
	}()
	go func() {
		readers.Wait()
		s.cancel()
		_ = s.stdin.Close()
		<-s.writerDone
		err := cmd.Wait()
		d.mu.Lock()
		if d.current == s {
			d.status.Running = false
			d.status.State = i18n.Source("text.97efa5c193f3")
			if err != nil {
				d.status.Error = err.Error()
				d.status.State = i18n.Source("text.d98ee0e5f939")
			}
		}
		d.mu.Unlock()
		close(s.done)
	}()
}

func (d *Decoder) fail(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.status.State = i18n.Source("text.d98ee0e5f939")
	d.status.Error = err.Error()
}
func (d *Decoder) stop() {
	d.mu.Lock()
	s := d.current
	children := d.children
	d.children = nil
	d.generation++
	d.detections = nil
	d.current = nil
	d.status.Running = false
	d.status.State = i18n.Source("text.7dc7253c376a")
	d.mu.Unlock()
	for _, child := range children {
		child.Close()
	}
	if s != nil {
		s.cancel()
		_ = s.cmd.Process.Kill()
		_ = s.stdin.Close()
		<-s.done
	}
}
func (d *Decoder) Close() { d.lifecycle.Lock(); defer d.lifecycle.Unlock(); d.stop() }
func (d *Decoder) ProcessIQ(iq []float32) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.children) > 0 {
		for _, child := range d.children {
			child.ProcessIQ(iq)
		}
		return
	}
	if !d.status.Running || d.current == nil || len(iq) == 0 || len(iq)%2 != 0 {
		return
	}
	select {
	case d.current.queue <- append([]float32(nil), iq...):
	default:
		d.status.Dropped++
	}
}
func (d *Decoder) Snapshot() Status {
	d.mu.RLock()
	s := d.status
	s.Events = len(d.events)
	children := append([]*Decoder(nil), d.children...)
	detected := []string{}
	for _, family := range Families {
		if v := d.detections[family]; v.count >= 2 && time.Since(v.last) < 15*time.Second {
			detected = append(detected, family)
		}
	}
	d.mu.RUnlock()
	if len(children) > 0 {
		s.Running = false
		s.Dropped = 0
		errors := []string{}
		for i, c := range children {
			cs := c.Snapshot()
			s.Running = s.Running || cs.Running
			s.Dropped += cs.Dropped
			if !cs.Running {
				errors = append(errors, Families[i]+": "+cs.State+" "+cs.Error)
			}
		}
		s.Error = strings.Join(errors, "; ")
		s.State = i18n.Source("text.f8bd2e05139c")
		if len(detected) > 0 {
			s.State = i18n.Source("text.331556128e97") + strings.Join(detected, " + ")
		}
		if !s.Running {
			s.State = i18n.Source("text.d98ee0e5f939")
		}
	}
	return s
}
func (d *Decoder) Events() []Event {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return append([]Event(nil), d.events...)
}
func (d *Decoder) Clear() {
	d.mu.Lock()
	d.events = nil
	d.status.Dropped = 0
	d.detections = make(map[string]autoDetection)
	children := append([]*Decoder(nil), d.children...)
	d.mu.Unlock()
	for _, c := range children {
		c.Clear()
	}
}
