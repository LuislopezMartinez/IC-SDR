package datv

import "testing"

func TestTransportFramerAlignsSplitReads(t *testing.T) {
	stream := make([]byte, 5+188*4)
	for i := 0; i < 4; i++ {
		stream[5+i*188] = 0x47
	}
	var f transportFramer
	if got := f.push(stream[:200]); len(got) != 0 {
		t.Fatalf("premature output: %d", len(got))
	}
	got := f.push(stream[200:500])
	if len(got) != 188*2 || got[0] != 0x47 || got[188] != 0x47 {
		t.Fatalf("bad aligned output: %d", len(got))
	}
	got = f.push(stream[500:])
	if len(got) != 188*2 {
		t.Fatalf("tail output: %d", len(got))
	}
}

func TestTransportAnalyzerPATAndPMT(t *testing.T) {
	pat := psiPacket(0, []byte{0x00, 0xb0, 0x0d, 0, 1, 0xc1, 0, 0, 0, 1, 0xe1, 0x00, 0, 0, 0, 0})
	pmt := psiPacket(0x100, []byte{0x02, 0xb0, 0x17, 0, 1, 0xc1, 0, 0, 0xe1, 1, 0xf0, 0, 0x1b, 0xe1, 1, 0xf0, 0, 0x0f, 0xe1, 2, 0xf0, 0, 0, 0, 0, 0})
	a := transportAnalyzer{}
	a.consume(pat)
	video, audio := a.consume(pmt)
	if a.pmtPID != 0x100 || video != "H.264/AVC" || audio != "AAC" {
		t.Fatalf("pid=%x video=%q audio=%q", a.pmtPID, video, audio)
	}
}

func psiPacket(pid uint16, section []byte) []byte {
	p := make([]byte, 188)
	for i := range p {
		p[i] = 0xff
	}
	p[0], p[1], p[2], p[3], p[4] = 0x47, 0x40|byte(pid>>8), byte(pid), 0x10, 0
	copy(p[5:], section)
	return p
}
