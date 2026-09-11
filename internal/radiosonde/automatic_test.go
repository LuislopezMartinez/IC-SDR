package radiosonde

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAutomaticConfirmation(t *testing.T) {
	d := New(100000, "")
	d.detections = make(map[string]autoDetection)
	e, _ := ParseEvent([]byte(`{"type":"RS41","id":"test","frame":1,"lat":40,"lon":-3,"alt":1000}`))
	d.acceptAutomatic(0, "DFM", e)
	if len(d.events) != 0 {
		t.Fatal("accepted wrong decoder family")
	}
	d.acceptAutomatic(0, "RS41", e)
	d.acceptAutomatic(0, "RS41", e)
	if d.detections["RS41"].count != 1 {
		t.Fatal("duplicate frame confirmed family")
	}
	e.Frame = 2
	d.acceptAutomatic(0, "RS41", e)
	if d.detections["RS41"].count != 2 {
		t.Fatal("two distinct frames did not confirm family")
	}
	d.detections["RS41"] = autoDetection{id: e.ID, frame: 2, count: 2, last: time.Now().Add(-16 * time.Second)}
	e.Frame = 3
	d.acceptAutomatic(0, "RS41", e)
	if d.detections["RS41"].count != 1 {
		t.Fatal("expired confirmation reused")
	}
	before := len(d.events)
	d.generation++
	d.acceptAutomatic(0, "RS41", e)
	if len(d.events) != before {
		t.Fatal("old session published frame")
	}
}

func TestAutomaticMissingRuntime(t *testing.T) {
	d := New(100000, t.TempDir())
	defer d.Close()
	d.Configure(true, "AUTO", 403000000, 403000000)
	s := d.Snapshot()
	if s.Running || s.State != "ERROR" || !strings.Contains(s.Error, "RS41") || !strings.Contains(s.Error, "DFM") || !strings.Contains(s.Error, "M10/M20") {
		t.Fatalf("missing candidates: %+v", s)
	}
	d.Close()
	d.Close()
}

func TestNativeAutomatic(t *testing.T) {
	root := os.Getenv("RADIOSONDE_INTEGRATION_ROOT")
	if root == "" {
		t.Skip("native assets not requested")
	}
	wav, err := os.ReadFile(filepath.Join(root, "vendor", "RS", "iq", "dfmIQ.wav"))
	if err != nil {
		t.Fatal(err)
	}
	var pcm []byte
	for off := 12; off+8 < len(wav); {
		n := int(binary.LittleEndian.Uint32(wav[off+4:]))
		if off+8+n > len(wav) {
			t.Fatal("bad WAV")
		}
		if string(wav[off:off+4]) == "data" {
			pcm = wav[off+8 : off+8+n]
			break
		}
		off += 8 + n + n%2
	}
	if len(pcm) == 0 {
		t.Fatal("no samples")
	}
	d := New(2_000_000, filepath.Join(root, "runtime", "bin"))
	defer d.Close()
	d.Configure(true, "AUTO", 402990000, 403000000)
	if s := d.Snapshot(); !s.Running || s.Error != "" {
		t.Fatalf("auto start: %+v", s)
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
	s := d.Snapshot()
	if s.Events < 8 || s.Dropped != 0 || !strings.Contains(s.State, "DFM") {
		t.Fatalf("automatic detection failed: %+v", s)
	}
	t.Logf("AUTO at 2 MS/s: %+v", s)
	for _, e := range d.Events() {
		if e.Type != "DFM" {
			t.Fatalf("false positive: %s", e.Type)
		}
	}
	d.Configure(true, "AUTO", 403010000, 403000000)
	if s := d.Snapshot(); s.State != "AUTO · SEARCHING" {
		t.Fatalf("retune kept detection: %+v", s)
	}
	d.Configure(true, "RS41", 403000000, 403000000)
	if len(d.children) != 0 {
		t.Fatal("manual mode left automatic workers")
	}
	d.Close()
	if d.Snapshot().Running {
		t.Fatal("still running")
	}
}
