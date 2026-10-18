package datv

import "fmt"

type transportFramer struct {
	buffer []byte
	synced bool
}

func (f *transportFramer) push(data []byte) []byte {
	f.buffer = append(f.buffer, data...)
	if !f.synced {
		if ok, offset := transportSync(f.buffer); ok {
			f.buffer = f.buffer[offset:]
			f.synced = true
		} else {
			// Keep enough tail to detect three sync bytes split across reads.
			if len(f.buffer) > 188*3 {
				f.buffer = append(f.buffer[:0], f.buffer[len(f.buffer)-(188*3-1):]...)
			}
			return nil
		}
	}
	complete := len(f.buffer) / 188 * 188
	if complete == 0 {
		return nil
	}
	out := append([]byte(nil), f.buffer[:complete]...)
	f.buffer = append(f.buffer[:0], f.buffer[complete:]...)
	// A corrupt stream can lose framing. Re-acquire rather than forwarding
	// malformed datagrams indefinitely.
	for i := 0; i < len(out); i += 188 {
		if out[i] != 0x47 {
			f.synced = false
			f.buffer = append(out[i+1:], f.buffer...)
			return out[:i]
		}
	}
	return out
}

type transportAnalyzer struct {
	pmtPID uint16
	video  string
	audio  string
}

func (a *transportAnalyzer) consume(data []byte) (video, audio string) {
	for len(data) >= 188 {
		packet := data[:188]
		data = data[188:]
		if packet[0] != 0x47 || packet[1]&0x80 != 0 {
			continue
		}
		pid := uint16(packet[1]&0x1f)<<8 | uint16(packet[2])
		payloadStart := packet[1]&0x40 != 0
		control := (packet[3] >> 4) & 3
		at := 4
		if control == 0 || control == 2 {
			continue
		}
		if control == 3 {
			if at >= len(packet) {
				continue
			}
			at += 1 + int(packet[at])
		}
		if at >= len(packet) || !payloadStart {
			continue
		}
		at += 1 + int(packet[at]) // pointer field
		if at+3 > len(packet) {
			continue
		}
		section := packet[at:]
		switch {
		case pid == 0 && section[0] == 0x00:
			a.parsePAT(section)
		case a.pmtPID != 0 && pid == a.pmtPID && section[0] == 0x02:
			a.parsePMT(section)
		}
	}
	return a.video, a.audio
}

func (a *transportAnalyzer) parsePAT(section []byte) {
	length := sectionLength(section)
	if length < 12 || length > len(section) {
		return
	}
	for at := 8; at+4 <= length-4; at += 4 {
		program := uint16(section[at])<<8 | uint16(section[at+1])
		if program != 0 {
			a.pmtPID = uint16(section[at+2]&0x1f)<<8 | uint16(section[at+3])
			return
		}
	}
}

func (a *transportAnalyzer) parsePMT(section []byte) {
	length := sectionLength(section)
	if length < 16 || length > len(section) {
		return
	}
	at := 12 + int(section[10]&0x0f)<<8 + int(section[11])
	for at+5 <= length-4 {
		streamType := section[at]
		descriptorLength := int(section[at+3]&0x0f)<<8 | int(section[at+4])
		name, kind := streamTypeName(streamType)
		if kind == "video" && a.video == "" {
			a.video = name
		} else if kind == "audio" && a.audio == "" {
			a.audio = name
		}
		at += 5 + descriptorLength
	}
}

func sectionLength(section []byte) int {
	if len(section) < 3 {
		return 0
	}
	return 3 + int(section[1]&0x0f)<<8 + int(section[2])
}

func streamTypeName(streamType byte) (string, string) {
	switch streamType {
	case 0x01:
		return "MPEG-1 Video", "video"
	case 0x02:
		return "MPEG-2 Video", "video"
	case 0x1b:
		return "H.264/AVC", "video"
	case 0x24:
		return "H.265/HEVC", "video"
	case 0x03:
		return "MPEG-1 Audio", "audio"
	case 0x04:
		return "MPEG-2 Audio", "audio"
	case 0x0f:
		return "AAC", "audio"
	case 0x11:
		return "AAC LATM", "audio"
	case 0x81:
		return "AC-3", "audio"
	default:
		return fmt.Sprintf("tipo 0x%02X", streamType), "other"
	}
}
