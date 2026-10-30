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

func TestParseFLEXEvent(t *testing.T) {
	e, ok := ParseEvent([]byte(`{"demod_name":"flex_alphanumeric","capcode":1120103,"sync_baud":1600,"sync_level":2,"phase_number":"A","cycle_number":9,"frame_number":49,"message":"PRUEBA FLEX"}`))
	if !ok || e.Protocol != ProtocolFLEX || e.Address != 1120103 || e.Baud != 1600 || e.Kind != "ALFA" || e.Message != "PRUEBA FLEX" || e.Phase != "A" || e.Cycle != 9 || e.Frame != 49 {
		t.Fatalf("unexpected FLEX event: %+v, %v", e, ok)
	}
}

func TestDecoderArgumentsSelectOnePagerFamily(t *testing.T) {
	pocsagArgs := strings.Join(decoderArguments(ProtocolPOCSAG), " ")
	flexArgs := strings.Join(decoderArguments(ProtocolFLEX), " ")
	if !strings.Contains(pocsagArgs, "POCSAG1200") || strings.Contains(pocsagArgs, "FLEX") {
		t.Fatalf("unexpected POCSAG arguments: %s", pocsagArgs)
	}
	if !strings.Contains(flexArgs, "-a FLEX") || !strings.Contains(flexArgs, "-a FLEX_NEXT") || strings.Contains(flexArgs, "POCSAG") {
		t.Fatalf("unexpected FLEX arguments: %s", flexArgs)
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

func TestClearProtocolPreservesOtherPagerHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paging.jsonl")
	d := New("", path)
	d.events = []Event{{Address: 1, Protocol: ProtocolFLEX}, {Address: 2, Protocol: ProtocolPOCSAG}}
	d.ClearProtocol(ProtocolFLEX)
	events := d.Snapshot().Events
	if len(events) != 1 || events[0].Protocol != ProtocolPOCSAG || events[0].Address != 2 {
		t.Fatalf("clearing FLEX changed POCSAG history: %+v", events)
	}
	restored := New("", path).Snapshot().Events
	if len(restored) != 1 || restored[0].Protocol != ProtocolPOCSAG {
		t.Fatalf("persisted protocol history is incorrect: %+v", restored)
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
