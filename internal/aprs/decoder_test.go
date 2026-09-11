package aprs

import (
	"testing"
	"time"
)

func TestDecoderSuppressesImmediateIdenticalPackets(t *testing.T) {
	d := New(2_048_000, "", "", "")
	packet := Packet{Raw: "EA1ABC>APRS:>test", Source: "EA1ABC", Destination: "APRS", Received: time.Now()}
	d.add(packet)
	d.add(packet)
	d.add(packet)
	if got := len(d.Packets()); got != 1 {
		t.Fatalf("identical KISS delivery stored %d copies", got)
	}
	if got := d.Snapshot().PacketCount; got != 1 {
		t.Fatalf("packet count includes duplicates: %d", got)
	}
}

func TestOldAPRSSessionCannotBecomeActiveAgain(t *testing.T) {
	d := New(2_048_000, "", "", "")
	d.running.Store(true)
	old := d.session.Add(1)
	if !d.sessionActive(old) {
		t.Fatal("current session was not active")
	}
	d.session.Add(1)
	if d.sessionActive(old) {
		t.Fatal("old session became active after generation changed")
	}
}
