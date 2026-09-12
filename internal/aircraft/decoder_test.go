package aircraft

import (
	"encoding/hex"
	"math"
	"strings"
	"testing"
	"time"
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

func TestDecodeADSBSouthernHemisphere(t *testing.T) {
	d := New(2_048_000, "", "", "")
	now := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	for _, p := range []struct{ lat, lon float64 }{
		{-33.8688, 151.2093},
		{-45.8788, 170.5028},
		{61.2181, -149.9003},
	} {
		d.Clear()
		st := d.stateFor("7C0001", "1090 ADS-B")
		ye, xe := encodeAirborne(p.lat, p.lon, false)
		yo, xo := encodeAirborne(p.lat, p.lon, true)
		st.applyCPR(&st.aircraft, ye, xe, false, false, now)
		st.applyCPR(&st.aircraft, yo, xo, true, false, now.Add(time.Second))
		list := d.Aircraft()
		if len(list) != 1 || list[0].Latitude == nil || list[0].Longitude == nil {
			t.Fatalf("missing position at %+v: %+v", p, list)
		}
		if math.Abs(*list[0].Latitude-p.lat) > 0.002 || math.Abs(wrap180(*list[0].Longitude-p.lon)) > 0.002 {
			t.Fatalf("wrong position at %+v: %.5f %.5f", p, *list[0].Latitude, *list[0].Longitude)
		}
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

func TestDecodeRejectsTeleportJump(t *testing.T) {
	d := New(2_048_000, "", "", "")
	now := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	st := d.stateFor("7C0001", "1090 ADS-B")
	ye, xe := encodeAirborne(-33.8688, 151.2093, false)
	yo, xo := encodeAirborne(-33.8688, 151.2093, true)
	st.applyCPR(&st.aircraft, ye, xe, false, false, now)
	st.applyCPR(&st.aircraft, yo, xo, true, false, now.Add(time.Second))
	if st.aircraft.Latitude == nil {
		t.Fatal("first fix missing")
	}
	firstLat, firstLon := *st.aircraft.Latitude, *st.aircraft.Longitude
	ye, xe = encodeAirborne(51.5, -0.12, false)
	yo, xo = encodeAirborne(51.5, -0.12, true)
	st.even, st.odd = nil, nil
	st.applyCPR(&st.aircraft, ye, xe, false, false, now.Add(2*time.Second))
	st.applyCPR(&st.aircraft, yo, xo, true, false, now.Add(3*time.Second))
	if math.Abs(*st.aircraft.Latitude-firstLat) > 0.01 || math.Abs(wrap180(*st.aircraft.Longitude-firstLon)) > 0.01 {
		t.Fatalf("teleport accepted: %.5f %.5f", *st.aircraft.Latitude, *st.aircraft.Longitude)
	}
}
