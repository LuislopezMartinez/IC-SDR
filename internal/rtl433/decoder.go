package rtl433

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type Event struct {
	Received time.Time `json:"received"`
	Time     string    `json:"time,omitempty"`
	Protocol int       `json:"protocol,omitempty"`
	Model    string    `json:"model,omitempty"`
	Type     string    `json:"type,omitempty"`
	ID       string    `json:"id,omitempty"`
	Channel  string    `json:"channel,omitempty"`
	Mod      string    `json:"mod,omitempty"`
	FreqMHz  float64   `json:"frequencyMHz,omitempty"`
	RSSI     float64   `json:"rssi,omitempty"`
	SNR      float64   `json:"snr,omitempty"`
	Summary  string    `json:"summary,omitempty"`
	Raw      string    `json:"raw"`
}

type Status struct {
	Running         bool
	State, Error    string
	Queued, Dropped uint64
	Events          int
	FrequencyHz     int64
	BandwidthHz     int
	SampleRate      int
}

type Decoder struct {
	inputRate               float64
	executable              string
	mu                      sync.RWMutex
	events                  []Event
	state, lastError        string
	frequencyHz, centerHz   int64
	bandwidthHz, outputRate int
	front                   *frontend
	queue                   chan []float32
	stop                    chan struct{}
	done                    chan struct{}
	running                 atomic.Bool
	dropped                 atomic.Uint64
	cmd                     *exec.Cmd
	listener                net.Listener
	manager                 bool
	children                []*Decoder
	eventSink               func(Event)
}

func New(inputRate float64, executable string) *Decoder {
	return &Decoder{inputRate: inputRate, executable: executable, state: "STOPPED"}
}

func (d *Decoder) Configure(enabled bool, frequencyHz, centerHz int64, bandwidthHz int) {
	if !enabled {
		d.Stop()
		return
	}
	d.mu.RLock()
	same := d.running.Load() && d.frequencyHz == frequencyHz && d.centerHz == centerHz && d.bandwidthHz == bandwidthHz
	d.mu.RUnlock()
	if same {
		return
	}
	d.Stop()
	if bandwidthHz >= 1_000_000 {
		d.configureMultichannel(frequencyHz, centerHz, bandwidthHz)
		return
	}
	d.configureSingle(frequencyHz, centerHz, bandwidthHz)
}

func (d *Decoder) configureSingle(frequencyHz, centerHz int64, bandwidthHz int) {
	d.mu.Lock()
	d.frequencyHz, d.centerHz, d.bandwidthHz = frequencyHz, centerHz, bandwidthHz
	d.front = newFrontend(d.inputRate, float64(frequencyHz-centerHz), bandwidthHz)
	d.outputRate = d.front.outputRate
	d.mu.Unlock()
	d.start()
}

// configureMultichannel uses the wide SDR capture as a panorama, while every
// rtl_433 process receives a clean 250 kHz virtual receiver. This avoids the
// noise and sensitivity penalty of asking one decoder to consume 1 or 2 MHz.
func (d *Decoder) configureMultichannel(frequencyHz, centerHz int64, bandwidthHz int) {
	centers := multichannelCenters(frequencyHz, bandwidthHz)
	children := make([]*Decoder, 0, len(centers))
	for _, channelHz := range centers {
		child := New(d.inputRate, d.executable)
		child.eventSink = d.addEvent
		child.configureSingle(channelHz, centerHz, 250_000)
		children = append(children, child)
	}
	d.mu.Lock()
	d.manager = true
	d.children = children
	d.frequencyHz, d.centerHz, d.bandwidthHz = frequencyHz, centerHz, bandwidthHz
	d.outputRate = 256_000
	d.state = fmt.Sprintf("MULTICHANNEL %d×250 kHz", len(centers))
	d.lastError = ""
	d.mu.Unlock()
	d.running.Store(true)
}

func multichannelCenters(frequencyHz int64, bandwidthHz int) []int64 {
	channels := max(bandwidthHz/250_000, 1)
	centers := make([]int64, channels)
	low := frequencyHz - int64(bandwidthHz)/2
	for index := range centers {
		centers[index] = low + 125_000 + int64(index)*250_000
	}
	return centers
}

func (d *Decoder) start() {
	if d.executable == "" {
		d.setError(errors.New("rtl_433 not configured"))
		return
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		d.setError(err)
		return
	}
	d.queue, d.stop, d.done, d.listener = make(chan []float32, 12), make(chan struct{}), make(chan struct{}), listener
	port := listener.Addr().(*net.TCPAddr).Port
	d.cmd = exec.Command(d.executable, "-d", "rtl_tcp://127.0.0.1:"+strconv.Itoa(port), "-s", strconv.Itoa(d.outputRate), "-f", strconv.FormatInt(d.frequencyHz, 10), "-C", "si", "-M", "protocol", "-M", "level", "-F", "json")
	d.cmd.SysProcAttr = hiddenProcessAttributes()
	stdout, err := d.cmd.StdoutPipe()
	if err != nil {
		listener.Close()
		d.setError(err)
		return
	}
	stderr, _ := d.cmd.StderrPipe()
	if err = d.cmd.Start(); err != nil {
		listener.Close()
		d.setError(err)
		return
	}
	d.running.Store(true)
	d.mu.Lock()
	d.state, d.lastError = "WAITING FOR SIGNAL", ""
	d.mu.Unlock()
	go d.transport(listener)
	go d.readEvents(stdout)
	go d.readErrors(stderr)
}

func (d *Decoder) transport(listener net.Listener) {
	defer close(d.done)
	if tcp, ok := listener.(*net.TCPListener); ok {
		_ = tcp.SetDeadline(time.Now().Add(8 * time.Second))
	}
	connection, err := listener.Accept()
	if err != nil {
		if d.running.Load() {
			d.setError(err)
		}
		return
	}
	defer connection.Close()
	_, _ = connection.Write([]byte{'R', 'T', 'L', '0', 0, 0, 0, 5, 0, 0, 0, 0})
	d.mu.Lock()
	d.state = "DECODIFICANDO"
	d.mu.Unlock()
	for {
		select {
		case <-d.stop:
			return
		case iq := <-d.queue:
			d.mu.Lock()
			payload := d.front.process(iq)
			d.mu.Unlock()
			if len(payload) > 0 {
				if _, err = connection.Write(payload); err != nil {
					d.setError(err)
					return
				}
			}
		}
	}
}

func (d *Decoder) ProcessIQ(iq []float32) {
	if !d.running.Load() || d.queue == nil {
		d.mu.RLock()
		manager, children := d.manager, append([]*Decoder(nil), d.children...)
		d.mu.RUnlock()
		if manager {
			for _, child := range children {
				child.ProcessIQ(iq)
			}
		}
		return
	}
	copyIQ := append([]float32(nil), iq...)
	select {
	case d.queue <- copyIQ:
	default:
		d.dropped.Add(1)
	}
}

func (d *Decoder) readEvents(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		if event, ok := ParseEvent(scanner.Bytes()); ok {
			d.addEvent(event)
		}
	}
}
func (d *Decoder) readErrors(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		d.mu.Lock()
		d.state = scanner.Text()
		d.mu.Unlock()
	}
}

func ParseEvent(line []byte) (Event, bool) {
	var values map[string]any
	if json.Unmarshal(line, &values) != nil {
		return Event{}, false
	}
	e := Event{Received: time.Now(), Raw: string(line), Time: textValue(values["time"]), Model: textValue(values["model"]), Type: textValue(values["type"]), ID: textValue(values["id"]), Channel: textValue(values["channel"]), Mod: textValue(values["mod"]), Protocol: int(numberValue(values["protocol"])), RSSI: numberValue(values["rssi"]), SNR: numberValue(values["snr"])}
	e.FreqMHz = numberValue(values["freq"])
	if e.FreqMHz == 0 {
		e.FreqMHz = numberValue(values["freq1"])
	}
	for _, key := range []string{"temperature_C", "humidity", "pressure_kPa", "wind_avg_km_h", "battery_mV", "battery_ok"} {
		if value, exists := values[key]; exists {
			if e.Summary != "" {
				e.Summary += " · "
			}
			e.Summary += fmt.Sprintf("%s %v", key, value)
		}
	}
	return e, e.Model != "" || e.Protocol != 0
}
func textValue(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
func numberValue(v any) float64 {
	if n, ok := v.(float64); ok {
		return n
	}
	return 0
}

func (d *Decoder) addEvent(event Event) {
	if d.eventSink != nil {
		d.eventSink(event)
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	key := event.Model + "|" + event.ID + "|" + event.Channel
	for i, old := range d.events {
		if old.Model+"|"+old.ID+"|"+old.Channel == key {
			d.events = append(d.events[:i], d.events[i+1:]...)
			break
		}
	}
	d.events = append([]Event{event}, d.events...)
	if len(d.events) > 100 {
		d.events = d.events[:100]
	}
}
func (d *Decoder) Events() []Event {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return append([]Event(nil), d.events...)
}
func (d *Decoder) Clear() { d.mu.Lock(); d.events = nil; d.mu.Unlock() }
func (d *Decoder) Snapshot() Status {
	d.mu.RLock()
	manager, children := d.manager, append([]*Decoder(nil), d.children...)
	status := Status{Running: d.running.Load(), State: d.state, Error: d.lastError, Queued: uint64(len(d.queue)), Dropped: d.dropped.Load(), Events: len(d.events), FrequencyHz: d.frequencyHz, BandwidthHz: d.bandwidthHz, SampleRate: d.outputRate}
	d.mu.RUnlock()
	if manager {
		status.Running = false
		for _, child := range children {
			childStatus := child.Snapshot()
			status.Running = status.Running || childStatus.Running
			status.Queued += childStatus.Queued
			status.Dropped += childStatus.Dropped
			if childStatus.Error != "" && status.Error == "" {
				status.Error = childStatus.Error
			}
		}
	}
	return status
}
func (d *Decoder) setError(err error) {
	d.mu.Lock()
	d.lastError = err.Error()
	d.state = "ERROR"
	d.mu.Unlock()
}

func (d *Decoder) Stop() {
	if !d.running.Swap(false) {
		return
	}
	d.mu.Lock()
	if d.manager {
		children := d.children
		d.children = nil
		d.manager = false
		d.state = "STOPPED"
		d.mu.Unlock()
		for _, child := range children {
			child.Stop()
		}
		return
	}
	d.mu.Unlock()
	close(d.stop)
	if d.listener != nil {
		_ = d.listener.Close()
	}
	if d.cmd != nil && d.cmd.Process != nil {
		_ = d.cmd.Process.Kill()
		_, _ = d.cmd.Process.Wait()
	}
	select {
	case <-d.done:
	case <-time.After(time.Second):
	}
	d.mu.Lock()
	d.state = "STOPPED"
	d.mu.Unlock()
}
