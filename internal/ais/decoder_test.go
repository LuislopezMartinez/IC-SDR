package ais

import "testing"

func TestParseAndMergeVesselMessages(t *testing.T) {
	d := New(2_048_000, "")
	position, ok := parseMessage([]byte(`{"mmsi":244009864,"status":0,"status_text":"Under way using engine","lat":52.1,"lon":4.3,"speed":10.4,"course":91.2}`))
	if !ok {
		t.Fatal("position message was rejected")
	}
	d.merge(position)
	static, ok := parseMessage([]byte(`{"mmsi":244009864,"shipname":"TEST VESSEL","callsign":"PABC","shiptype":70,"shiptype_text":"Cargo","destination":"ROTTERDAM"}`))
	if !ok {
		t.Fatal("static message was rejected")
	}
	d.merge(static)
	vessels := d.Vessels()
	if len(vessels) != 1 || vessels[0].Name != "TEST VESSEL" || vessels[0].Latitude == nil || vessels[0].Messages != 2 {
		t.Fatalf("messages were not merged: %+v", vessels)
	}
}

func TestParseRejectsInvalidPosition(t *testing.T) {
	m, ok := parseMessage([]byte(`{"mmsi":123456789,"lat":91,"lon":4}`))
	if !ok {
		t.Fatal("identity of an AIS report must still be kept")
	}
	if m.Lat != nil || m.Lon != nil {
		t.Fatalf("invalid coordinates were kept: lat=%v lon=%v", m.Lat, m.Lon)
	}
}

func TestStoppedSessionRejectsBufferedMessages(t *testing.T) {
	d := New(2_048_000, "")
	d.running = false
	d.session = 4
	m, ok := parseMessage([]byte(`{"mmsi":244009864,"lat":52.1,"lon":4.3}`))
	if !ok {
		t.Fatal("test message was rejected")
	}
	d.mergeSession(m, 4)
	if got := len(d.Vessels()); got != 0 {
		t.Fatalf("stopped decoder accepted %d buffered vessel messages", got)
	}
}
