package aircraft

import (
	"encoding/hex"
	"math"
	"testing"
	"time"
)

func encodeModeS(msg []byte) []uint16 {
	mag := make([]uint16, modeSPreambleSamples+len(msg)*16)
	for i := range mag {
		mag[i] = 2000
	}
	for _, i := range []int{0, 2, 7, 9} {
		mag[i] = 40000
	}
	bit := 0
	for _, b := range msg {
		for k := 7; k >= 0; k-- {
			off := modeSPreambleSamples + bit*2
			if (b>>uint(k))&1 == 1 {
				mag[off], mag[off+1] = 40000, 2000
			} else {
				mag[off], mag[off+1] = 2000, 40000
			}
			bit++
		}
	}
	return mag
}

func TestModeSCRCKnownFrames(t *testing.T) {
	for _, h := range []string{
		"8D4840D6202CC371C32CE0576098",
		"8D40621D58C382D690C8AC2863A7",
		"8D40621D58C386435CC412692AD6",
	} {
		raw, err := hex.DecodeString(h)
		if err != nil {
			t.Fatal(err)
		}
		if !modeSCRCValid(raw) {
			t.Fatalf("%s failed CRC (syndrome %06x)", h, modeSSyndrome(raw, 112))
		}
	}
}

func TestDetectModeSKnownPositionPair(t *testing.T) {
	even, _ := hex.DecodeString("8D40621D58C382D690C8AC2863A7")
	odd, _ := hex.DecodeString("8D40621D58C386435CC412692AD6")
	mag := encodeModeS(even)
	mag = append(mag, encodeModeS(odd)...)
	frames := detectModeS(mag)
	if len(frames) != 2 {
		t.Fatalf("got %d frames", len(frames))
	}
	if hex.EncodeToString(frames[0]) != "8d40621d58c382d690c8ac2863a7" {
		t.Fatalf("even frame %x", frames[0])
	}
	if hex.EncodeToString(frames[1]) != "8d40621d58c386435cc412692ad6" {
		t.Fatalf("odd frame %x", frames[1])
	}
	d := New(2_000_000, "", "", "")
	for _, f := range frames {
		d.decode1090(f)
	}
	list := d.Aircraft()
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
}

func TestProcessIQNativePosition(t *testing.T) {
	d := New(2_000_000, "", "", "")
	d.Configure(true, Mode1090)
	defer d.Close()
	even, _ := hex.DecodeString("8D40621D58C382D690C8AC2863A7")
	odd, _ := hex.DecodeString("8D40621D58C386435CC412692AD6")
	iq := magToIQ(append(encodeModeS(even), encodeModeS(odd)...))
	d.ProcessIQ(iq)
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatalf("no position from IQ: %+v", d.Aircraft())
		default:
			for _, a := range d.Aircraft() {
				if a.ICAO == "40621D" && a.Latitude != nil && a.Longitude != nil {
					return
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestNative1090StartStop(t *testing.T) {
	d := New(2_000_000, "", "", "")
	d.Configure(true, Mode1090)
	if !d.Snapshot().Running {
		t.Fatal("native 1090 did not start")
	}
	d.Close()
	if d.Snapshot().Running {
		t.Fatal("native 1090 did not stop")
	}
}

func TestSingleFrameCPRUsesReceiverReference(t *testing.T) {
	d := New(2_000_000, "", "", "")
	d.SetReference(-33.87, 151.21, true)
	st := d.stateFor("7C0001", Mode1090)
	ye, xe := encodeAirborne(-33.8688, 151.2093, false)
	st.applyCPR(&st.aircraft, ye, xe, false, false, st.aircraft.LastSeen)
	if st.aircraft.Latitude == nil || st.aircraft.Longitude == nil {
		t.Fatal("expected relative CPR from receiver location")
	}
	if math.Abs(*st.aircraft.Latitude+33.8688) > 0.002 || math.Abs(wrap180(*st.aircraft.Longitude-151.2093)) > 0.002 {
		t.Fatalf("got %.5f %.5f", *st.aircraft.Latitude, *st.aircraft.Longitude)
	}
}
