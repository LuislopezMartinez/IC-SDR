package satellite

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Station struct {
	Name           string  `json:"name"`
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
	AltitudeMeters float64 `json:"altitudeMeters"`
}

type Signal struct {
	Name, Mode string
	DownlinkHz int64
}

type Elements struct {
	Epoch                                                                     time.Time
	Inclination, RAAN, Eccentricity, ArgumentPerigee, MeanAnomaly, MeanMotion float64
}

type Satellite struct {
	Name, Group  string
	NORAD        int
	Line1, Line2 string
	Signals      []Signal
	Elements     Elements
}

type Point struct{ Latitude, Longitude float64 }

type PassPrediction struct {
	Found        bool      `json:"found"`
	Continuous   bool      `json:"continuous,omitempty"`
	InProgress   bool      `json:"inProgress,omitempty"`
	AOS          time.Time `json:"aos,omitempty"`
	TCA          time.Time `json:"tca,omitempty"`
	LOS          time.Time `json:"los,omitempty"`
	MinRangeKM   float64   `json:"minRangeKm,omitempty"`
	MaxElevation float64   `json:"maxElevation,omitempty"`
}

type State struct {
	Name, Group, Signal, Mode                                    string
	NORAD                                                        int
	DownlinkHz                                                   int64
	Latitude, Longitude, AltitudeKM, Azimuth, Elevation, RangeKM float64
	Visible                                                      bool
	Trajectory                                                   []Point
	NextPass                                                     PassPrediction `json:"nextPass,omitempty"`
}

type Snapshot struct {
	Updated       time.Time `json:"updated"`
	Source        string    `json:"source"`
	Theme         string    `json:"theme,omitempty"`
	Station       Station   `json:"station"`
	SelectedNORAD int       `json:"selectedNorad"`
	Satellites    []State   `json:"satellites"`
}

type Tracker struct {
	mu              sync.RWMutex
	satellites      []Satellite
	transmitters    map[int][]Signal
	station         Station
	selected        int
	source          string
	cachePath       string
	transmitterPath string
}

var groups = []struct{ Name, Query string }{
	{"Space stations", "stations"}, {"Amateur radio", "amateur"},
	{"CubeSats", "cubesat"}, {"Weather", "weather"},
	{"GPS", "gps-ops"}, {"Galileo", "galileo"}, {"GLONASS", "glo-ops"}, {"BeiDou", "beidou"},
	{"Iridium NEXT", "iridium-NEXT"}, {"Orbcomm", "orbcomm"}, {"Starlink", "starlink"},
}

func NewTracker(cachePath string) *Tracker {
	t := &Tracker{
		cachePath:       cachePath,
		transmitterPath: filepath.Join(filepath.Dir(cachePath), "satellites-transmitters.json"),
		transmitters:    builtinTransmitters(),
		station:         DefaultStation(""),
		selected:        25544,
		source:          "built-in catalog",
	}
	_ = t.loadTransmitters()
	t.satellites = fallbackCatalog()
	_ = t.loadCache()
	t.attachSignalsLocked()
	return t
}

func (t *Tracker) Station() Station     { t.mu.RLock(); defer t.mu.RUnlock(); return t.station }
func (t *Tracker) SetStation(s Station) { t.mu.Lock(); t.station = s; t.mu.Unlock() }
func (t *Tracker) Select(norad int)     { t.mu.Lock(); t.selected = norad; t.mu.Unlock() }
func (t *Tracker) Selected() int        { t.mu.RLock(); defer t.mu.RUnlock(); return t.selected }
func (t *Tracker) Satellites() []Satellite {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return append([]Satellite(nil), t.satellites...)
}

func (t *Tracker) Refresh(ctx context.Context) error {
	client := &http.Client{Timeout: 30 * time.Second}
	seen := map[int]Satellite{}
	// Keep the two promised anchor objects available even if a remote group is
	// temporarily incomplete. Fresh TLEs replace these entries when received.
	for _, sat := range fallbackCatalog() {
		seen[sat.NORAD] = sat
	}
	var failures int
	remoteCount := 0
	for _, g := range groups {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://celestrak.org/NORAD/elements/gp.php?GROUP="+g.Query+"&FORMAT=TLE", nil)
		resp, err := client.Do(req)
		if err != nil {
			failures++
			continue
		}
		list, parseErr := parseTLE(resp.Body, g.Name)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK || parseErr != nil {
			failures++
			continue
		}
		for _, sat := range list {
			remoteCount++
			if old, ok := seen[sat.NORAD]; !ok || priority(sat.Group) < priority(old.Group) {
				seen[sat.NORAD] = sat
			}
		}
	}
	if remoteCount == 0 {
		return fmt.Errorf("could not update orbital catalog (%d groups failed)", failures)
	}
	var remoteTransmitters map[int][]Signal
	if table, err := fetchSatnogsTransmitters(client); err == nil {
		remoteTransmitters = table
	}
	list := make([]Satellite, 0, len(seen))
	for _, sat := range seen {
		list = append(list, sat)
	}
	sort.Slice(list, func(i, j int) bool {
		if priority(list[i].Group) == priority(list[j].Group) {
			return list[i].Name < list[j].Name
		}
		return priority(list[i].Group) < priority(list[j].Group)
	})
	t.mu.Lock()
	if remoteTransmitters != nil {
		t.transmitters = mergeTransmitterTables(builtinTransmitters(), remoteTransmitters)
	}
	t.satellites = list
	t.attachSignalsLocked()
	t.source = "CelesTrak · SatNOGS · " + time.Now().Format("02 Jan 15:04")
	t.mu.Unlock()
	if remoteTransmitters != nil {
		_ = t.saveTransmitters()
	}
	return t.saveCache()
}

func priority(group string) int {
	for i, g := range groups {
		if g.Name == group {
			return i
		}
	}
	return len(groups)
}

func (t *Tracker) Snapshot(at time.Time) Snapshot {
	t.mu.RLock()
	sats := append([]Satellite(nil), t.satellites...)
	station, selected, source := t.station, t.selected, t.source
	t.mu.RUnlock()
	states := make([]State, 0, len(sats))
	for _, sat := range sats {
		lat, lon, alt := position(sat.Elements, at)
		az, el, rng := lookAngles(station, lat, lon, alt)
		sig := Signal{Name: "Catalogued signal", Mode: "--"}
		if len(sat.Signals) > 0 {
			sig = sat.Signals[0]
		}
		state := State{Name: sat.Name, Group: sat.Group, NORAD: sat.NORAD, Signal: sig.Name, Mode: sig.Mode, DownlinkHz: sig.DownlinkHz, Latitude: lat, Longitude: lon, AltitudeKM: alt, Azimuth: az, Elevation: el, RangeKM: rng, Visible: el >= 0}
		if sat.NORAD == selected {
			for m := -45; m <= 90; m += 5 {
				la, lo, _ := position(sat.Elements, at.Add(time.Duration(m)*time.Minute))
				state.Trajectory = append(state.Trajectory, Point{la, lo})
			}
			state.NextPass = predictPass(sat.Elements, station, at)
		}
		states = append(states, state)
	}
	return Snapshot{Updated: at, Source: source, Station: station, SelectedNORAD: selected, Satellites: states}
}

// predictPass finds the next useful closest approach above the observer's
// horizon. A currently setting pass is skipped because its TCA has elapsed.
func predictPass(elements Elements, station Station, now time.Time) PassPrediction {
	const step = 30 * time.Second
	end := now.Add(48 * time.Hour)
	look := func(at time.Time) (float64, float64) {
		lat, lon, alt := position(elements, at)
		_, elevation, distance := lookAngles(station, lat, lon, alt)
		return elevation, distance
	}

	elevation, _ := look(now)
	inProgress := elevation >= 0
	if inProgress {
		nextElevation, _ := look(now.Add(step))
		if nextElevation < elevation {
			wentBelow := false
			for at := now.Add(step); at.Before(end); at = at.Add(step) {
				elevation, _ = look(at)
				if elevation < 0 {
					now = at
					inProgress = false
					wentBelow = true
					break
				}
			}
			if !wentBelow {
				return PassPrediction{Found: true, Continuous: true, InProgress: true}
			}
		} else {
			aos := now
			for at := now.Add(-step); at.After(now.Add(-12 * time.Hour)); at = at.Add(-step) {
				previous, _ := look(at)
				if previous < 0 {
					aos = refineHorizon(at, at.Add(step), look)
					break
				}
				aos = at
			}
			if now.Sub(aos) >= 12*time.Hour-step {
				return PassPrediction{Found: true, Continuous: true, InProgress: true}
			}
			return finishPass(aos, now, end, true, look)
		}
	}

	previousTime := now
	previousElevation, _ := look(previousTime)
	for at := now.Add(step); !at.After(end); at = at.Add(step) {
		currentElevation, _ := look(at)
		if previousElevation < 0 && currentElevation >= 0 {
			aos := refineHorizon(previousTime, at, look)
			return finishPass(aos, aos, end, false, look)
		}
		previousTime, previousElevation = at, currentElevation
	}
	if math.Abs(elements.MeanMotion-1) < .1 {
		return PassPrediction{Found: true, Continuous: true}
	}
	return PassPrediction{}
}

func finishPass(aos, scanStart, end time.Time, inProgress bool, look func(time.Time) (float64, float64)) PassPrediction {
	const step = 30 * time.Second
	bestTime := scanStart
	bestElevation, bestRange := look(scanStart)
	maxElevation := bestElevation
	previousTime := scanStart
	for at := scanStart.Add(step); !at.After(end); at = at.Add(step) {
		elevation, distance := look(at)
		if distance < bestRange {
			bestRange, bestTime = distance, at
		}
		maxElevation = math.Max(maxElevation, elevation)
		if elevation < 0 {
			los := refineHorizon(previousTime, at, look)
			return PassPrediction{Found: true, InProgress: inProgress, AOS: aos, TCA: bestTime, LOS: los, MinRangeKM: bestRange, MaxElevation: maxElevation}
		}
		previousTime = at
	}
	return PassPrediction{Found: true, Continuous: true, InProgress: inProgress}
}

func refineHorizon(low, high time.Time, look func(time.Time) (float64, float64)) time.Time {
	lowElevation, _ := look(low)
	for i := 0; i < 16; i++ {
		mid := low.Add(high.Sub(low) / 2)
		midElevation, _ := look(mid)
		if (lowElevation < 0) == (midElevation < 0) {
			low, lowElevation = mid, midElevation
		} else {
			high = mid
		}
	}
	return low.Add(high.Sub(low) / 2)
}

func parseTLE(r interface{ Read([]byte) (int, error) }, group string) ([]Satellite, error) {
	s := bufio.NewScanner(r)
	var lines []string
	for s.Scan() {
		v := strings.TrimSpace(s.Text())
		if v != "" {
			lines = append(lines, v)
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	var out []Satellite
	for i := 0; i+2 < len(lines); i++ {
		if strings.HasPrefix(lines[i+1], "1 ") && strings.HasPrefix(lines[i+2], "2 ") {
			sat, err := makeSatellite(lines[i], lines[i+1], lines[i+2], group)
			if err == nil {
				out = append(out, sat)
			}
			i += 2
		}
	}
	if len(out) == 0 {
		return nil, errors.New("response has no TLE")
	}
	return out, nil
}

func makeSatellite(name, l1, l2, group string) (Satellite, error) {
	if len(l1) < 32 || len(l2) < 63 {
		return Satellite{}, errors.New("incomplete TLE")
	}
	norad, _ := strconv.Atoi(strings.TrimSpace(l1[2:7]))
	year, _ := strconv.Atoi(l1[18:20])
	day, _ := strconv.ParseFloat(strings.TrimSpace(l1[20:32]), 64)
	if year < 57 {
		year += 2000
	} else {
		year += 1900
	}
	epoch := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration((day - 1) * float64(24*time.Hour)))
	f := strings.Fields(l2)
	if len(f) < 8 {
		return Satellite{}, errors.New("invalid line 2")
	}
	inc, _ := strconv.ParseFloat(f[2], 64)
	raan, _ := strconv.ParseFloat(f[3], 64)
	ecc, _ := strconv.ParseFloat("0."+f[4], 64)
	arg, _ := strconv.ParseFloat(f[5], 64)
	ma, _ := strconv.ParseFloat(f[6], 64)
	mm, _ := strconv.ParseFloat(f[7], 64)
	return Satellite{Name: strings.TrimSpace(name), Group: group, NORAD: norad, Line1: l1, Line2: l2, Elements: Elements{epoch, inc, raan, ecc, arg, ma, mm}}, nil
}

func position(e Elements, at time.Time) (lat, lon, alt float64) {
	const mu = 398600.4418
	n := e.MeanMotion * 2 * math.Pi / 86400
	if n <= 0 {
		return
	}
	a := math.Cbrt(mu / (n * n))
	M := radians(e.MeanAnomaly) + n*at.Sub(e.Epoch).Seconds()
	E := M
	for i := 0; i < 8; i++ {
		E = M + e.Eccentricity*math.Sin(E)
	}
	x := a * (math.Cos(E) - e.Eccentricity)
	y := a * math.Sqrt(1-e.Eccentricity*e.Eccentricity) * math.Sin(E)
	r := math.Hypot(x, y)
	nu := math.Atan2(y, x)
	u := nu + radians(e.ArgumentPerigee)
	inc, raan := radians(e.Inclination), radians(e.RAAN)
	xi := r * (math.Cos(raan)*math.Cos(u) - math.Sin(raan)*math.Sin(u)*math.Cos(inc))
	yi := r * (math.Sin(raan)*math.Cos(u) + math.Cos(raan)*math.Sin(u)*math.Cos(inc))
	zi := r * math.Sin(u) * math.Sin(inc)
	theta := gmst(at)
	xe := math.Cos(theta)*xi + math.Sin(theta)*yi
	ye := -math.Sin(theta)*xi + math.Cos(theta)*yi
	lon = degrees(math.Atan2(ye, xe))
	lat = degrees(math.Atan2(zi, math.Hypot(xe, ye)))
	alt = r - 6371.0
	return
}

func lookAngles(s Station, lat, lon, alt float64) (az, el, rng float64) {
	re := 6371.0
	slat, slon := radians(s.Latitude), radians(s.Longitude)
	r1 := re + s.AltitudeMeters/1000
	r2 := re + alt
	sx, sy, sz := r1*math.Cos(slat)*math.Cos(slon), r1*math.Cos(slat)*math.Sin(slon), r1*math.Sin(slat)
	la, lo := radians(lat), radians(lon)
	x, y, z := r2*math.Cos(la)*math.Cos(lo), r2*math.Cos(la)*math.Sin(lo), r2*math.Sin(la)
	dx, dy, dz := x-sx, y-sy, z-sz
	east := -math.Sin(slon)*dx + math.Cos(slon)*dy
	north := -math.Sin(slat)*math.Cos(slon)*dx - math.Sin(slat)*math.Sin(slon)*dy + math.Cos(slat)*dz
	up := math.Cos(slat)*math.Cos(slon)*dx + math.Cos(slat)*math.Sin(slon)*dy + math.Sin(slat)*dz
	rng = math.Sqrt(east*east + north*north + up*up)
	az = math.Mod(degrees(math.Atan2(east, north))+360, 360)
	el = degrees(math.Asin(up / rng))
	return
}

func gmst(t time.Time) float64 {
	jd := float64(t.UnixNano())/86400e9 + 2440587.5
	d := jd - 2451545.0
	return radians(math.Mod(280.46061837+360.98564736629*d, 360))
}
func radians(v float64) float64 { return v * math.Pi / 180 }
func degrees(v float64) float64 { return v * 180 / math.Pi }

type cachedCatalog struct {
	Saved      time.Time   `json:"saved"`
	Satellites []Satellite `json:"satellites"`
}

func (t *Tracker) saveCache() error {
	t.mu.RLock()
	c := cachedCatalog{time.Now(), append([]Satellite(nil), t.satellites...)}
	t.mu.RUnlock()
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(t.cachePath), 0755); err != nil {
		return err
	}
	return os.WriteFile(t.cachePath, data, 0644)
}
func (t *Tracker) loadCache() error {
	data, err := os.ReadFile(t.cachePath)
	if err != nil {
		return err
	}
	var c cachedCatalog
	if json.Unmarshal(data, &c) != nil || len(c.Satellites) == 0 {
		return errors.New("invalid cache")
	}
	t.satellites = c.Satellites
	t.source = "CelesTrak cache · " + c.Saved.Format("02 Jan 15:04")
	return nil
}

func applyKnownSignals(s *Satellite) {
	s.Signals = mergeSignals(knownSignals(s.NORAD), s.Signals)
}
func fallbackCatalog() []Satellite {
	raw := [][4]string{{"ISS (ZARYA)", "Space stations", "1 25544U 98067A   25250.50000000  .00012000  00000-0  22000-3 0  9991", "2 25544  51.6340 150.0000 0004000 100.0000 260.0000 15.50000000123456"}, {"QO-100 (ES'HAIL 2)", "Amateur radio", "1 43700U 18090A   25250.50000000  .00000010  00000-0  00000-0 0  9991", "2 43700   0.0150  85.0000 0001800 270.0000  90.0000  1.00270000 25000"}}
	out := make([]Satellite, 0, len(raw))
	for _, v := range raw {
		sat, _ := makeSatellite(v[0], v[2], v[3], v[1])
		applyKnownSignals(&sat)
		out = append(out, sat)
	}
	return out
}
