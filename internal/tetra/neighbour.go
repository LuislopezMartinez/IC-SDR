package tetra

import "time"

type bitReader struct {
	bits []byte
	pos  int
}

func (r *bitReader) take(n int) (uint32, bool) {
	if n < 0 || r.pos+n > len(r.bits) {
		return 0, false
	}
	v := bitsToUint(r.bits, r.pos, n)
	r.pos += n
	return v, true
}

func (r *bitReader) optional(n int) (uint32, bool, bool) {
	present, ok := r.take(1)
	if !ok {
		return 0, false, false
	}
	if present == 0 {
		return 0, false, true
	}
	v, ok := r.take(n)
	return v, true, ok
}

func carrierFrequency(carrier uint32, network NetworkInfo) int64 {
	if !network.Valid {
		return 0
	}
	return carrierFrequencyParameters(carrier, network.FrequencyBand, network.FrequencyOffset)
}

func carrierFrequencyParameters(carrier uint32, band, offset uint8) int64 {
	offsets := [4]int64{0, 6250, -6250, 12500}
	return int64(band)*100_000_000 + int64(carrier)*25_000 + offsets[offset&3]
}

// parseNeighbourBroadcast handles MLE D-NWRK-BROADCAST and its optional
// neighbour-cell elements as defined by EN 300 392-2.
func parseNeighbourBroadcast(tl []byte, system SystemInfo, network NetworkInfo) ([]Neighbour, bool) {
	r := bitReader{bits: tl}
	pdisc, ok := r.take(3)
	if !ok || pdisc != 5 {
		return nil, false
	}
	primitive, ok := r.take(3)
	if !ok || primitive != 2 {
		return nil, false
	}
	if _, ok = r.take(16); !ok {
		return nil, false
	}
	if _, ok = r.take(2); !ok {
		return nil, false
	}
	options, ok := r.take(1)
	if !ok {
		return nil, false
	}
	if options == 0 {
		return []Neighbour{}, true
	}
	if present, ok := r.take(1); !ok {
		return nil, false
	} else if present != 0 {
		if _, ok = r.take(24 + 1 + 23); !ok {
			return nil, false
		}
	}
	present, ok := r.take(1)
	if !ok {
		return nil, false
	}
	if present == 0 {
		return []Neighbour{}, true
	}
	count, ok := r.take(3)
	if !ok {
		return nil, false
	}
	result := make([]Neighbour, 0, count)
	for i := uint32(0); i < count; i++ {
		cell, ok := parseNeighbour(&r, system, network)
		if !ok {
			return result, false
		}
		result = append(result, cell)
	}
	return result, true
}

func parseNeighbour(r *bitReader, system SystemInfo, network NetworkInfo) (Neighbour, bool) {
	cellID, ok := r.take(5)
	if !ok {
		return Neighbour{}, false
	}
	if _, ok = r.take(2); !ok {
		return Neighbour{}, false
	}
	syncValue, ok := r.take(1)
	if !ok {
		return Neighbour{}, false
	}
	service, ok := r.take(2)
	if !ok {
		return Neighbour{}, false
	}
	carrier, ok := r.take(12)
	if !ok {
		return Neighbour{}, false
	}
	n := Neighbour{CellID: uint8(cellID), Carrier: carrier, MCC: system.MCC, MNC: system.MNC, Synchronized: syncValue != 0, ServiceLevel: uint8(service), LastSeen: time.Now()}
	if network.Valid {
		n.FrequencyBand = network.FrequencyBand
		n.FrequencyOffset = network.FrequencyOffset
		n.DuplexSpacing = network.DuplexSpacing
	}
	options, ok := r.take(1)
	if !ok {
		return Neighbour{}, false
	}
	if options == 0 {
		n.FrequencyHz = carrierFrequency(n.Carrier, network)
		return n, true
	}
	if present, ok := r.take(1); !ok {
		return Neighbour{}, false
	} else if present != 0 {
		band, ok := r.take(4)
		if !ok {
			return Neighbour{}, false
		}
		offset, ok := r.take(2)
		if !ok {
			return Neighbour{}, false
		}
		duplex, ok := r.take(3)
		if !ok {
			return Neighbour{}, false
		}
		reverse, ok := r.take(1)
		if !ok {
			return Neighbour{}, false
		}
		n.FrequencyBand = uint8(band)
		n.FrequencyOffset = uint8(offset)
		n.DuplexSpacing = uint8(duplex)
		n.Reverse = reverse != 0
	}
	if value, present, valid := r.optional(10); !valid {
		return Neighbour{}, false
	} else if present {
		n.MCC = uint16(value)
	}
	if value, present, valid := r.optional(14); !valid {
		return Neighbour{}, false
	} else if present {
		n.MNC = uint16(value)
	}
	if value, present, valid := r.optional(14); !valid {
		return Neighbour{}, false
	} else if present {
		n.LocationArea = uint16(value)
	}
	for _, width := range []int{3, 4, 16} {
		if _, _, valid := r.optional(width); !valid {
			return Neighbour{}, false
		}
	}
	servicesPresent, ok := r.take(1)
	if !ok {
		return Neighbour{}, false
	}
	if servicesPresent != 0 {
		services, ok := r.take(12)
		if !ok {
			return Neighbour{}, false
		}
		n.Priority = services&(1<<9) != 0
		n.Voice = services&(1<<5) != 0
	}
	for _, width := range []int{5, 6} {
		if _, _, valid := r.optional(width); !valid {
			return Neighbour{}, false
		}
	}
	if network.Valid || n.FrequencyBand != 0 {
		n.FrequencyHz = carrierFrequencyParameters(n.Carrier, n.FrequencyBand, n.FrequencyOffset)
	}
	return n, true
}
