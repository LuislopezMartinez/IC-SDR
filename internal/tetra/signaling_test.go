package tetra

import (
	"strings"
	"testing"
	"time"
)

func setupExample() []byte {
	b := appendBits(nil, 2, 3)
	b = appendBits(b, 7, 5)
	for _, v := range []struct {
		v uint32
		n int
	}{{43, 14}, {0, 4}, {0, 1}, {0, 1}, {0, 3}, {0, 1}, {1, 2}, {0, 2}, {3, 2}, {0, 1}, {0, 4}, {1, 1}, {0, 1}, {0, 1}, {1, 1}, {1, 2}, {5003014, 24}, {0, 1}} {
		b = appendBits(b, v.v, v.n)
	}
	return b
}
func TestExtendedSetupAndDetail(t *testing.T) {
	bits := setupExample()
	c, _, ok := parseTLSDU(bits)
	if !ok || c.CallID != 43 || c.CallingSSI != 5003014 || c.Fields["Transmission_grant"] != 3 || c.Fields["Basic_service_Communication_type"] != 1 {
		t.Fatalf("setup fields: %+v valid=%v", c, ok)
	}
	carrier := uint16(832)
	m := Message{Kind: c.Kind, CallID: c.CallID, AddressSSI: 500100, PartySSI: c.CallingSSI, Carrier: &carrier, AssignedSlots: 3, Slot: 1, Fields: c.Fields}
	detail := m.Detail()
	for _, s := range []string{"Carrier:832", "SSI:500100", "Call ID:43", "Granted_to_another_user", "Party_SSI:5003014", "Basic_service:Group", "Clear Speech_TCH_S"} {
		if !strings.Contains(detail, s) {
			t.Fatalf("missing %s in %s", s, detail)
		}
	}
	for _, n := range []int{9, 22, 30, 45, len(bits) - 3} {
		if _, _, valid := parseTLSDU(bits[:n]); valid {
			t.Fatalf("truncated setup accepted at %d bits", n)
		}
	}
}
func TestSDSWithTransportHeader(t *testing.T) {
	b := appendBits(nil, 130, 8)
	b = appendBits(b, 0, 4)
	b = appendBits(b, 0, 2)
	b = appendBits(b, 0, 1)
	b = appendBits(b, 0, 1)
	b = appendBits(b, 17, 8)
	b = appendBits(b, 0, 1)
	b = appendBits(b, 1, 7)
	for _, c := range []byte("HOLA TL") {
		b = appendBits(b, uint32(c), 8)
	}
	m, p, ok := parseSDS(b, 123, time.Now())
	if !ok || p != nil || m.Text != "HOLA TL" || m.Fields["Message_reference"] != 17 || m.SDSProtocol != 130 {
		t.Fatalf("TL: %+v ok=%v", m, ok)
	}
}
func TestSimpleLocationNMEA(t *testing.T) {
	b := appendBits(nil, 3, 8)
	b = appendBits(b, 0, 8)
	for _, c := range []byte("$GPRMC,123519,A,4807.038,N,01131.000,E,022.4,084.4,230394,003.1,W*6A") {
		b = appendBits(b, uint32(c), 8)
	}
	m, p, ok := parseSDS(b, 500012, time.Now())
	if !ok || p == nil || p.SSI != 500012 || p.Latitude < 48.1172 || p.Latitude > 48.1174 || p.Longitude < 11.5166 || m.RawHex == "" {
		t.Fatalf("NMEA %+v %+v ok=%v", m, p, ok)
	}
	if parseNMEAPosition("$GPRMC,123519,V,4807.038,N,01131.000,E,0,0", 1, time.Now()) != nil {
		t.Fatal("invalid fix accepted")
	}
	if parseNMEAPosition("$GPRMC,123519,A,4807.038,N,01131.000,E,0,0*00", 1, time.Now()) != nil {
		t.Fatal("bad checksum accepted")
	}
}
func TestAllocationFieldsPreserved(t *testing.T) {
	// SSI resource followed by an allocation with one downlink assignment.
	bits := make([]byte, 100)
	put := func(off, n int, v uint32) {
		for i := 0; i < n; i++ {
			bits[off+i] = byte((v >> uint(n-1-i)) & 1)
		}
	}
	put(7, 6, 12)
	put(13, 3, 1)
	put(16, 24, 500100)
	put(42, 1, 1)
	start := 43
	put(start+2, 4, 3)
	put(start+6, 2, 1)
	put(start+10, 12, 832)
	put(start+23, 2, 1)
	a, ok := parseMACResource(bits)
	if !ok || a.Carrier != 832 || a.AssignedSlots != 3 || !a.ChannelAllocation {
		t.Fatalf("allocation: %+v valid=%v", a, ok)
	}
}
func TestSDSDoesNotCreatePhantomCall(t *testing.T) {
	b := appendBits(nil, 2, 3)
	b = appendBits(b, 15, 5)
	b = appendBits(b, 1, 2)
	b = appendBits(b, 6009004, 24)
	b = appendBits(b, 0, 2)
	b = appendBits(b, 0, 16)
	c, _, ok := parseTLSDU(b)
	if !ok || c.CallID != 0 {
		t.Fatal("SDS caller address interpreted as call ID")
	}
}

func TestLiveMACEventContainsCompleteSetup(t *testing.T) {
	header := make([]byte, 68)
	put := func(off, n int, v uint32) {
		for i := 0; i < n; i++ {
			header[off+i] = byte((v >> uint(n-1-i)) & 1)
		}
	}
	put(13, 3, 1)
	put(16, 24, 500100)
	put(42, 1, 1)
	put(45, 4, 3)
	put(49, 2, 1)
	put(53, 12, 832)
	put(66, 2, 1)
	payload := appendBits(header, 2, 4)
	payload = append(payload, setupExample()...)
	for len(payload)%8 != 0 {
		payload = append(payload, 0)
	}
	length := uint32(len(payload) / 8)
	for i := 0; i < 6; i++ {
		payload[7+i] = byte((length >> uint(5-i)) & 1)
	}
	d := &Decoder{users: map[uint32]User{}, groups: map[uint32]Group{}, calls: map[uint16]Call{}}
	d.processMACResourceLocked(payload, 0)
	if len(d.messages) != 1 {
		t.Fatalf("event not delivered: MAC rejected=%d LLC rejected=%d", d.macRejected, d.llcRejected)
	}
	e := d.messages[0]
	if e.CallID != 43 || e.Carrier == nil || *e.Carrier != 832 || e.PartySSI != 5003014 || e.RawHex == "" {
		t.Fatalf("live event incomplete: %+v", e)
	}
}
