package tetra

import "time"

func scramblingInit(si SystemInfo) uint32 {
	return ((uint32(si.ColourCode&0x3f) | uint32(si.MNC&0x3fff)<<6 | uint32(si.MCC&0x3ff)<<20) << 2) | 3
}

type NetworkInfo struct {
	Valid           bool   `json:"valid"`
	MainCarrier     uint16 `json:"mainCarrier"`
	FrequencyBand   uint8  `json:"frequencyBand"`
	FrequencyOffset uint8  `json:"frequencyOffset"`
	DuplexSpacing   uint8  `json:"duplexSpacing"`
	DownlinkHz      int64  `json:"downlinkHz"`
	UplinkHz        int64  `json:"uplinkHz"`
	LocationArea    uint16 `json:"locationArea"`
	ServiceDetails  uint16 `json:"serviceDetails"`
}

func parseMACSysinfo(bits []byte) (NetworkInfo, bool) {
	if len(bits) < 124 || bitsToUint(bits, 0, 2) != 2 || bitsToUint(bits, 2, 2) != 0 {
		return NetworkInfo{}, false
	}
	n := NetworkInfo{Valid: true}
	n.MainCarrier = uint16(bitsToUint(bits, 4, 12))
	n.FrequencyBand = uint8(bitsToUint(bits, 16, 4))
	n.FrequencyOffset = uint8(bitsToUint(bits, 20, 2))
	n.DuplexSpacing = uint8(bitsToUint(bits, 22, 3))
	reverse := bitsToUint(bits, 25, 1) != 0
	offsets := [4]int64{0, 6250, -6250, 12500}
	n.DownlinkHz = int64(n.FrequencyBand)*100_000_000 + int64(n.MainCarrier)*25_000 + offsets[n.FrequencyOffset&3]
	spacing := [8][16]int64{
		{-1, 1600, 10000, 10000, 10000, 10000, 10000, -1, -1, -1, -1, -1, -1, -1, -1, -1},
		{-1, 4500, -1, 36000, 7000, -1, -1, -1, 45000, 45000, -1, -1, -1, -1, -1, -1},
		{},
		{-1, -1, -1, 8000, 8000, -1, -1, -1, 18000, 18000, -1, -1, -1, -1, -1, -1},
		{-1, -1, -1, 18000, 5000, -1, 30000, 30000, -1, 39000, -1, -1, -1, -1, -1, -1},
		{-1, -1, -1, -1, 9500, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
	}
	gap := spacing[n.DuplexSpacing&7][n.FrequencyBand&15]
	if gap >= 0 {
		n.UplinkHz = n.DownlinkHz - gap*1000
		if reverse {
			n.UplinkHz = n.DownlinkHz + gap*1000
		}
	}
	n.LocationArea = uint16(bitsToUint(bits, 82, 14))
	n.ServiceDetails = uint16(bitsToUint(bits, 112, 12))
	// Real serving-cell SYSINFO announces at least one service and a usable
	// carrier. Zero-filled remainder must never replace a previously decoded cell.
	if n.MainCarrier == 0 || n.FrequencyBand == 0 || n.ServiceDetails == 0 || n.DownlinkHz < 100_000_000 {
		return NetworkInfo{}, false
	}
	return n, true
}

// decodeSCHF decodes the 432 type-5 bits carried by a full-slot SCH/F.
func decodeSCHF(coded []byte, si SystemInfo) ([]byte, bool) {
	payload, ok, _, _ := decodeSCHFMetrics(coded, si)
	return payload, ok
}

func decodeSCHFMetrics(coded []byte, si SystemInfo) ([]byte, bool, int, int) {
	if len(coded) != 432 || !si.Valid {
		return nil, false, 0, 0
	}
	work := descrambleFullSlot(coded, si)
	if work == nil {
		return nil, false, 0, 0
	}
	deint := make([]byte, 432)
	for k := 1; k <= 432; k++ {
		j := 1 + (103*k)%432
		deint[k-1] = work[j-1]
	}
	depunct := make([]byte, 1152)
	src := 0
	keep := [8]bool{true, true, false, false, true, false, false, false}
	for i := range depunct {
		if keep[i&7] {
			depunct[i] = deint[src]
			src++
		} else {
			depunct[i] = erasure
		}
	}
	decoded := viterbi(depunct, 288)
	errors, observed := convolutionErrors(decoded, depunct)
	if crc16Bits(decoded[:284]) != 0x1d0f {
		return nil, false, errors, observed
	}
	return append([]byte(nil), decoded[:268]...), true, errors, observed
}

// decodeSCHH decodes one 216-bit half-slot signalling block carried by NDB2.
func decodeSCHH(coded []byte, si SystemInfo) ([]byte, bool) {
	payload, ok, _, _ := decodeSCHHMetrics(coded, si)
	return payload, ok
}

func decodeSCHHMetrics(coded []byte, si SystemInfo) ([]byte, bool, int, int) {
	if len(coded) != 216 || !si.Valid {
		return nil, false, 0, 0
	}
	work := descrambleBlock(coded, si)
	deint := make([]byte, 216)
	for k := 1; k <= 216; k++ {
		j := 1 + (101*k)%216
		deint[k-1] = work[j-1]
	}
	depunct := make([]byte, 576)
	src := 0
	keep := [8]bool{true, true, false, false, true, false, false, false}
	for i := range depunct {
		if keep[i&7] {
			depunct[i] = deint[src]
			src++
		} else {
			depunct[i] = erasure
		}
	}
	decoded := viterbi(depunct, 144)
	errors, observed := convolutionErrors(decoded, depunct)
	if crc16Bits(decoded[:140]) != 0x1d0f {
		return nil, false, errors, observed
	}
	return append([]byte(nil), decoded[:124]...), true, errors, observed
}

func convolutionErrors(decoded, received []byte) (errors, observed int) {
	poly := [4]byte{0x13, 0x1d, 0x17, 0x1b}
	parity := func(v byte) byte { v ^= v >> 4; v ^= v >> 2; v ^= v >> 1; return v & 1 }
	state := byte(0)
	for _, bit := range decoded {
		sr := (state << 1) | (bit & 1)
		for _, p := range poly {
			if observed >= len(received) {
				return errors, observed
			}
			if received[observed] != erasure && received[observed] != parity(sr&p) {
				errors++
			}
			if received[observed] != erasure { /* count only transmitted punctures */
			}
			observed++
		}
		state = sr & 15
	}
	// observed currently includes punctured erasures; return the actual number
	// of received coded bits so BER has the correct denominator.
	actual := 0
	for _, bit := range received[:observed] {
		if bit != erasure {
			actual++
		}
	}
	return errors, actual
}

func descrambleFullSlot(coded []byte, si SystemInfo) []byte {
	if len(coded) != 432 || !si.Valid {
		return nil
	}
	return descrambleBlock(coded, si)
}

type resourceAddress struct {
	Type              byte
	SSI               uint32
	Encrypted         bool
	UsageMarker       uint8
	HasUsageMarker    bool
	HeaderBits        int
	LengthBits        int
	ChannelAllocation bool
}

func parseMACResource(bits []byte) (resourceAddress, bool) {
	a, _, ok := parseMACResourceDetailed(bits)
	return a, ok
}

func parseMACResourceDetailed(bits []byte) (resourceAddress, string, bool) {
	if len(bits) < 40 || bitsToUint(bits, 0, 2) != 0 {
		return resourceAddress{}, "TIPO", false
	}
	encryption := bitsToUint(bits, 4, 2)
	lengthField := bitsToUint(bits, 7, 6)
	lengthBits := 0
	if lengthField >= 1 && lengthField <= 0x3a {
		lengthBits = int(lengthField) * 8
	}
	addrType := byte(bitsToUint(bits, 13, 3))
	lengths := [8]int{0, 24, 10, 24, 24, 34, 30, 34}
	need := 16 + lengths[addrType]
	if len(bits) < need || addrType == 0 {
		return resourceAddress{}, "DIRECCION", false
	}
	var ssi uint32
	var usageMarker uint8
	hasUsageMarker := false
	switch addrType {
	case 1, 3, 4, 5, 6, 7:
		ssi = bitsToUint(bits, 16, 24)
	default:
		return resourceAddress{}, "DIRECCION", false
	}
	if addrType == 6 {
		usageMarker = uint8(bitsToUint(bits, 40, 6))
		hasUsageMarker = true
	}
	if ssi == 0 {
		return resourceAddress{}, "SSI", false
	}
	cur := need
	if cur >= len(bits) {
		return resourceAddress{}, "CABECERA", false
	}
	if bits[cur] != 0 {
		cur += 5
	} else {
		cur++
	}
	if cur >= len(bits) {
		return resourceAddress{}, "CABECERA", false
	}
	if bits[cur] != 0 {
		cur += 9
	} else {
		cur++
	}
	if cur >= len(bits) {
		return resourceAddress{}, "CABECERA", false
	}
	hasAllocation := bits[cur] != 0
	cur++
	if hasAllocation {
		if encryption > 0 {
			// The clear MAC header already contains SSI+USAGE and encryption
			// mode. Keep that association even though the following allocation
			// cannot be interpreted without decrypting the PDU.
			return resourceAddress{Type: addrType, SSI: ssi, Encrypted: true, UsageMarker: usageMarker, HasUsageMarker: hasUsageMarker, HeaderBits: cur, LengthBits: lengthBits, ChannelAllocation: true}, "", true
		}
		var ok bool
		cur, ok = skipChannelAllocation(bits, cur)
		if !ok {
			return resourceAddress{}, "ASIGNACION", false
		}
	}
	if cur > len(bits) {
		return resourceAddress{}, "CABECERA", false
	}
	return resourceAddress{Type: addrType, SSI: ssi, Encrypted: encryption > 0, UsageMarker: usageMarker, HasUsageMarker: hasUsageMarker, HeaderBits: cur, LengthBits: lengthBits, ChannelAllocation: hasAllocation}, "", true
}

// skipChannelAllocation advances over 21.4.2.2 channel allocation, including
// its optional extended carrier and uplink/downlink assignment fields.
func skipChannelAllocation(bits []byte, cur int) (int, bool) {
	take := func(n int) (uint32, bool) {
		if n < 0 || cur+n > len(bits) {
			return 0, false
		}
		v := bitsToUint(bits, cur, n)
		cur += n
		return v, true
	}
	if _, ok := take(2 + 4); !ok {
		return cur, false
	}
	uldl, ok := take(2)
	if !ok {
		return cur, false
	}
	if _, ok = take(1 + 1 + 12); !ok {
		return cur, false
	}
	ext, ok := take(1)
	if !ok {
		return cur, false
	}
	if ext != 0 {
		if _, ok = take(4 + 2 + 3 + 1); !ok {
			return cur, false
		}
	}
	monitor, ok := take(2)
	if !ok {
		return cur, false
	}
	if monitor == 0 {
		if _, ok = take(2); !ok {
			return cur, false
		}
	}
	if uldl == 0 {
		if _, ok = take(2 + 3 + 3 + 3 + 3 + 3 + 4 + 5); !ok {
			return cur, false
		}
		napping, ok := take(2)
		if !ok {
			return cur, false
		}
		if napping == 1 {
			if _, ok = take(11); !ok {
				return cur, false
			}
		}
		if _, ok = take(4); !ok {
			return cur, false
		}
		flag, ok := take(1)
		if !ok {
			return cur, false
		}
		if flag != 0 {
			if _, ok = take(16); !ok {
				return cur, false
			}
		}
		flag, ok = take(1)
		if !ok {
			return cur, false
		}
		if flag != 0 {
			if _, ok = take(16); !ok {
				return cur, false
			}
		}
		if _, ok = take(1); !ok {
			return cur, false
		}
	}
	return cur, true
}
func bitsToUint(bits []byte, start, n int) uint32 {
	var v uint32
	for i := 0; i < n; i++ {
		v = (v << 1) | uint32(bits[start+i]&1)
	}
	return v
}

func userFromResource(a resourceAddress, slot int) User {
	return User{SSI: a.SSI, AddressType: a.Type, Slot: uint8(slot), Encrypted: a.Encrypted, LastSeen: time.Now(), Seen: 1}
}

type cmceInfo struct {
	Kind        string
	Code        uint8
	CallID      uint16
	CallingSSI  uint32
	SDSDataType uint8
	SDS         []byte
}

type llcPDU struct {
	Type       uint8
	TL         []byte
	Fragment   bool
	Final      bool
	NS, SS     uint8
	FCSInvalid bool
}

func llcFCS(bits []byte) uint32 {
	crc := uint32(0xffffffff)
	if len(bits) < 32 {
		crc <<= uint(32 - len(bits))
	}
	for _, input := range bits {
		bit := (uint32(input&1) ^ (crc >> 31)) & 1
		crc <<= 1
		if bit != 0 {
			crc ^= 0x04c11db7
		}
	}
	return ^crc
}

func parseLLC(mac []byte, a resourceAddress) (llcPDU, bool) {
	end := len(mac)
	if a.LengthBits > 0 && a.LengthBits < end {
		end = a.LengthBits
	}
	if a.HeaderBits >= end || a.Encrypted {
		return llcPDU{}, false
	}
	bits := mac[a.HeaderBits:end]
	if len(bits) < 4 {
		return llcPDU{}, false
	}
	t := uint8(bitsToUint(bits, 0, 4))
	p := llcPDU{Type: t}
	off := 0
	switch t {
	case 0:
		off = 6
	case 1:
		off = 5
	case 2:
		off = 4
	case 3:
		off = 5
	case 4:
		off = 6
	case 5:
		off = 5
	case 6:
		off = 4
	case 7:
		off = 5
	case 8:
		off = 20
	case 9:
		if len(bits) < 17 {
			return p, false
		}
		p.Final = bits[4] != 0
		p.Fragment = true
		p.NS = uint8(bitsToUint(bits, 6, 3))
		p.SS = uint8(bitsToUint(bits, 9, 8))
		off = 17
	case 10:
		if len(bits) < 21 {
			return p, false
		}
		p.Final = bits[4] != 0
		p.Fragment = true
		p.NS = uint8(bitsToUint(bits, 5, 8))
		p.SS = uint8(bitsToUint(bits, 13, 8))
		off = 21
	case 11, 15:
		return p, true
	case 12:
		off = 20
	case 13:
		if len(bits) < 7 {
			return p, false
		}
		sub := bitsToUint(bits, 4, 2)
		switch sub {
		case 0:
			if len(bits) < 24 {
				return p, false
			}
			p.Final = bits[6] != 0
			p.Fragment = true
			p.NS = uint8(bitsToUint(bits, 8, 8))
			p.SS = uint8(bitsToUint(bits, 16, 8))
			off = 24
		case 1:
			if len(bits) < 23 {
				return p, false
			}
			p.Final = bits[6] != 0
			p.Fragment = true
			p.NS = uint8(bitsToUint(bits, 7, 8))
			p.SS = uint8(bitsToUint(bits, 15, 8))
			off = 23
		default:
			return p, true
		}
	default:
		return p, true
	}
	if off > len(bits) {
		return p, false
	}
	p.TL = append([]byte(nil), bits[off:]...)
	if t >= 4 && t <= 7 {
		if len(p.TL) < 32 {
			return p, false
		}
		data, got := p.TL[:len(p.TL)-32], bitsToUint(p.TL, len(p.TL)-32, 32)
		p.FCSInvalid = llcFCS(data) != got
		p.TL = data
	}
	return p, true
}

func parseTLSDU(tl []byte) (cmceInfo, uint8, bool) {
	if len(tl) < 8 {
		return cmceInfo{}, 0, false
	}
	pdisc := uint8(bitsToUint(tl, 0, 3))
	if pdisc != 2 {
		return cmceInfo{}, pdisc, false
	}
	code := uint8(bitsToUint(tl, 3, 5))
	names := map[uint8]string{0: "D-ALERT", 1: "D-CALL PROCEEDING", 2: "D-CONNECT", 3: "D-CONNECT ACK", 4: "D-DISCONNECT", 5: "D-INFO", 6: "D-RELEASE", 7: "D-SETUP", 8: "D-STATUS", 9: "D-TX CEASED", 10: "D-TX CONTINUE", 11: "D-TX GRANTED", 12: "D-TX WAIT", 13: "D-TX INTERRUPT", 14: "D-CALL RESTORE", 15: "D-SDS DATA", 16: "D-FACILITY"}
	name, ok := names[code]
	if !ok {
		return cmceInfo{}, pdisc, false
	}
	var call uint16
	if len(tl) >= 22 {
		call = uint16(bitsToUint(tl, 8, 14))
	}
	result := cmceInfo{Kind: name, Code: code, CallID: call}
	if code == 15 {
		offset := 8
		if offset+2+24+2 > len(tl) {
			return result, pdisc, true
		}
		callerType := bitsToUint(tl, offset, 2)
		offset += 2
		result.CallingSSI = bitsToUint(tl, offset, 24)
		offset += 24
		if callerType == 2 {
			if offset+24 > len(tl) {
				return result, pdisc, true
			}
			offset += 24
		}
		dataType := bitsToUint(tl, offset, 2)
		result.SDSDataType = uint8(dataType)
		offset += 2
		length := [4]int{16, 32, 64, 0}[dataType]
		if dataType == 3 {
			if offset+11 > len(tl) {
				return result, pdisc, true
			}
			length = int(bitsToUint(tl, offset, 11))
			offset += 11
		}
		if length > 0 && offset+length <= len(tl) {
			result.SDS = append([]byte(nil), tl[offset:offset+length]...)
		}
	}
	return result, pdisc, true
}

func parseLLCCMCE(mac []byte, a resourceAddress) (cmceInfo, bool) {
	p, ok := parseLLC(mac, a)
	if !ok || p.Fragment || p.FCSInvalid {
		return cmceInfo{}, false
	}
	cmce, _, ok := parseTLSDU(p.TL)
	return cmce, ok
}
