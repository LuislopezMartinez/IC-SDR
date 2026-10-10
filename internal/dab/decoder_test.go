package dab

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestDecodePCMFloat32DownmixesStereo(t *testing.T) {
	values := []float32{.75, .25, -1, .5}
	data := make([]byte, len(values)*4)
	for i, value := range values {
		binary.LittleEndian.PutUint32(data[i*4:], math.Float32bits(value))
	}
	got := decodePCMFloat32(data, 2)
	if len(got) != 2 || math.Abs(float64(got[0]-.5)) > 1e-6 || math.Abs(float64(got[1]+.25)) > 1e-6 {
		t.Fatalf("downmix = %v", got)
	}
}

func TestMissingRuntimeIsReportedWithoutStartingProcess(t *testing.T) {
	d := New(2_048_000, t.TempDir()+"/missing.exe", nil)
	if d.Snapshot().Available {
		t.Fatal("missing runtime reported as available")
	}
	if err := d.Configure(true, 0); err == nil {
		t.Fatal("expected missing runtime error")
	}
}

func TestServiceEventsAreCollectedWithoutDuplicates(t *testing.T) {
	d := New(2_048_000, t.TempDir()+"/missing.exe", nil)
	d.applyEvent(runtimeEvent{Type: "service", ServiceID: 0x1234, Service: "Radio Uno"}, 0)
	d.applyEvent(runtimeEvent{Type: "service", ServiceID: 0x1234, Service: "Radio Uno HD"}, 0)
	status := d.Snapshot()
	if len(status.Services) != 1 || status.Services[0].Name != "Radio Uno HD" {
		t.Fatalf("services = %+v", status.Services)
	}
}

func TestLateServiceLabelUpdatesSelectedService(t *testing.T) {
	d := New(2_048_000, t.TempDir()+"/missing.exe", nil)
	d.applyEvent(runtimeEvent{Type: "selected", ServiceID: 0x5678}, 0)
	d.applyEvent(runtimeEvent{Type: "service", ServiceID: 0x5678, Service: "Radio Barcelona"}, 0)
	status := d.Snapshot()
	if status.Service != "Radio Barcelona" || status.ServiceID != 0x5678 {
		t.Fatalf("selected service = %q/%X", status.Service, status.ServiceID)
	}
}

func TestStreamResamplerConverts32kTo48kAcrossBlocks(t *testing.T) {
	r := streamResampler{outputRate: 48_000}
	var output []float32
	for block := 0; block < 10; block++ {
		input := make([]float32, 320)
		for index := range input {
			input[index] = float32(block*len(input) + index)
		}
		output = append(output, r.Process(input, 32_000)...)
	}
	if len(output) < 4798 || len(output) > 4800 {
		t.Fatalf("resampled length = %d, want approximately 4800", len(output))
	}
	for index := 1; index < len(output); index++ {
		if output[index] < output[index-1] {
			t.Fatalf("resampler discontinuity at %d: %f < %f", index, output[index], output[index-1])
		}
	}
}
