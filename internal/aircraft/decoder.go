package aircraft

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	Mode1090              = "1090 ADS-B"
	Mode978               = "978 UAT"
	Frequency1090Hz int64 = 1_090_000_000
	Frequency978Hz  int64 = 978_000_000
)

type Aircraft struct {
	ICAO         string    `json:"icao"`
	Callsign     string    `json:"callsign,omitempty"`
	Latitude     *float64  `json:"lat,omitempty"`
	Longitude    *float64  `json:"lon,omitempty"`
	Altitude     *int      `json:"altitudeFt,omitempty"`
	Speed        *float64  `json:"speedKt,omitempty"`
	Track        *float64  `json:"trackDeg,omitempty"`
	VerticalRate *float64  `json:"verticalRateFpm,omitempty"`
	Squawk       string    `json:"squawk,omitempty"`
	Category     string    `json:"category,omitempty"`
	Source       string    `json:"source,omitempty"`
	OnGround     bool      `json:"onGround,omitempty"`
	LastSeen     time.Time `json:"lastSeen"`
	Messages     uint64    `json:"messages"`
}
type Status struct {
	Running                    bool
	Mode, State, Error, Detail string
	Dropped                    uint64
	Messages, Aircraft         int
}
type cprFrame struct {
	lat, lon int
	odd      bool
	surface  bool
	at       time.Time
}
type trackState struct {
	aircraft       Aircraft
	even, odd      *cprFrame
	refLat, refLon float64
	hasRef         bool
	lastPos        time.Time
}

type Decoder struct {
	mu, lifecycle                  sync.Mutex
	rate                           float64
	exe1090, exe978, exeUATText    string
	mode, state, lastError, detail string
	running                        bool
	queue                          chan []float32
	stop, done                     chan struct{}
	cmd, helper                    *exec.Cmd
	stdin                          io.WriteCloser
	dropped                        uint64
	messages                       int
	tracks                         map[string]*trackState
	magTail                        []uint16
	receiverLat, receiverLon       float64
	hasReceiverRef                 bool
}

func New(rate float64, exe1090, exe978, exeUATText string) *Decoder {
	return &Decoder{rate: rate, exe1090: exe1090, exe978: exe978, exeUATText: exeUATText, state: "STOPPED", tracks: make(map[string]*trackState)}
}
func (d *Decoder) Configure(enabled bool, mode string) {
	d.lifecycle.Lock()
	defer d.lifecycle.Unlock()
	d.stopProcess()
	if !enabled {
		return
	}
	if mode != Mode978 {
		mode = Mode1090
	}
	d.mode = mode
	d.queue = make(chan []float32, 64)
	d.stop = make(chan struct{})
	d.done = make(chan struct{})
	d.magTail = nil
	if mode == Mode1090 {
		d.running = true
		d.state = "WAITING FOR AIRCRAFT"
		d.lastError = ""
		d.detail = "native 1090 demod"
		go func(stop, done chan struct{}) {
			defer close(done)
			for {
				select {
				case <-stop:
					return
				case iq := <-d.queue:
					d.consume1090(iq)
				}
			}
		}(d.stop, d.done)
		return
	}
	var stdout io.ReadCloser
	var err error
	if d.exe978 == "" || d.exeUATText == "" {
		d.fail(errors.New("dump978 not configured"))
		return
	}
	d.cmd = exec.Command(d.exe978)
	d.helper = exec.Command(d.exeUATText)
	d.cmd.SysProcAttr = hiddenProcessAttributes()
	d.helper.SysProcAttr = hiddenProcessAttributes()
	d.stdin, err = d.cmd.StdinPipe()
	var pipe io.ReadCloser
	if err == nil {
		pipe, err = d.cmd.StdoutPipe()
	}
	if err == nil {
		d.helper.Stdin = pipe
		stdout, err = d.helper.StdoutPipe()
	}
	if err == nil {
		err = d.helper.Start()
	}
	if err == nil {
		err = d.cmd.Start()
	}
	if err != nil {
		if d.stdin != nil {
			_ = d.stdin.Close()
		}
		d.fail(err)
		return
	}
	d.running = true
	d.state = "WAITING FOR AIRCRAFT"
	d.lastError = ""
	d.detail = "978 UAT"
	go d.writeIQ()
	go d.read978(stdout)
	cmd, helper, done := d.cmd, d.helper, d.done
	go func() {
		err := cmd.Wait()
		if helper != nil && helper.Process != nil {
			_ = helper.Process.Kill()
			_ = helper.Wait()
		}
		d.mu.Lock()
		if d.cmd == cmd {
			d.running = false
			if err != nil {
				d.state = "ERROR"
				d.lastError = err.Error()
			} else {
				d.state = "FINISHED"
			}
		}
		d.mu.Unlock()
		close(done)
	}()
}
func (d *Decoder) fail(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.running = false
	d.state = "ERROR"
	d.lastError = err.Error()
}
func (d *Decoder) SetReference(lat, lon float64, ok bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.hasReceiverRef = ok && validLatLon(lat, lon)
	d.receiverLat, d.receiverLon = lat, lon
	if !d.hasReceiverRef {
		return
	}
	for _, t := range d.tracks {
		if t.aircraft.Latitude == nil {
			t.refLat, t.refLon, t.hasRef = lat, lon, true
		}
	}
}

func (d *Decoder) stopProcess() {
	d.mu.Lock()
	cmd, helper, stdin, stop, done := d.cmd, d.helper, d.stdin, d.stop, d.done
	active := d.running || cmd != nil
	d.running = false
	d.cmd = nil
	d.helper = nil
	d.stdin = nil
	d.mu.Unlock()
	if !active {
		d.mu.Lock()
		d.state = "STOPPED"
		d.mu.Unlock()
		return
	}
	if stop != nil {
		close(stop)
	}
	if stdin != nil {
		_ = stdin.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	if helper != nil && helper.Process != nil {
		_ = helper.Process.Kill()
	}
	if done != nil {
		<-done
	}
	d.mu.Lock()
	d.state = "STOPPED"
	d.mu.Unlock()
}
func (d *Decoder) Close() { d.lifecycle.Lock(); defer d.lifecycle.Unlock(); d.stopProcess() }
func (d *Decoder) ProcessIQ(iq []float32) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.running || len(iq) < 2 {
		return
	}
	select {
	case d.queue <- append([]float32(nil), iq...):
	default:
		d.dropped++
	}
}
func (d *Decoder) writeIQ() {
	stop := d.stop
	for {
		select {
		case <-stop:
			return
		case iq := <-d.queue:
			samples := resampleCU8(iq, d.rate, 2_083_334)
			if _, err := d.stdin.Write(samples); err != nil {
				return
			}
		}
	}
}

func (d *Decoder) consume1090(iq []float32) {
	samples := resampleIQ(iq, d.rate, 2_000_000)
	mag := magnitudes(samples)
	if len(d.magTail) > 0 {
		combined := make([]uint16, 0, len(d.magTail)+len(mag))
		combined = append(combined, d.magTail...)
		mag = append(combined, mag...)
	}
	frames := detectModeS(mag)
	if len(mag) > modeSOverlapSamples {
		d.magTail = append([]uint16(nil), mag[len(mag)-modeSOverlapSamples:]...)
	} else {
		d.magTail = append([]uint16(nil), mag...)
	}
	seen := map[string]struct{}{}
	for _, raw := range frames {
		key := hex.EncodeToString(raw)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		d.decode1090(raw)
	}
}
func resampleCU8(iq []float32, inputRate, outputRate float64) []byte {
	n := len(iq) / 2
	if n == 0 {
		return nil
	}
	outN := int(float64(n) * outputRate / inputRate)
	out := make([]byte, outN*2)
	for j := 0; j < outN; j++ {
		p := float64(j) * inputRate / outputRate
		i := int(p)
		if i >= n {
			i = n - 1
		}
		for q := 0; q < 2; q++ {
			x := iq[i*2+q]
			if x > 1 {
				x = 1
			}
			if x < -1 {
				x = -1
			}
			out[j*2+q] = byte((x + 1) * 127.5)
		}
	}
	return out
}

func (d *Decoder) read1090(r io.Reader) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if len(line) > 2 && line[0] == '*' {
			line = strings.TrimSuffix(strings.TrimPrefix(line, "*"), ";")
			raw, err := hex.DecodeString(line)
			if err == nil {
				d.decode1090(raw)
			}
		}
	}
}
func bit(data []byte, start, length int) uint64 {
	var v uint64
	for i := 0; i < length; i++ {
		p := start + i
		v = (v << 1) | uint64((data[p/8]>>uint(7-p%8))&1)
	}
	return v
}
func (d *Decoder) stateFor(icao, source string) *trackState {
	t := d.tracks[icao]
	if t == nil {
		t = &trackState{aircraft: Aircraft{ICAO: icao, Source: source}}
		d.tracks[icao] = t
	}
	if !t.hasRef && d.hasReceiverRef {
		t.refLat, t.refLon, t.hasRef = d.receiverLat, d.receiverLon, true
	}
	return t
}
func (d *Decoder) decode1090(raw []byte) {
	if len(raw) != 14 {
		return
	}
	df := raw[0] >> 3
	if df != 17 && df != 18 {
		return
	}
	icao := strings.ToUpper(hex.EncodeToString(raw[1:4]))
	tc := int(bit(raw, 32, 5))
	now := time.Now().UTC()
	d.mu.Lock()
	defer d.mu.Unlock()
	t := d.stateFor(icao, "1090 ADS-B")
	a := &t.aircraft
	a.Messages++
	a.LastSeen = now
	d.messages++
	switch {
	case tc >= 1 && tc <= 4:
		chars := "#ABCDEFGHIJKLMNOPQRSTUVWXYZ#####_###############0123456789######"
		var b strings.Builder
		for i := 0; i < 8; i++ {
			c := int(bit(raw, 40+i*6, 6))
			if c < len(chars) {
				b.WriteByte(chars[c])
			}
		}
		a.Callsign = strings.TrimSpace(strings.ReplaceAll(b.String(), "_", " "))
		a.Category = fmt.Sprintf("TC %d", tc)
	case tc >= 5 && tc <= 8:
		t.applyCPR(a, int(bit(raw, 54, 17)), int(bit(raw, 71, 17)), bit(raw, 53, 1) == 1, true, now)
	case (tc >= 9 && tc <= 18) || (tc >= 20 && tc <= 22):
		if tc <= 18 {
			q := bit(raw, 47, 1)
			altCode := int(bit(raw, 40, 12))
			if q == 1 {
				alt := ((altCode & 0xFE0) >> 1) | (altCode & 0xF)
				alt = alt*25 - 1000
				a.Altitude = &alt
			}
		}
		t.applyCPR(a, int(bit(raw, 54, 17)), int(bit(raw, 71, 17)), bit(raw, 53, 1) == 1, false, now)
	case tc == 19:
		sub := int(bit(raw, 37, 3))
		if sub == 1 || sub == 2 {
			ew := float64(bit(raw, 46, 10)) - 1
			if bit(raw, 45, 1) == 1 {
				ew = -ew
			}
			ns := float64(bit(raw, 57, 10)) - 1
			if bit(raw, 56, 1) == 1 {
				ns = -ns
			}
			speed := math.Hypot(ew, ns)
			track := math.Mod(math.Atan2(ew, ns)*180/math.Pi+360, 360)
			a.Speed = &speed
			a.Track = &track
			vrCode := bit(raw, 69, 9)
			if vrCode > 0 {
				vr := float64(vrCode-1) * 64
				if bit(raw, 68, 1) == 1 {
					vr = -vr
				}
				a.VerticalRate = &vr
			}
		}
	}
}
func (t *trackState) applyCPR(a *Aircraft, cprlat, cprlon int, odd, surface bool, now time.Time) {
	f := &cprFrame{lat: cprlat, lon: cprlon, odd: odd, surface: surface, at: now}
	if odd {
		t.odd = f
	} else {
		t.even = f
	}
	maxAge := 10.0
	if surface {
		maxAge = 25
	}
	if t.even != nil && t.odd != nil && t.even.surface == t.odd.surface && t.even.surface == surface && math.Abs(t.even.at.Sub(t.odd.at).Seconds()) <= maxAge {
		if !surface {
			if lat, lon, ok := decodeCPRAirborne(t.even.lat, t.even.lon, t.odd.lat, t.odd.lon, odd); ok {
				t.setPosition(a, lat, lon, false, now)
				return
			}
		} else if t.hasRef {
			if lat, lon, ok := decodeCPRSurface(t.even.lat, t.even.lon, t.odd.lat, t.odd.lon, odd, t.refLon); ok {
				t.setPosition(a, lat, lon, true, now)
				return
			}
		}
	}
	if t.hasRef {
		if lat, lon, ok := decodeCPRRelative(t.refLat, t.refLon, cprlat, cprlon, odd, surface); ok {
			t.setPosition(a, lat, lon, surface, now)
		}
	}
}

func (t *trackState) setPosition(a *Aircraft, lat, lon float64, onGround bool, now time.Time) {
	if !validLatLon(lat, lon) {
		return
	}
	if a.Latitude != nil && a.Longitude != nil {
		dist := earthDistanceKm(*a.Latitude, *a.Longitude, lat, lon)
		dt := now.Sub(t.lastPos).Hours()
		if t.lastPos.IsZero() || dt < 0 {
			dt = 1.0 / 3600
		}
		if dt < 1.0/3600 {
			dt = 1.0 / 3600
		}
		speed := 2800.0
		slop := 8.0
		if a.OnGround || onGround {
			speed = 370
			slop = 2
		}
		if dist > slop+speed*dt {
			return
		}
	}
	a.Latitude, a.Longitude = &lat, &lon
	a.OnGround = onGround
	t.refLat, t.refLon, t.hasRef = lat, lon, true
	t.lastPos = now
}

func (d *Decoder) read978(r io.Reader) {
	s := bufio.NewScanner(r)
	fields := map[string]string{}
	flush := func() {
		icao := fields["Address"]
		if icao == "" {
			return
		}
		d.mu.Lock()
		defer d.mu.Unlock()
		t := d.stateFor(icao, "978 UAT")
		a := &t.aircraft
		a.Messages++
		a.LastSeen = time.Now().UTC()
		d.messages++
		a.Callsign = fields["Callsign"]
		if v, ok := parseFloatField(fields["Latitude"]); ok && validLatLon(v, 0) {
			a.Latitude = &v
		}
		if v, ok := parseFloatField(fields["Longitude"]); ok && validLatLon(0, v) {
			a.Longitude = &v
		}
		if a.Latitude == nil || a.Longitude == nil || !validLatLon(*a.Latitude, *a.Longitude) {
			a.Latitude, a.Longitude = nil, nil
		}
		if v, ok := parseIntField(fields["Altitude"]); ok {
			a.Altitude = &v
		}
		if v, ok := parseFloatField(fields["Speed"]); ok {
			a.Speed = &v
		}
		if v, ok := parseFloatField(fields["Track"]); ok {
			a.Track = &v
		}
		if v, ok := parseFloatField(fields["Vertical rate"]); ok {
			a.VerticalRate = &v
		}
		fields = map[string]string{}
	}
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "HDR:" {
			flush()
			continue
		}
		if i := strings.Index(line, ":"); i > 0 {
			key, val := strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:])
			if key == "Address" {
				val = strings.Fields(val)[0]
			}
			fields[key] = val
		}
	}
	flush()
}
func parseFloatField(s string) (float64, bool) {
	f := strings.Fields(s)
	if len(f) == 0 {
		return 0, false
	}
	v, e := strconv.ParseFloat(strings.TrimPrefix(f[0], "+"), 64)
	return v, e == nil
}
func parseIntField(s string) (int, bool) { v, ok := parseFloatField(s); return int(v), ok }

func (d *Decoder) Snapshot() Status {
	d.mu.Lock()
	defer d.mu.Unlock()
	return Status{d.running, d.mode, d.state, d.lastError, d.detail, d.dropped, d.messages, len(d.tracks)}
}
func (d *Decoder) Aircraft() []Aircraft {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]Aircraft, 0, len(d.tracks))
	for _, t := range d.tracks {
		out = append(out, t.aircraft)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastSeen.After(out[j].LastSeen) })
	return out
}
func (d *Decoder) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.tracks = make(map[string]*trackState)
	d.messages = 0
	d.dropped = 0
}
