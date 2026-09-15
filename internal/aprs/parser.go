package aprs

import (
	"go-zero/internal/i18n"

	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Packet struct {
	Received      time.Time `json:"received"`
	Source        string    `json:"source"`
	Destination   string    `json:"destination"`
	Path          string    `json:"path"`
	Information   string    `json:"information"`
	Raw           string    `json:"raw"`
	Type          string    `json:"type"`
	Symbol        string    `json:"symbol"`
	Coordinates   string    `json:"coordinates"`
	Locator       string    `json:"locator"`
	Summary       string    `json:"summary"`
	Course        string    `json:"course"`
	Speed         string    `json:"speed"`
	Altitude      string    `json:"altitude"`
	MessageTarget string    `json:"messageTarget"`
	MessageID     string    `json:"messageId"`
	Temperature   string    `json:"temperature"`
	Humidity      string    `json:"humidity"`
	Pressure      string    `json:"pressure"`
	Wind          string    `json:"wind"`
	Rain          string    `json:"rain"`
	Telemetry     string    `json:"telemetry"`
	PHG           string    `json:"phg"`
	Latitude      float64   `json:"latitude,omitempty"`
	Longitude     float64   `json:"longitude,omitempty"`
	ReceiveLevel  int       `json:"receiveLevel"`
}

func DecodeAX25(frame []byte, level int) (Packet, bool) {
	addresses := []string{}
	offset := 0
	for offset+7 <= len(frame) && len(addresses) < 10 {
		addresses = append(addresses, decodeAddress(frame[offset:offset+7], len(addresses) >= 2))
		last := frame[offset+6]&1 != 0
		offset += 7
		if last {
			break
		}
	}
	if len(addresses) < 2 || offset+2 > len(frame) || frame[offset] != 3 || frame[offset+1] != 0xf0 {
		return Packet{}, false
	}
	offset += 2
	end := len(frame)
	for end > offset && (frame[end-1] == 0 || frame[end-1] == '\r' || frame[end-1] == '\n') {
		end--
	}
	info := string(frame[offset:end])
	path := ""
	if len(addresses) > 2 {
		path = strings.Join(addresses[2:], ",")
	}
	raw := addresses[1] + ">" + addresses[0]
	if path != "" {
		raw += "," + path
	}
	raw += ":" + info
	p := Packet{Received: time.Now(), Source: addresses[1], Destination: addresses[0], Path: path, Information: info, Raw: raw, Type: i18n.Source("text.1c55d9b826e8"), Symbol: "—", Coordinates: "—", Locator: "—", Summary: clean(info), Course: "—", Speed: "—", Altitude: "—", MessageTarget: "—", MessageID: "—", Temperature: "—", Humidity: "—", Pressure: "—", Wind: "—", Rain: "—", Telemetry: "—", PHG: "—", ReceiveLevel: level}
	parseAPRS(&p)
	return p, true
}
func decodeAddress(b []byte, digi bool) string {
	var s strings.Builder
	for i := 0; i < 6; i++ {
		c := b[i] >> 1
		if c != ' ' {
			s.WriteByte(c)
		}
	}
	ssid := (b[6] >> 1) & 15
	if ssid > 0 {
		fmt.Fprintf(&s, "-%d", ssid)
	}
	if digi && b[6]&0x80 != 0 {
		s.WriteByte('*')
	}
	return s.String()
}
func parseAPRS(p *Packet) {
	d := p.Information
	if d == "" {
		return
	}
	kind := d[0]
	offset := -1
	switch kind {
	case '!', '=':
		p.Type = i18n.Source("text.aad6faa6418d")
		offset = 1
	case '/', '@':
		p.Type = i18n.Source("text.aad6faa6418d")
		if len(d) >= 8 {
			offset = 8
		}
	case ':':
		parseMessage(p, d)
		return
	case '>':
		p.Type = i18n.Source("text.8c2e4a035f5f")
		p.Summary = clean(d[1:])
		return
	case '_':
		p.Type = i18n.Source("text.0a244108837b")
		parseWeather(p, d)
		return
	case ';':
		p.Type = i18n.Source("text.afdc96fa07e7")
		if len(d) >= 18 {
			offset = 18
		}
	case ')':
		p.Type = i18n.Source("text.8550f3405394")
		if i := strings.IndexAny(d, "!_"); i >= 0 {
			offset = i + 1
		}
	case '`', '\'':
		p.Type = i18n.Source("text.005ebc34fe5e")
		p.Summary = i18n.Source("text.db63f329de5f") + clean(d[1:])
		return
	default:
		if strings.HasPrefix(d, "T#") {
			p.Type = i18n.Source("text.deb116486d9f")
			p.Telemetry = clean(d)
			p.Summary = p.Telemetry
			return
		}
	}
	if offset >= 0 && parsePosition(p, d, offset) {
		return
	}
	p.Summary = clean(d)
}
func parsePosition(p *Packet, d string, o int) bool {
	if o+19 > len(d) {
		return false
	}
	lat, lon := d[o:o+8], d[o+9:o+18]
	ns, ew := d[o+7], d[o+17]
	if (ns != 'N' && ns != 'S') || (ew != 'E' && ew != 'W') {
		return false
	}
	latD, e1 := strconv.ParseFloat(lat[:2], 64)
	latM, e2 := strconv.ParseFloat(strings.ReplaceAll(lat[2:7], " ", "0"), 64)
	lonD, e3 := strconv.ParseFloat(lon[:3], 64)
	lonM, e4 := strconv.ParseFloat(strings.ReplaceAll(lon[3:8], " ", "0"), 64)
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
		return false
	}
	p.Latitude = latD + latM/60
	p.Longitude = lonD + lonM/60
	if ns == 'S' {
		p.Latitude = -p.Latitude
	}
	if ew == 'W' {
		p.Longitude = -p.Longitude
	}
	p.Coordinates = fmt.Sprintf("%.5f, %.5f", p.Latitude, p.Longitude)
	p.Locator = maidenhead(p.Latitude, p.Longitude)
	p.Symbol = string([]byte{d[o+8], d[o+18]})
	comment := ""
	if o+19 < len(d) {
		comment = clean(d[o+19:])
	}
	parseExtras(p, comment)
	parseWeather(p, comment)
	if comment != "" {
		p.Summary = comment
	} else {
		p.Summary = p.Coordinates
	}
	return true
}
func parseMessage(p *Packet, d string) {
	p.Type = i18n.Source("text.b194d92018d6")
	if len(d) < 11 {
		p.Summary = clean(d[1:])
		return
	}
	p.MessageTarget = strings.TrimSpace(d[1:min(10, len(d))])
	message := ""
	if len(d) > 11 {
		message = d[11:]
	}
	if i := strings.LastIndex(message, "{"); i >= 0 && i+1 < len(message) {
		p.MessageID = strings.TrimSpace(message[i+1:])
		message = message[:i]
	}
	if strings.HasPrefix(message, "ack") {
		p.Type = i18n.Source("text.a2f1a6d79bfb")
	} else if strings.HasPrefix(message, "rej") {
		p.Type = i18n.Source("text.0a8e61fbc428")
	}
	p.Summary = "→ " + p.MessageTarget + "  " + clean(message)
}

var movementRE = regexp.MustCompile(`(?:^|[^0-9])(\d{3})/(\d{3})(?:[^0-9]|$)`)
var altitudeRE = regexp.MustCompile(`/A=(\d{6})`)
var phgRE = regexp.MustCompile(i18n.Source("text.6e6e9d521e89"))
var tempRE = regexp.MustCompile(`t(-?\d{3})`)
var humidRE = regexp.MustCompile(`h(\d{2})`)
var pressureRE = regexp.MustCompile(`b(\d{5})`)
var rainRE = regexp.MustCompile(`r(\d{3})`)

func parseExtras(p *Packet, s string) {
	if m := movementRE.FindStringSubmatch(s); m != nil {
		knots, _ := strconv.Atoi(m[2])
		p.Course = m[1] + "°"
		p.Speed = fmt.Sprintf(i18n.Source("text.dba783d6751c"), int(math.Round(float64(knots)*1.852)))
	}
	if m := altitudeRE.FindStringSubmatch(s); m != nil {
		feet, _ := strconv.Atoi(m[1])
		p.Altitude = fmt.Sprintf("%d m", int(math.Round(float64(feet)*.3048)))
	}
	if m := phgRE.FindString(s); m != "" {
		p.PHG = m
	}
}
func parseWeather(p *Packet, s string) {
	if m := movementRE.FindStringSubmatch(s); m != nil {
		knots, _ := strconv.Atoi(m[2])
		p.Wind = fmt.Sprintf(i18n.Source("text.ba050b56d77f"), m[1], int(math.Round(float64(knots)*1.852)))
	}
	if m := tempRE.FindStringSubmatch(s); m != nil {
		f, _ := strconv.Atoi(m[1])
		p.Temperature = fmt.Sprintf("%.1f °C", float64(f-32)*5/9)
	}
	if m := humidRE.FindStringSubmatch(s); m != nil {
		if m[1] == "00" {
			m[1] = "100"
		}
		p.Humidity = m[1] + " %"
	}
	if m := pressureRE.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		p.Pressure = fmt.Sprintf(i18n.Source("text.efe95347f7ba"), float64(n)/10)
	}
	if m := rainRE.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		p.Rain = fmt.Sprintf(i18n.Source("text.fb023bf80ffb"), float64(n)*.254)
	}
	if p.Temperature != "—" || p.Humidity != "—" || p.Pressure != "—" {
		p.Type = i18n.Source("text.0a244108837b")
	}
}
func maidenhead(lat, lon float64) string {
	lon += 180
	lat += 90
	a, b := int(lon/20), int(lat/10)
	lon -= float64(a * 20)
	lat -= float64(b * 10)
	c, d := int(lon/2), int(lat)
	lon -= float64(c * 2)
	lat -= float64(d)
	e, f := int(lon*12), int(lat*24)
	return fmt.Sprintf("%c%c%d%d%c%c", 'A'+a, 'A'+b, c, d, 'a'+e, 'a'+f)
}
func clean(s string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if r < ' ' && r != '\t' {
			return -1
		}
		return r
	}, s))
}
