package tetra

import "testing"

func TestParseMACResourceSSI(t *testing.T) {
	bits := make([]byte, 64)
	put := func(start, n int, v uint32) {
		for i := 0; i < n; i++ {
			bits[start+i] = byte((v >> uint(n-1-i)) & 1)
		}
	}
	put(0, 2, 0)
	put(4, 2, 1)
	put(7, 6, 5)
	put(13, 3, 1)
	put(16, 24, 0x123456)
	a, ok := parseMACResource(bits)
	if !ok || a.SSI != 0x123456 || a.Type != 1 || !a.Encrypted {
		t.Fatalf("unexpected resource: %#v ok=%v", a, ok)
	}
}

func TestParseLLCCMCESetup(t *testing.T) {
	bits := make([]byte, 48)
	put := func(start, n int, v uint32) {
		for i := 0; i < n; i++ {
			bits[start+i] = byte((v >> uint(n-1-i)) & 1)
		}
	}
	put(0, 4, 2)
	put(4, 3, 2)
	put(7, 5, 7)
	put(12, 14, 321)
	got, ok := parseLLCCMCE(bits, resourceAddress{})
	if !ok || got.Kind != "D-SETUP" || got.CallID != 321 {
		t.Fatalf("unexpected CMCE: %#v ok=%v", got, ok)
	}
}

func TestDecodeSCHFRoundTrip(t *testing.T) {
	payload := make([]byte, 268)
	for i := range payload {
		payload[i] = byte((i*5 + i/7) & 1)
	}
	payload[0], payload[1] = 0, 0
	codeword := append([]byte(nil), payload...)
	found := false
	for candidate := 0; candidate < 65536; candidate++ {
		trial := append([]byte(nil), codeword...)
		for i := 0; i < 16; i++ {
			trial = append(trial, byte((candidate>>(15-i))&1))
		}
		if crc16Bits(trial) == 0x1d0f {
			codeword = trial
			found = true
			break
		}
	}
	if !found {
		t.Fatal("could not construct CRC")
	}
	codeword = append(codeword, 0, 0, 0, 0)
	poly := [4]byte{0x13, 0x1d, 0x17, 0x1b}
	parity := func(v byte) byte { v ^= v >> 4; v ^= v >> 2; v ^= v >> 1; return v & 1 }
	state := byte(0)
	mother := make([]byte, 0, 1152)
	for _, bit := range codeword {
		sr := (state << 1) | bit
		for _, p := range poly {
			mother = append(mother, parity(sr&p))
		}
		state = sr & 15
	}
	punct := make([]byte, 0, 432)
	keep := [8]bool{true, true, false, false, true, false, false, false}
	for i, b := range mother {
		if keep[i&7] {
			punct = append(punct, b)
		}
	}
	interleaved := make([]byte, 432)
	for k := 1; k <= 432; k++ {
		j := 1 + (103*k)%432
		interleaved[j-1] = punct[k-1]
	}
	si := SystemInfo{Valid: true, MCC: 214, MNC: 8, ColourCode: 23}
	lfsr := scramblingInit(si)
	for i := range interleaved {
		st := func(y uint) uint32 { return lfsr >> (32 - y) }
		b := (st(32) ^ st(26) ^ st(23) ^ st(22) ^ st(16) ^ st(12) ^ st(11) ^ st(10) ^ st(8) ^ st(7) ^ st(5) ^ st(4) ^ st(2) ^ st(1)) & 1
		interleaved[i] ^= byte(b)
		lfsr = (lfsr >> 1) | (b << 31)
	}
	got, ok := decodeSCHF(interleaved, si)
	if !ok {
		t.Fatal("valid SCH/F rejected")
	}
	for i := range payload {
		if got[i] != payload[i] {
			t.Fatalf("payload bit %d differs", i)
		}
	}
}
