package radiosonde

import (
	"bytes"
	"context"
	"encoding/binary"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseTelemetry(t *testing.T) {
	line := []byte(`{"type":"RS41","id":"K1930308","frame":5047,"datetime":"2014-07-17T12:32:13.999Z","lat":45.66939,"lon":15.87963,"alt":28527.16665,"vel_v":0,"ref_datetime":"GPS","ref_position":"GPS"}`)
	e, ok := ParseEvent(line)
	if !ok || e.ID != "K1930308" || e.Alt == nil || *e.Alt != 28527.16665 || e.Climb == nil || *e.Climb != 0 || e.Temperature != nil || e.DatetimeReference != "GPS" {
		t.Fatalf("incorrect telemetry: %+v", e)
	}
	line[0] = 'x'
	if e.Raw[0] != '{' {
		t.Fatal("raw record aliases input buffer")
	}
	for _, s := range []string{`diagnostic`, `{}`, `{"type":"DFM","id":"a","lat":91}`, `{"type":"DFM","id":"a","lon":-181}`, `{"type":"DFM","id":"a","lat":"bad"}`} {
		if _, ok := ParseEvent([]byte(s)); ok {
			t.Fatalf("accepted invalid record %s", s)
		}
	}
}

func TestChannelArguments(t *testing.T) {
	_, args, err := Arguments("DFM", 2_000_000, 402_500_000, 403_000_000)
	if err != nil || !strings.Contains(strings.Join(args, " "), "--IQ -0.250000000") || !strings.HasSuffix(strings.Join(args, " "), "- 2000000 32") {
		t.Fatalf("wrong IQ channel: %v %v", args, err)
	}
	for _, rate := range []float64{0, math.NaN(), math.Inf(1), 48000} {
		if _, _, err := Arguments("RS41", rate, 403e6, 403e6); err == nil {
			t.Fatal("accepted bad rate", rate)
		}
	}
	if _, _, err := Arguments("RS41", 100000, 403050000, 403000000); err == nil {
		t.Fatal("accepted channel outside capture")
	}
	if _, _, err := Arguments("unknown", 100000, 403000000, 403000000); err == nil {
		t.Fatal("accepted unknown family")
	}
}

func TestMissingRuntime(t *testing.T) {
	d := New(100000, t.TempDir())
	defer d.Close()
	d.Configure(true, "RS41", 403e6, 403e6)
	s := d.Snapshot()
	if s.Running || s.Error == "" {
		t.Fatalf("missing runtime not reported: %+v", s)
	}
	d.Close()
	d.Close()
}

// Enable against the shipped upstream recording and native executables with
// RADIOSONDE_INTEGRATION_ROOT pointing to DATA/tools/radiosonde.
// This validates the real stdin float32 transport, channel offsets and process
// shutdown/restart, not a mocked JSON producer.
func TestNativeDFMIQ(t *testing.T) {
	root := os.Getenv("RADIOSONDE_INTEGRATION_ROOT")
	if root == "" {
		t.Skip("native integration assets not requested")
	}
	wav, err := os.ReadFile(filepath.Join(root, "vendor", "RS", "iq", "dfmIQ.wav"))
	if err != nil {
		t.Fatal(err)
	}
	var pcm []byte
	for offset := 12; offset+8 <= len(wav); {
		size := int(binary.LittleEndian.Uint32(wav[offset+4:]))
		end := offset + 8 + size
		if end > len(wav) {
			t.Fatal("invalid WAV chunk")
		}
		if string(wav[offset:offset+4]) == "data" {
			pcm = wav[offset+8 : end]
			break
		}
		offset = end + size%2
	}
	if len(pcm) == 0 {
		t.Fatal("no IQ data")
	}
	d := New(100000, filepath.Join(root, "runtime", "bin"))
	defer d.Close()
	var ids []string
	for _, offset := range []int64{-10000, 10000} {
		d.Clear()
		d.Configure(true, "DFM", 403000000+offset, 403000000)
		if s := d.Snapshot(); !s.Running {
			t.Fatalf("start failed: %+v", s)
		}
		// Feed more slowly than the queue drains, while staying faster than RF time.
		for start := 0; start < len(pcm); start += 4096 {
			end := min(start+4096, len(pcm))
			iq := make([]float32, end-start)
			for i, b := range pcm[start:end] {
				iq[i] = (float32(b) - 128) / 128
			}
			d.ProcessIQ(iq)
			time.Sleep(2 * time.Millisecond)
		}
		deadline := time.Now().Add(3 * time.Second)
		for len(d.Events()) < 8 && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		records := d.Events()
		status := d.Snapshot()
		d.Close()
		if len(records) < 8 || status.Dropped != 0 {
			t.Fatalf("native IQ decode: records=%d status=%+v", len(records), status)
		}
		e := records[len(records)-1]
		if e.Lat == nil || *e.Lat < 49 || *e.Lat > 51 || e.FrequencyHz != 403000000+offset {
			t.Fatalf("wrong position/frequency: %+v", e)
		}
		ids = append(ids, e.ID)
		t.Logf("offset %d Hz: %d frames, id=%s, lat=%.5f lon=%.5f", offset, len(records), e.ID, *e.Lat, *e.Lon)
		if d.Snapshot().Running {
			t.Fatal("decoder still running after close")
		}
	}
	if ids[0] == ids[1] {
		t.Fatalf("different IQ channels produced same identity: %v", ids)
	}
	// Repeat at a realistic 2 MS/s capture rate. Hold each complex sample for
	// 20 input samples; RS's anti-alias filter removes the interpolation images.
	d = New(2_000_000, filepath.Join(root, "runtime", "bin"))
	defer d.Close()
	d.Configure(true, "DFM", 402990000, 403000000)
	if !d.Snapshot().Running {
		t.Fatalf("high-rate start: %+v", d.Snapshot())
	}
	for start := 0; start < len(pcm); start += 4096 {
		end := min(start+4096, len(pcm))
		iq := make([]float32, (end-start)*20)
		for i := start; i+1 < end; i += 2 {
			for n := 0; n < 20; n++ {
				j := (i-start)*20 + n*2
				iq[j] = (float32(pcm[i]) - 128) / 128
				iq[j+1] = (float32(pcm[i+1]) - 128) / 128
			}
		}
		d.ProcessIQ(iq)
		time.Sleep(21 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	if s := d.Snapshot(); s.Events < 8 || s.Dropped != 0 {
		t.Fatalf("2 MS/s decoding: %+v", s)
	} else {
		t.Logf("2 MS/s: %d frames, %d dropped blocks", s.Events, s.Dropped)
	}
	d.Close()
}

func TestNativeRS41AndLifecycle(t *testing.T) {
	root := os.Getenv("RADIOSONDE_INTEGRATION_ROOT")
	if root == "" {
		t.Skip("native integration assets not requested")
	}
	bin := filepath.Join(root, "runtime", "bin")
	name, _, _ := Arguments("RS41", 100000, 403000000, 403000000)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(bin, name), "--json", "--auto", "--ptu", filepath.Join(root, "vendor", "RS", "rs41", "wav", "20140717_402MHz.wav"))
	cmd.SysProcAttr = hiddenProcessAttributes()
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, line := range bytes.Split(out, []byte{'\n'}) {
		if e, ok := ParseEvent(line); ok {
			count++
			if e.ID != "K1930308" || e.Lat == nil {
				t.Fatalf("unexpected RS41: %+v", e)
			}
		}
	}
	if count < 100 {
		t.Fatalf("too few RS41 records: %d", count)
	}
	t.Logf("RS41 WAV: %d frames", count)
	d := New(100000, bin)
	defer d.Close()
	for _, family := range Families {
		d.Configure(true, family, 403000000, 403000000)
		if !d.Snapshot().Running {
			t.Fatalf("cannot start %s: %+v", family, d.Snapshot())
		}
		d.ProcessIQ(make([]float32, 16000))
		time.Sleep(50 * time.Millisecond)
		d.mu.RLock()
		s := d.current
		d.mu.RUnlock()
		_ = s.cmd.Process.Kill()
		select {
		case <-s.done:
		case <-time.After(3 * time.Second):
			t.Fatal("process exit did not stop workers")
		}
		select {
		case <-s.writerDone:
		default:
			t.Fatal("writer leaked after process exit")
		}
		if d.Snapshot().Running || d.Snapshot().Error == "" {
			t.Fatalf("unexpected exit not reported for %s", family)
		}
		d.Close()
		d.Close()
	}
}
