package tetra

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func TestParseSDSSimpleText(t *testing.T) {
	bits := appendBits(nil, 2, 8)
	bits = appendBits(bits, 0, 1) // no timestamp
	bits = appendBits(bits, 1, 7) // 8-bit character coding
	for _, b := range []byte("HELLO") {
		bits = appendBits(bits, uint32(b), 8)
	}
	message, position, ok := parseSDS(bits, 6009004, time.Unix(123, 0))
	if !ok || position != nil || message.Kind != "SDS TEXT" || message.PartySSI != 6009004 || !strings.Contains(message.Text, "HELLO") || !message.Recognized {
		t.Fatalf("unexpected SDS result: ok=%v position=%v message=%+v", ok, position, message)
	}
}

func TestMessageJSONKeepsKindAndText(t *testing.T) {
	want := Message{Time: time.Unix(123, 0), Kind: "D-SDS DATA", Text: "contenido", AddressSSI: 5017011, PartySSI: 4198000, Slot: 3}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var got Message
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Kind != want.Kind || got.Text != want.Text || got.AddressSSI != want.AddressSSI || got.PartySSI != want.PartySSI || got.Slot != want.Slot {
		t.Fatalf("message fields were lost in JSON: %s", data)
	}
}

func TestBitsToHexPreservesPartialNibble(t *testing.T) {
	if got := bitsToHex([]byte{1, 0, 1, 0, 1, 1}); got != "AC" {
		t.Fatalf("bitsToHex() = %q, want AC", got)
	}
}

func TestParseSDSLIPAlsoCreatesMessage(t *testing.T) {
	const wantLat, wantLon = 40.4168, -3.7038
	latEncoded := uint32(math.Round(wantLat * float64(uint64(1)<<24) / 180))
	lonValue := wantLon
	if lonValue < 0 {
		lonValue += 360
	}
	lonEncoded := uint32(math.Round(lonValue * float64(uint64(1)<<25) / 360))
	bits := appendBits(nil, 10, 8)
	bits = appendBits(bits, 0, 2) // short location report
	bits = appendBits(bits, 1, 2) // elapsed time code
	bits = appendBits(bits, lonEncoded, 25)
	bits = appendBits(bits, latEncoded, 24)
	bits = appendBits(bits, 2, 3)  // accuracy: 40 m
	bits = appendBits(bits, 28, 7) // speed: 28 km/h
	bits = appendBits(bits, 4, 4)  // heading: 90 degrees
	bits = appendBits(bits, 0, 1)  // no optional data
	message, position, ok := parseSDS(bits, 6009004, time.Unix(456, 0))
	if !ok || position == nil || message.Kind != "GPS / LIP" {
		t.Fatalf("unexpected LIP result: ok=%v position=%v message=%+v", ok, position, message)
	}
	if math.Abs(position.Latitude-wantLat) > 0.00002 || math.Abs(position.Longitude-wantLon) > 0.00002 {
		t.Fatalf("wrong coordinates: got %.6f, %.6f", position.Latitude, position.Longitude)
	}
	if position.SpeedKmh != 28 || position.Heading != 90 || position.AccuracyM != 40 {
		t.Fatalf("wrong movement data: %+v", position)
	}
	if !strings.Contains(message.Text, "40.4168") || !strings.Contains(message.Text, "-3.7037") {
		t.Fatalf("GPS event is not useful in Messages: %q", message.Text)
	}
}
