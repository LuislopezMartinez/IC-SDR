package digitalvoice

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestModeArgumentCoversInterfaceModes(t *testing.T) {
	cases := map[string]string{
		"AUTO · TODOS": "-fa", "DMR": "-fs", "P25 I": "-f1", "P25 II": "-f2",
		"NXDN 48": "-fi", "NXDN 96": "-fn", "D-STAR": "-fd", "YSF": "-fy",
		"dPMR": "-fm", "PROVOICE": "-fp", "M17": "-fz", "X2-TDMA": "-fx",
	}
	for mode, want := range cases {
		if got := modeArgument(mode); got != want {
			t.Errorf("%s: got %s, want %s", mode, got, want)
		}
	}
}

func TestParseLogUpdatesCommonCallFields(t *testing.T) {
	d := New(2_048_000, "missing", nil)
	d.parseLine("DMR voice TS 1 CC=7 SRC=123456 TG=91 BER=0.7 SNR=21.4")
	s := d.Snapshot()
	if s.Protocol != "DMR" || s.Slot != "SLOT 1" || s.ColorCode != "7" || s.Source != "123456" || s.Target != "91" {
		t.Fatalf("unexpected status: %+v", s)
	}
	if s.BER != .7 || s.SNR != 21.4 {
		t.Fatalf("unexpected quality: %+v", s)
	}
}

func TestRejectedAutoCandidateDoesNotChangeProtocol(t *testing.T) {
	d := New(2_048_000, "missing", nil)
	d.parseLine("DMR voice TS 1 CC=1 TG=9")
	d.parseLine("Sync: +M17 LSF CRC ERR")
	d.parseLine("Sync: +M17 EOT")
	if got := d.Snapshot().Protocol; got != "DMR" {
		t.Fatalf("rejected candidate replaced protocol: %q", got)
	}
}

func TestDigitalVoiceOutputResamplesEightKToFortyEightK(t *testing.T) {
	pcm := make([]byte, 480*2)
	for i := 0; i < 480; i++ {
		binary.LittleEndian.PutUint16(pcm[i*2:], uint16(int16(5000)))
	}
	count := 0
	d := New(2_048_000, "missing", func(samples []float32) { count += len(samples) })
	d.status.Running = true
	d.runOutput(bytes.NewReader(pcm))
	if count != (480-1)*6 {
		t.Fatalf("decoded output has wrong duration: got %d samples, want %d", count, (480-1)*6)
	}
}

func TestStereoDigitalVoiceOutputKeepsFrameDuration(t *testing.T) {
	const frames = 480
	pcm := make([]byte, frames*2*2)
	for frame := 0; frame < frames; frame++ {
		binary.LittleEndian.PutUint16(pcm[frame*4:], uint16(int16(5000)))
		binary.LittleEndian.PutUint16(pcm[frame*4+2:], uint16(int16(0)))
	}
	count := 0
	d := New(2_048_000, "missing", func(samples []float32) { count += len(samples) })
	d.outputChannels = 2
	d.status.Running = true
	d.runOutput(bytes.NewReader(pcm))
	if count != (frames-1)*6 {
		t.Fatalf("stereo output has wrong duration: got %d samples, want %d", count, (frames-1)*6)
	}
}

func TestModeOutputChannelsMatchDSDNeoPresets(t *testing.T) {
	for _, mode := range []string{"AUTO · TODOS", "DMR", "P25 II", "X2-TDMA"} {
		if got := modeOutputChannels(mode); got != 2 {
			t.Errorf("%s channels = %d, want 2", mode, got)
		}
	}
	for _, mode := range []string{"P25 I", "NXDN 48", "NXDN 96", "D-STAR", "YSF", "dPMR", "PROVOICE", "M17"} {
		if got := modeOutputChannels(mode); got != 1 {
			t.Errorf("%s channels = %d, want 1", mode, got)
		}
	}
}
