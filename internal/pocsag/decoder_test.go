package pocsag

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseEvent(t *testing.T) {
	e, ok := ParseEvent([]byte(`{"demod_name":"POCSAG1200","address":1234567,"function":3,"alpha":"PRUEBA"}`))
	if !ok || e.Address != 1234567 || e.Function != 3 || e.Baud != 1200 || e.Kind != "ALFA" || e.Message != "PRUEBA" {
		t.Fatalf("unexpected event: %+v, %v", e, ok)
	}
}

func TestReaderPrefersAlphaAutoCandidate(t *testing.T) {
	d := New("")
	d.generation = 1
	d.reader(strings.NewReader(
		"{\"demod_name\":\"POCSAG1200\",\"address\":1234567,\"function\":3,\"numeric\":\"05965\"}\n"+
			"{\"demod_name\":\"POCSAG1200\",\"address\":1234567,\"function\":3,\"alpha\":\"PRUEBA<NUL>\"}\n"), 1)
	status := d.Snapshot()
	if len(status.Events) != 1 || status.Events[0].Kind != "ALFA" || status.Events[0].Message != "PRUEBA" {
		t.Fatalf("auto candidates were not coalesced: %+v", status.Events)
	}
}

func TestHistoryRestoresNewestFirstAndFrequency(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pocsag.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []Event{{Address: 1, FrequencyHz: 169625000}, {Address: 2, FrequencyHz: 466075000}} {
		data, _ := json.Marshal(event)
		_, _ = f.Write(append(data, '\n'))
	}
	_ = f.Close()
	events := New("", path).Snapshot().Events
	if len(events) != 2 || events[0].Address != 2 || events[0].FrequencyHz != 466075000 {
		t.Fatalf("unexpected restored history: %+v", events)
	}
}

func TestResamplerMaintainsRateAcrossChunks(t *testing.T) {
	r := newResampler(InputRate, OutputRate)
	total := 0
	for n := 0; n < 100; n++ {
		input := make([]float32, 480)
		for i := range input {
			input[i] = float32(math.Sin(float64(n*len(input)+i) * .01))
		}
		total += len(r.process(input))
	}
	if total < 22045 || total > 22055 {
		t.Fatalf("unexpected output sample count: %d", total)
	}
}
