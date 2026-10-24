//go:build windows

package pocsag

import (
	"encoding/binary"
	"os"
	"testing"
	"time"
)

// Set POCSAG_RUNTIME and POCSAG_FIXTURE_RAW to validate multimon-ng end to end.
// The fixture must be signed PCM16 mono at 22.050 kHz.
func TestRuntimeFixture(t *testing.T) {
	executable, fixture := os.Getenv("POCSAG_RUNTIME"), os.Getenv("POCSAG_FIXTURE_RAW")
	if executable == "" || fixture == "" {
		t.Skip("POCSAG_RUNTIME and POCSAG_FIXTURE_RAW are not set")
	}
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	source := make([]float32, len(data)/2)
	for index := range source {
		source[index] = float32(int16(binary.LittleEndian.Uint16(data[index*2:]))) / 32768
	}
	// Linear interpolation recreates the application's 48 kHz discriminator
	// stream; Decoder then exercises its production 22.050 kHz conversion.
	count := int(float64(len(source)) * InputRate / OutputRate)
	input := make([]float32, count)
	for index := range input {
		position := float64(index) * OutputRate / InputRate
		left := int(position)
		if left+1 >= len(source) {
			input[index] = source[len(source)-1]
			continue
		}
		fraction := float32(position - float64(left))
		input[index] = source[left] + (source[left+1]-source[left])*fraction
	}
	d := New(executable)
	d.Configure(true)
	defer d.Close()
	for offset := 0; offset < len(input); offset += 480 {
		end := min(offset+480, len(input))
		d.ProcessAudio(input[offset:end])
		time.Sleep(time.Millisecond)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if len(d.Snapshot().Events) > 0 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("runtime produced no POCSAG events: %+v", d.Snapshot())
}
