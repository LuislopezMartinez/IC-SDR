package aircraft

import (
	"encoding/hex"
	"math"
	"strings"
	"testing"
)

func decodeHex(t *testing.T, d *Decoder, value string) {
	t.Helper()
	b, err := hex.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	d.decode1090(b)
}

func TestDecodeADSBIdentificationAndPosition(t *testing.T) {
	d := New(2_048_000, "", "", "")
	decodeHex(t, d, "8D4840D6202CC371C32CE0576098")
	decodeHex(t, d, "8D40621D58C382D690C8AC2863A7")
	decodeHex(t, d, "8D40621D58C386435CC412692AD6")
	list := d.Aircraft()
	if len(list) != 2 {
		t.Fatalf("got %d aircraft", len(list))
	}
	var pos *Aircraft
	for i := range list {
		if list[i].ICAO == "40621D" {
			pos = &list[i]
		}
	}
	if pos == nil || pos.Latitude == nil || pos.Longitude == nil {
		t.Fatalf("position not decoded: %+v", pos)
	}
	// The odd frame is the most recent one in this pair.
	if math.Abs(*pos.Latitude-52.26578) > .001 || math.Abs(*pos.Longitude-3.93891) > .001 {
		t.Fatalf("wrong CPR position %.5f %.5f", *pos.Latitude, *pos.Longitude)
	}
	var id *Aircraft
	for i := range list {
		if list[i].ICAO == "4840D6" {
			id = &list[i]
		}
	}
	if id == nil || id.Callsign != "KLM1023" {
		t.Fatalf("wrong callsign: %+v", id)
	}
}

func TestResampleCU8(t *testing.T) {
	iq := make([]float32, 2048*2)
	out := resampleCU8(iq, 2_048_000, 2_000_000)
	if len(out) != 4000 {
		t.Fatalf("unexpected output length %d", len(out))
	}
}

func TestDecodeUATText(t *testing.T) {
	d := New(2_048_000, "", "", "")
	d.read978(strings.NewReader("HDR:\n Address: ABC123 (ICAO)\n Callsign: N123AB\n Latitude: +40.4168 degrees\n Longitude: -3.7038 degrees\n Altitude: 12500 ft\n Speed: 245 kt\n Track: 87 degrees\n Vertical rate: +640 ft/min\n"))
	list := d.Aircraft()
	if len(list) != 1 || list[0].ICAO != "ABC123" || list[0].Callsign != "N123AB" {
		t.Fatalf("unexpected aircraft: %+v", list)
	}
	if list[0].Latitude == nil || math.Abs(*list[0].Latitude-40.4168) > .0001 || list[0].Altitude == nil || *list[0].Altitude != 12500 {
		t.Fatalf("UAT fields not decoded: %+v", list[0])
	}
}
