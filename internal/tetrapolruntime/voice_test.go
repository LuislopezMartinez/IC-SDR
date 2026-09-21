package tetrapolruntime

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestVoiceBitsUsesLeastSignificantBitFirstOrder(t *testing.T) {
	bits, err := VoiceBits("010000000000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	if len(bits) != 120 || !strings.HasPrefix(bits, "10000000") {
		t.Fatalf("unexpected voice bits %q", bits)
	}
}

func TestVoiceBitsFromKitJSONFiltersNonVoiceFrames(t *testing.T) {
	line := []byte(`{"event":"frame","frame":{"state":"ok","type":"VOICE","data":{"encoding":"hex","value":"010000000000000000000000000000"}}}`)
	bits, accepted, err := VoiceBitsFromKitJSON(line)
	if err != nil || !accepted || len(bits) != 120 {
		t.Fatalf("bits=%q accepted=%v err=%v", bits, accepted, err)
	}
	_, accepted, err = VoiceBitsFromKitJSON([]byte(`{"event":"frame","frame":{"state":"bad_CRC"}}`))
	if err != nil || accepted {
		t.Fatalf("bad frame accepted=%v err=%v", accepted, err)
	}
}

func TestPCM16LEToFloat32(t *testing.T) {
	got := PCM16LEToFloat32([]byte{0, 0, 0xff, 0x7f, 0, 0x80})
	if len(got) != 3 || got[0] != 0 || got[1] < .999 || got[2] != -1 {
		t.Fatalf("samples=%v", got)
	}
}

func TestRPCELPStreamsPCMFromExternalRuntime(t *testing.T) {
	if os.Getenv("GO_WANT_RPCELP_HELPER") == "1" {
		var line string
		_, _ = fmt.Fscanln(os.Stdin, &line)
		_, _ = os.Stdout.Write([]byte{0, 0, 0xff, 0x7f})
		os.Exit(0)
	}
	if err := os.Setenv("GO_WANT_RPCELP_HELPER", "1"); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("GO_WANT_RPCELP_HELPER")
	got := make(chan []float32, 1)
	decoder := &RPCELP{}
	if err := decoder.Start(os.Args[0], []string{"-test.run=TestRPCELPStreamsPCMFromExternalRuntime"}, func(samples []float32) { got <- samples }); err != nil {
		t.Fatal(err)
	}
	line := []byte(`{"event":"frame","frame":{"state":"ok","type":"VOICE","data":{"encoding":"hex","value":"010000000000000000000000000000"}}}`)
	if _, err := decoder.SubmitKitJSON(line); err != nil {
		t.Fatal(err)
	}
	select {
	case samples := <-got:
		if len(samples) != 2 || samples[0] != 0 || samples[1] < .999 {
			t.Fatalf("unexpected PCM %v", samples)
		}
	case <-time.After(time.Second):
		t.Fatal("RP-CELP output was not delivered")
	}
	if err := decoder.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestKitStreamsFrameRecordsFromExternalRuntime(t *testing.T) {
	if os.Getenv("GO_WANT_KIT_HELPER") == "1" {
		var value byte
		_, _ = os.Stdin.Read([]byte{value})
		_, _ = fmt.Fprintln(os.Stdout, `{"event":"frame","frame":{"state":"ok","type":"VOICE"}}`)
		os.Exit(0)
	}
	if err := os.Setenv("GO_WANT_KIT_HELPER", "1"); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("GO_WANT_KIT_HELPER")
	records := make(chan []byte, 1)
	kit := &Kit{}
	if err := kit.Start(os.Args[0], []string{"-test.run=TestKitStreamsFrameRecordsFromExternalRuntime"}, func(record []byte) { records <- record }); err != nil {
		t.Fatal(err)
	}
	if err := kit.SubmitBits([]byte{1}); err != nil {
		t.Fatal(err)
	}
	select {
	case record := <-records:
		if !strings.Contains(string(record), `"VOICE"`) {
			t.Fatalf("record = %q", record)
		}
	case <-time.After(time.Second):
		t.Fatal("tetrapol_dump output was not delivered")
	}
	if err := kit.Stop(); err != nil {
		t.Fatal(err)
	}
}
