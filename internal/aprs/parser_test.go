package aprs

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDecodeKnownPosition(t *testing.T) {
	frame := append(address("APDW18", 0, false), address("ICSDR1", 0, true)...)
	frame = append(frame, 3, 0xf0)
	frame = append(frame, []byte("!4045.00N/00342.00W-IC-SDR APRS self-test")...)
	packet, ok := DecodeAX25(frame, 60)
	if !ok {
		t.Fatal("known AX.25 UI frame rejected")
	}
	if packet.Source != "ICSDR1" || packet.Destination != "APDW18" || packet.Type != "POSITION" {
		t.Fatalf("unexpected packet: %+v", packet)
	}
	if math.Abs(packet.Latitude-40.75) > .00001 || math.Abs(packet.Longitude+3.7) > .00001 {
		t.Fatalf("position %.6f, %.6f", packet.Latitude, packet.Longitude)
	}
	if packet.Locator == "—" || !strings.Contains(packet.Summary, "self-test") {
		t.Fatalf("metadata missing: %+v", packet)
	}
}

func TestDecodeCompressedPosition(t *testing.T) {
	lat, lon := 40.75, -3.7
	info := "!" + string('/') + base91TestEncode(int(math.Round((90-lat)*380926)), 4) + base91TestEncode(int(math.Round((lon+180)*190463)), 4) + ">  T"
	frame := append(address("APDW18", 0, false), address("ICSDR1", 0, true)...)
	frame = append(frame, 3, 0xf0)
	frame = append(frame, []byte(info)...)
	packet, ok := DecodeAX25(frame, 40)
	if !ok {
		t.Fatal("compressed frame rejected")
	}
	if math.Abs(packet.Latitude-lat) > .001 || math.Abs(packet.Longitude-lon) > .001 {
		t.Fatalf("compressed position %.6f, %.6f", packet.Latitude, packet.Longitude)
	}
}

func TestDecodeMicEPosition(t *testing.T) {
	frame := append(address("TPTU0P", 0, false), address("ICSDR1", 0, true)...)
	frame = append(frame, 3, 0xf0)
	info := []byte{'`', byte(3 + 28), byte(42 + 28), byte(0 + 28), byte(28), byte(28), byte(28), '>', '/'}
	frame = append(frame, info...)
	packet, ok := DecodeAX25(frame, 40)
	if !ok {
		t.Fatal("Mic-E frame rejected")
	}
	if packet.Type != "MIC-E" {
		t.Fatalf("type %q", packet.Type)
	}
	if math.Abs(packet.Latitude-40.75) > .001 || math.Abs(packet.Longitude+3.7) > .001 {
		t.Fatalf("Mic-E position %.6f, %.6f", packet.Latitude, packet.Longitude)
	}
}

func base91TestEncode(v, n int) string {
	b := make([]byte, n)
	for i := n - 1; i >= 0; i-- {
		b[i] = byte(v%91 + 33)
		v /= 91
	}
	return string(b)
}
func address(call string, ssid byte, last bool) []byte {
	call = strings.Split(call, "-")[0]
	call += strings.Repeat(" ", 6-len(call))
	result := make([]byte, 7)
	for i := 0; i < 6; i++ {
		result[i] = call[i] << 1
	}
	result[6] = 0x60 | (ssid << 1)
	if last {
		result[6] |= 1
	}
	return result
}
func TestFrontendRate(t *testing.T) {
	f := newFrontend(2_048_000)
	f.reset(0, 12500)
	iq := make([]float32, 4096*2)
	out := f.process(iq)
	if len(out) < 94 || len(out) > 98 {
		t.Fatalf("48 kHz output samples=%d", len(out))
	}
}

func TestBundledDireWolfKISSTransport(t *testing.T) {
	root := filepath.Join("..", "..", "ORIGEN", "IC_SDR", "tools", "aprs")
	executable := filepath.Join(root, "runtime", "bin", "direwolf.exe")
	pcmPath := filepath.Join(root, "results", "self-test", "known-packet.s16le")
	if _, err := os.Stat(executable); err != nil {
		t.Skip("bundled Dire Wolf unavailable")
	}
	data, err := os.ReadFile(pcmPath)
	if err != nil {
		t.Fatal(err)
	}
	decoder := New(2_048_000, executable, filepath.Join(root, "config", "direwolf-rx.conf"), filepath.Join(root, "runtime"))
	decoder.Configure(true, 144_800_000, 144_800_000, 12_500)
	defer decoder.Stop()
	deadline := time.Now().Add(4 * time.Second)
	for !decoder.Snapshot().KISS && time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
	}
	if !decoder.Snapshot().KISS {
		if config, err := os.ReadFile(decoder.configPath); err == nil {
			t.Logf("runtime config:\n%s", config)
		}
		t.Fatalf("KISS did not connect: %+v", decoder.Snapshot())
	}
	for offset := 0; offset < len(data); {
		count := min(2048, (len(data)-offset)/2)
		samples := make([]int16, count)
		for i := range samples {
			samples[i] = int16(binary.LittleEndian.Uint16(data[offset+i*2:]))
		}
		decoder.queue <- samples
		offset += count * 2
	}
	for time.Now().Before(deadline) {
		packets := decoder.Packets()
		if len(packets) > 0 {
			if packets[0].Source != "ICSDR1" {
				t.Fatalf("unexpected %+v", packets[0])
			}
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("known APRS frame not received through KISS: %+v", decoder.Snapshot())
}
