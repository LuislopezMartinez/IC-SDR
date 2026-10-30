package tetra

import (
	"testing"
	"time"
)

func TestParseMACFragmentAndEnd(t *testing.T) {
	fragmentBits := appendBits(nil, 1, 2)
	fragmentBits = appendBits(fragmentBits, 0, 1) // MAC-FRAG
	fragmentBits = appendBits(fragmentBits, 0, 1) // no fill
	fragmentBits = append(fragmentBits, 1, 0, 1, 1)
	fragment, ok := parseMACFragment(fragmentBits)
	if !ok || fragment.End || len(fragment.Payload) != 4 {
		t.Fatalf("unexpected MAC-FRAG: %#v ok=%v", fragment, ok)
	}
	fragmentBits[3] = 1
	fragmentBits = append(fragmentBits, 0, 0)
	trimmed, ok := parseMACFragment(fragmentBits)
	if !ok || len(trimmed.Payload) != 4 {
		t.Fatalf("MAC-FRAG fill bits were not removed: %#v ok=%v", trimmed, ok)
	}

	endBits := appendBits(nil, 1, 2)
	endBits = appendBits(endBits, 1, 1) // MAC-END
	endBits = appendBits(endBits, 0, 1) // no fill
	endBits = appendBits(endBits, 2, 6)
	endBits = appendBits(endBits, 0, 1) // no slot grant
	endBits = appendBits(endBits, 0, 1) // no channel allocation
	endBits = append(endBits, 1, 1, 0, 1)
	end, ok := parseMACFragment(endBits)
	if !ok || !end.End || len(end.Payload) != 4 {
		t.Fatalf("unexpected MAC-END: %#v ok=%v", end, ok)
	}
}

func TestMACFragmentReassemblyDeliversNeighbourBroadcast(t *testing.T) {
	var tl []byte
	tl = appendBits(tl, 5, 3) // MLE
	tl = appendBits(tl, 2, 3) // D-NWRK-BROADCAST
	tl = appendBits(tl, 0, 16)
	tl = appendBits(tl, 2, 2)
	tl = appendBits(tl, 1, 1) // optional elements
	tl = appendBits(tl, 0, 1) // no network time
	tl = appendBits(tl, 1, 1) // neighbour count present
	tl = appendBits(tl, 1, 3)
	tl = appendBits(tl, 7, 5)
	tl = appendBits(tl, 0, 2)
	tl = appendBits(tl, 1, 1)
	tl = appendBits(tl, 3, 2)
	tl = appendBits(tl, 3666, 12)
	tl = appendBits(tl, 0, 1) // no neighbour options

	llc := appendBits(nil, 2, 4) // BL-ADATA LLC header
	llc = append(llc, tl...)
	if len(llc) < 50 {
		t.Fatalf("test vector unexpectedly short: %d", len(llc))
	}

	start := make([]byte, 43)
	putBits(start, 0, 2, 0)       // MAC-RESOURCE
	putBits(start, 7, 6, 0x3f)    // start fragment
	putBits(start, 13, 3, 1)      // SSI address
	putBits(start, 16, 24, 12345) // SSI
	start = append(start, llc[:20]...)

	continuation := appendBits(nil, 1, 2)
	continuation = appendBits(continuation, 0, 1)
	continuation = appendBits(continuation, 0, 1)
	continuation = append(continuation, llc[20:40]...)

	end := appendBits(nil, 1, 2)
	end = appendBits(end, 1, 1)
	end = appendBits(end, 0, 1)
	end = appendBits(end, 0, 6) // use the decoded block boundary
	end = appendBits(end, 0, 1) // no slot grant
	end = appendBits(end, 0, 1) // no channel allocation
	end = append(end, llc[40:]...)

	d := &Decoder{
		users:      map[uint32]User{},
		groups:     map[uint32]Group{},
		calls:      map[uint16]Call{},
		neighbours: map[uint8]Neighbour{},
		positions:  map[uint32]Position{},
		fragments:  map[uint32]llcFragment{},
		system:     SystemInfo{MCC: 214, MNC: 8},
		network:    NetworkInfo{Valid: true, FrequencyBand: 3},
	}
	d.processMACPayloadLocked(start, 0)
	// TETRA permits continuation fragments on another slot of a multi-slot
	// control channel. With a single unambiguous start, both must still attach.
	d.processMACPayloadLocked(continuation, 1)
	d.processMACPayloadLocked(end, 2)

	cell, found := d.neighbours[7]
	if !found || cell.Carrier != 3666 || cell.FrequencyHz != 391_650_000 {
		t.Fatalf("reassembled neighbour missing or invalid: %#v found=%v", cell, found)
	}
	if d.macFragmentStarts != 1 || d.macFragmentParts != 2 || d.macFragmentCompleted != 1 || d.neighbourBroadcasts != 1 || d.neighbourParseFailures != 0 {
		t.Fatalf("unexpected diagnostics: starts=%d parts=%d completed=%d broadcasts=%d failures=%d", d.macFragmentStarts, d.macFragmentParts, d.macFragmentCompleted, d.neighbourBroadcasts, d.neighbourParseFailures)
	}
}

func TestMACFragmentExpiresIncompleteSequence(t *testing.T) {
	d := &Decoder{}
	d.macReassembly[2] = macFragmentState{Active: true, Bits: []byte{1}, Updated: time.Now().Add(-macFragmentTimeout - time.Millisecond)}
	d.expireMACFragmentsLocked(time.Now())
	if d.macReassembly[2].Active || d.macFragmentDropped != 1 {
		t.Fatalf("expired fragment was not discarded: %#v dropped=%d", d.macReassembly[2], d.macFragmentDropped)
	}
}

func putBits(dst []byte, start, width int, value uint32) {
	for i := 0; i < width; i++ {
		dst[start+i] = byte(value>>uint(width-1-i)) & 1
	}
}
