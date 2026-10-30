package tetra

import "testing"

func appendBits(dst []byte, value uint32, width int) []byte {
	for i := width - 1; i >= 0; i-- {
		dst = append(dst, byte(value>>uint(i))&1)
	}
	return dst
}

func TestParseNeighbourBroadcast(t *testing.T) {
	var bits []byte
	bits = appendBits(bits, 5, 3) // MLE
	bits = appendBits(bits, 2, 3) // D-NWRK-BROADCAST
	bits = appendBits(bits, 0, 16)
	bits = appendBits(bits, 2, 2)
	bits = appendBits(bits, 1, 1) // options
	bits = appendBits(bits, 0, 1) // no network time
	bits = appendBits(bits, 1, 1) // neighbour count present
	bits = appendBits(bits, 1, 3)
	bits = appendBits(bits, 7, 5)
	bits = appendBits(bits, 0, 2)
	bits = appendBits(bits, 1, 1)
	bits = appendBits(bits, 3, 2)
	bits = appendBits(bits, 3666, 12)
	bits = appendBits(bits, 0, 1) // no neighbour options
	network := NetworkInfo{Valid: true, FrequencyBand: 3, FrequencyOffset: 0}
	cells, ok := parseNeighbourBroadcast(bits, SystemInfo{MCC: 214, MNC: 8}, network)
	if !ok || len(cells) != 1 {
		t.Fatalf("cells=%#v ok=%v", cells, ok)
	}
	cell := cells[0]
	if cell.CellID != 7 || cell.Carrier != 3666 || cell.FrequencyHz != 391_650_000 || !cell.Synchronized || cell.ServiceLevel != 3 {
		t.Fatalf("unexpected neighbour: %#v", cell)
	}
}

func TestParseNeighbourFrequencyExtension(t *testing.T) {
	var bits []byte
	bits = appendBits(bits, 3, 5)
	bits = appendBits(bits, 0, 2)
	bits = appendBits(bits, 1, 1)
	bits = appendBits(bits, 2, 2)
	bits = appendBits(bits, 100, 12)
	bits = appendBits(bits, 1, 1) // neighbour options
	bits = appendBits(bits, 1, 1) // main-carrier extension present
	bits = appendBits(bits, 4, 4) // frequency band
	bits = appendBits(bits, 1, 2) // +6.25 kHz offset
	bits = appendBits(bits, 5, 3) // duplex spacing
	bits = appendBits(bits, 1, 1) // reverse operation
	for i := 0; i < 9; i++ {
		bits = appendBits(bits, 0, 1) // remaining optional fields absent
	}

	cell, ok := parseNeighbour(&bitReader{bits: bits}, SystemInfo{MCC: 214, MNC: 8}, NetworkInfo{})
	if !ok {
		t.Fatal("valid extended neighbour rejected")
	}
	if cell.Carrier != 100 || cell.FrequencyBand != 4 || cell.FrequencyOffset != 1 || cell.DuplexSpacing != 5 || !cell.Reverse {
		t.Fatalf("extension decoded incorrectly: %#v", cell)
	}
	if cell.FrequencyHz != 402_506_250 {
		t.Fatalf("frequency=%d, want 402506250", cell.FrequencyHz)
	}
}
