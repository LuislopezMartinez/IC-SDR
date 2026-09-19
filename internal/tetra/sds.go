package tetra

import (
	"go-zero/internal/i18n"

	"fmt"
	"math"
	"strings"
	"time"
)

// parseSDS decodes the application payload carried by a CMCE D-SDS-DATA PDU.
// It currently covers the commonly observed simple text and short LIP formats.
func parseSDS(bits []byte, ssi uint32, now time.Time) (Message, *Position, bool) {
	if len(bits) < 8 {
		return Message{}, nil, false
	}
	protocol := uint8(bitsToUint(bits, 0, 8))
	payload := bits[8:]
	message := Message{Time: now, PartySSI: ssi, SDS: true, SDSProtocol: protocol, ProtocolName: sdsProtocolName(protocol), RawHex: bitsToHex(bits), RawBits: len(bits)}
	application := protocol
	if protocol >= 128 {
		var ok bool
		payload, message.Fields, ok = sdsTransport(payload)
		if !ok {
			return message, nil, false
		}
		if protocol == 130 {
			application = 2
		}
		if protocol == 137 {
			application = 9
		}
		if protocol == 131 {
			application = 3
		}
	}
	switch application {
	case 3:
		text, position, ok := parseLocationText(payload, ssi, now)
		if !ok {
			return Message{}, nil, false
		}
		message.Kind = "SDS LOCATION TEXT"
		message.Text = text
		message.Recognized = true
		return message, position, true

	case 2, 9: // simple text / simple immediate text
		text, ok := parseSDSText(payload)
		if !ok {
			return Message{}, nil, false
		}
		message.Kind, message.Text, message.Recognized = i18n.Source("text.f1e94935313e"), text, true
		return message, nil, true
	case 10: // Location Information Protocol
		position, ok := parseShortLIP(payload, ssi, now)
		if !ok {
			if fields, text, valid := lipControl(payload); valid {
				message.Fields = fields
				message.Kind = "LIP CONTROL"
				message.Text = text
				message.Recognized = true
				return message, nil, true
			}
			return Message{}, nil, false
		}
		speed, heading, accuracy := "—", "—", "—"
		if position.HasVelocity {
			speed = fmt.Sprintf("%.1f", position.SpeedKmh)
		}
		if position.HasHeading {
			heading = fmt.Sprintf("%.1f", position.Heading)
		}
		if position.AccuracyKnown {
			accuracy = fmt.Sprintf("~%.0f", position.AccuracyM)
		}
		text := fmt.Sprintf("%.6f, %.6f · %s km/h · %s° · %s m", position.Latitude, position.Longitude, speed, heading, accuracy)
		message.Kind, message.Text, message.Recognized = i18n.Source("text.d1647141cb62"), text, true
		return message, &position, true
	}
	return Message{}, nil, false
}

func sdsProtocolName(protocol uint8) string {
	switch protocol {
	case 3:
		return "Simple location system"
	case 130:
		return "Text messaging TL"
	case 131:
		return "Location system TL"
	case 137:
		return "Immediate text TL"
	case 2:
		return i18n.Source("text.6c359eec4e84")
	case 9:
		return i18n.Source("text.c25f8e9628b6")
	case 10:
		return i18n.Source("text.8ccbbd4a7262")
	default:
		return fmt.Sprintf(i18n.Source("text.1e5cd4a3ec58"), protocol)
	}
}

func bitsToHex(bits []byte) string {
	if len(bits) == 0 {
		return ""
	}
	const digits = "0123456789ABCDEF"
	out := make([]byte, (len(bits)+3)/4)
	for i := range out {
		var nibble byte
		for bit := 0; bit < 4; bit++ {
			index := i*4 + bit
			nibble <<= 1
			if index < len(bits) {
				nibble |= bits[index] & 1
			}
		}
		out[i] = digits[nibble]
	}
	return string(out)
}

func parseSDSText(bits []byte) (string, bool) {
	if len(bits) < 8 {
		return "", false
	}
	coding := bitsToUint(bits, 1, 7)
	if coding != 0 && coding != 1 {
		return "", false
	}
	offset := 8 // timestamp-used + coding scheme
	if bits[0] != 0 {
		offset += 24
	}
	if offset > len(bits) || len(bits)-offset < 8 {
		return "", false
	}
	var out strings.Builder
	for ; offset+8 <= len(bits); offset += 8 {
		b := byte(bitsToUint(bits, offset, 8))
		if b == 0 {
			break
		}
		if b >= 32 && b != 127 {
			out.WriteRune(rune(b)) // ISO-8859-1 is common on legacy TETRA SDS.
		} else if b == '\n' || b == '\r' || b == '\t' {
			out.WriteByte(' ')
		}
	}
	text := strings.TrimSpace(out.String())
	return text, text != ""
}

func parseShortLIP(bits []byte, ssi uint32, now time.Time) (Position, bool) {
	// PDU type 0: short location report, ETSI TS 100 392-18-1.
	if len(bits) < 67 || bitsToUint(bits, 0, 2) != 0 {
		return Position{}, false
	}
	age := uint8(bitsToUint(bits, 2, 2))
	lonRaw := bitsToUint(bits, 4, 25)
	latRaw := bitsToUint(bits, 29, 24)
	errorCode := bitsToUint(bits, 53, 3)
	velocityCode := bitsToUint(bits, 56, 7)
	directionCode := bitsToUint(bits, 63, 4)
	longitude := float64(lonRaw) * 360.0 / float64(uint64(1)<<25)
	latitude := float64(latRaw) * 180.0 / float64(uint64(1)<<24)
	if longitude >= 180 {
		longitude -= 360
	}
	if latitude >= 90 {
		latitude -= 180
	}
	velocity := float64(velocityCode)
	if velocityCode > 28 {
		velocity = 16 * math.Pow(1.038, float64(velocityCode)-13)
	}
	return Position{Protocol: "LIP", HasVelocity: velocityCode < 127, HasHeading: directionCode < 15, AccuracyKnown: errorCode < 7, SSI: ssi, Latitude: latitude, Longitude: longitude, SpeedKmh: velocity, Heading: float64(directionCode) * 22.5, AccuracyM: 10 * math.Pow(2, float64(errorCode)), AgeCode: age, Time: now}, true
}
