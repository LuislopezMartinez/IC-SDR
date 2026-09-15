package ais

import (
	"go-zero/internal/i18n"

	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

const CenterFrequencyHz int64 = 162_000_000

type Vessel struct {
	MMSI         uint32    `json:"mmsi"`
	Name         string    `json:"name,omitempty"`
	Callsign     string    `json:"callsign,omitempty"`
	ShipType     int       `json:"shipType,omitempty"`
	ShipTypeText string    `json:"shipTypeText,omitempty"`
	Status       int       `json:"status,omitempty"`
	StatusText   string    `json:"statusText,omitempty"`
	Latitude     *float64  `json:"lat,omitempty"`
	Longitude    *float64  `json:"lon,omitempty"`
	Speed        *float64  `json:"speed,omitempty"`
	Course       *float64  `json:"course,omitempty"`
	Heading      *float64  `json:"heading,omitempty"`
	Destination  string    `json:"destination,omitempty"`
	LastSeen     time.Time `json:"lastSeen"`
	Messages     uint64    `json:"messages"`
}
type message struct {
	MMSI         uint32   `json:"mmsi"`
	Name         string   `json:"shipname"`
	Callsign     string   `json:"callsign"`
	ShipType     int      `json:"shiptype"`
	ShipTypeText string   `json:"shiptype_text"`
	Status       int      `json:"status"`
	StatusText   string   `json:"status_text"`
	Lat          *float64 `json:"lat"`
	Lon          *float64 `json:"lon"`
	Speed        *float64 `json:"speed"`
	Course       *float64 `json:"course"`
	Heading      *float64 `json:"heading"`
	Destination  string   `json:"destination"`
}

func parseMessage(line []byte) (message, bool) {
	var m message
	if json.Unmarshal(line, &m) != nil || m.MMSI == 0 {
		return message{}, false
	}
	if (m.Lat != nil && (*m.Lat < -90 || *m.Lat > 90)) || (m.Lon != nil && (*m.Lon < -180 || *m.Lon > 180)) {
		return message{}, false
	}
	return m, true
}

type Status struct {
	Running              bool
	State, Error, Detail string
	Dropped              uint64
	Messages, Vessels    int
}
type Decoder struct {
	mu                       sync.RWMutex
	lifecycle                sync.Mutex
	executable               string
	rate                     float64
	queue                    chan []float32
	stop, done               chan struct{}
	cmd                      *exec.Cmd
	stdin                    io.WriteCloser
	running                  bool
	dropped                  uint64
	state, lastError, detail string
	messages                 int
	vessels                  map[uint32]Vessel
	session                  uint64
}

func New(rate float64, executable string) *Decoder {
	return &Decoder{rate: rate, executable: executable, state: i18n.Source("text.7dc7253c376a"), vessels: make(map[uint32]Vessel)}
}
func (d *Decoder) Configure(enabled bool) {
	d.lifecycle.Lock()
	defer d.lifecycle.Unlock()
	d.stopProcess()
	if !enabled {
		return
	}
	if d.executable == "" {
		d.fail(errors.New(i18n.Source("text.dc1932b0199c")))
		return
	}
	d.queue = make(chan []float32, 16)
	d.stop = make(chan struct{})
	d.done = make(chan struct{})
	d.cmd = exec.Command(d.executable, "-r", "CF32", ".", "-s", strconv.Itoa(int(math.Round(d.rate))), "-o", "5", "-G", i18n.Source("text.d81674b2bdd5"), i18n.Source("text.d98ee0e5f939"))
	d.cmd.SysProcAttr = hiddenProcessAttributes()
	stdin, err := d.cmd.StdinPipe()
	if err != nil {
		d.fail(err)
		return
	}
	stdout, err := d.cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		d.fail(err)
		return
	}
	stderr, _ := d.cmd.StderrPipe()
	if err = d.cmd.Start(); err != nil {
		stdin.Close()
		d.fail(err)
		return
	}
	d.mu.Lock()
	d.session++
	session := d.session
	d.stdin = stdin
	d.running = true
	d.state = i18n.Source("text.2565ad5076f8")
	d.lastError = ""
	d.detail = ""
	d.mu.Unlock()
	go d.writeIQ(stdin)
	go d.readJSON(stdout, session)
	go d.readErrors(stderr)
	go func(cmd *exec.Cmd, done chan struct{}) {
		err := cmd.Wait()
		d.mu.Lock()
		if d.cmd == cmd {
			d.running = false
			if err != nil {
				d.state = i18n.Source("text.d98ee0e5f939")
				d.lastError = err.Error()
			} else {
				d.state = i18n.Source("text.97efa5c193f3")
			}
		}
		d.mu.Unlock()
		close(done)
	}(d.cmd, d.done)
}
func (d *Decoder) writeIQ(w io.Writer) {
	for {
		select {
		case <-d.stop:
			return
		case iq := <-d.queue:
			payload := make([]byte, len(iq)*4)
			for i, v := range iq {
				binary.LittleEndian.PutUint32(payload[i*4:], math.Float32bits(v))
			}
			if _, err := w.Write(payload); err != nil {
				return
			}
		}
	}
}
func (d *Decoder) readJSON(r io.Reader, session uint64) {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 4096), 1024*1024)
	for s.Scan() {
		if m, ok := parseMessage(s.Bytes()); ok {
			d.mergeSession(m, session)
		}
	}
}

func (d *Decoder) mergeSession(m message, session uint64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.running || d.session != session {
		return
	}
	d.mergeLocked(m)
}
func (d *Decoder) readErrors(r io.Reader) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		d.mu.Lock()
		d.detail = s.Text()
		d.mu.Unlock()
	}
}
func (d *Decoder) merge(m message) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.mergeLocked(m)
}

func (d *Decoder) mergeLocked(m message) {
	v := d.vessels[m.MMSI]
	v.MMSI = m.MMSI
	if m.Name != "" {
		v.Name = m.Name
	}
	if m.Callsign != "" {
		v.Callsign = m.Callsign
	}
	if m.ShipType != 0 {
		v.ShipType = m.ShipType
	}
	if m.ShipTypeText != "" {
		v.ShipTypeText = m.ShipTypeText
	}
	if m.StatusText != "" {
		v.Status = m.Status
		v.StatusText = m.StatusText
	}
	if m.Lat != nil {
		v.Latitude = m.Lat
	}
	if m.Lon != nil {
		v.Longitude = m.Lon
	}
	if m.Speed != nil {
		v.Speed = m.Speed
	}
	if m.Course != nil {
		v.Course = m.Course
	}
	if m.Heading != nil {
		v.Heading = m.Heading
	}
	if m.Destination != "" {
		v.Destination = m.Destination
	}
	v.LastSeen = time.Now().UTC()
	v.Messages++
	d.vessels[m.MMSI] = v
	d.messages++
	d.state = i18n.Source("text.3ec713946605")
}
func (d *Decoder) ProcessIQ(iq []float32) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.running || len(iq) == 0 {
		return
	}
	select {
	case d.queue <- append([]float32(nil), iq...):
	default:
		d.dropped++
	}
}
func (d *Decoder) Snapshot() Status {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return Status{d.running, d.state, d.lastError, d.detail, d.dropped, d.messages, len(d.vessels)}
}
func (d *Decoder) Vessels() []Vessel {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]Vessel, 0, len(d.vessels))
	for _, v := range d.vessels {
		out = append(out, v)
	}
	return out
}
func (d *Decoder) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.vessels = make(map[uint32]Vessel)
	d.messages = 0
	d.dropped = 0
}
func (d *Decoder) Close() { d.lifecycle.Lock(); defer d.lifecycle.Unlock(); d.stopProcess() }
func (d *Decoder) stopProcess() {
	d.mu.Lock()
	if !d.running {
		d.state = i18n.Source("text.7dc7253c376a")
		d.mu.Unlock()
		return
	}
	cmd, stdin, stop, done := d.cmd, d.stdin, d.stop, d.done
	d.running = false
	d.cmd = nil
	d.session++
	d.mu.Unlock()
	close(stop)
	_ = stdin.Close()
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	<-done
	d.mu.Lock()
	d.state = i18n.Source("text.7dc7253c376a")
	d.mu.Unlock()
}
func (d *Decoder) fail(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.running = false
	d.state = i18n.Source("text.d98ee0e5f939")
	d.lastError = err.Error()
}
