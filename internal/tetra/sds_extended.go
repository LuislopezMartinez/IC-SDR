package tetra

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

func sdsTransport(payload []byte) ([]byte, map[string]uint32, bool) {
	if len(payload) < 4 {
		return nil, nil, false
	}
	typ := bitsToUint(payload, 0, 4)
	if typ != 0 && typ != 1 {
		return nil, map[string]uint32{"SDS_TL_MessageType": typ}, false
	}
	rules := forwardRules
	if typ == 1 {
		rules = reportRules
	}
	fields, off, ok := parseFields(payload, 4, rules)
	fields["SDS_TL_MessageType"] = typ
	return payload[off:], fields, ok && typ == 0
}
func parseLocationText(bits []byte, ssi uint32, now time.Time) (string, *Position, bool) {
	if len(bits) < 8 || bitsToUint(bits, 0, 8) != 0 {
		return "", nil, false
	}
	var out strings.Builder
	for i := 8; i+8 <= len(bits); i += 8 {
		b := byte(bitsToUint(bits, i, 8))
		if b != 0 {
			out.WriteByte(b)
		}
	}
	text := strings.TrimSpace(out.String())
	if text == "" {
		return "", nil, false
	}
	return text, parseNMEAPosition(text, ssi, now), true
}
func parseNMEAPosition(text string, ssi uint32, now time.Time) *Position {
	for _, line := range strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' }) {
		line = strings.TrimSpace(line)
		star := strings.IndexByte(line, '*')
		if star >= 0 {
			if len(line) < star+3 || len(line) == 0 || line[0] != '$' {
				continue
			}
			sum := byte(0)
			for i := 1; i < star; i++ {
				sum ^= line[i]
			}
			want, err := strconv.ParseUint(line[star+1:star+3], 16, 8)
			if err != nil || sum != byte(want) {
				continue
			}
			line = line[:star]
		}
		f := strings.Split(line, ",")
		if len(f) < 7 {
			continue
		}
		p := Position{Protocol: "NMEA", SSI: ssi, Time: now}
		var lat, lon, ns, ew string
		switch {
		case strings.HasSuffix(f[0], "RMC"):
			if len(f) < 9 || f[2] != "A" {
				continue
			}
			lat, ns, lon, ew = f[3], f[4], f[5], f[6]
			if speed, err := strconv.ParseFloat(f[7], 64); err == nil && !math.IsNaN(speed) && !math.IsInf(speed, 0) && speed >= 0 {
				p.SpeedKmh = speed * 1.852
				p.HasVelocity = true
			}
			heading, headingErr := strconv.ParseFloat(f[8], 64)
			p.Heading = heading
			p.HasHeading = headingErr == nil && heading >= 0 && heading < 360 && !math.IsNaN(heading) && !math.IsInf(heading, 0)
			if math.IsNaN(p.Heading) || math.IsInf(p.Heading, 0) || p.Heading < 0 || p.Heading >= 360 {
				p.Heading = 0
			}
		case strings.HasSuffix(f[0], "GGA"):
			if f[6] == "0" || f[6] == "" {
				continue
			}
			lat, ns, lon, ew = f[2], f[3], f[4], f[5]
		default:
			continue
		}
		convert := func(value, hem string, limit float64) (float64, bool) {
			v, err := strconv.ParseFloat(value, 64)
			if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
				return 0, false
			}
			deg := math.Floor(v / 100)
			minutes := v - deg*100
			if minutes >= 60 {
				return 0, false
			}
			v = deg + minutes/60
			if v > limit {
				return 0, false
			}
			if hem == "S" || hem == "W" {
				v = -v
			} else if hem != "N" && hem != "E" {
				return 0, false
			}
			return v, true
		}
		if (ns != "N" && ns != "S") || (ew != "E" && ew != "W") {
			continue
		}
		var ok1, ok2 bool
		p.Latitude, ok1 = convert(lat, ns, 90)
		p.Longitude, ok2 = convert(lon, ew, 180)
		if ok1 && ok2 {
			return &p
		}
	}
	return nil
}
func lipControl(bits []byte) (map[string]uint32, string, bool) {
	if len(bits) < 6 || bitsToUint(bits, 0, 2) != 1 {
		return nil, "", false
	}
	sub := bitsToUint(bits, 2, 4)
	fields := map[string]uint32{"Location_PDU_type": 1, "Location_PDU_type_extension": sub}
	return fields, fmt.Sprintf("LIP extended subtype:%d", sub), true
}
