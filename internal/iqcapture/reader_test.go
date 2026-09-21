package iqcapture

import (
	"io"
	"path/filepath"
	"testing"
)

func TestWAVReaderReplaysWriterOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.wav")
	writer, err := New(path, 100_000_000, 100_000_000, Rate)
	if err != nil {
		t.Fatal(err)
	}
	input := make([]float32, 4096*2)
	for index := range input {
		input[index] = .25
	}
	writer.Write(input)
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	reader, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if reader.SampleRate() != Rate {
		t.Fatalf("sample rate = %d", reader.SampleRate())
	}
	var count int
	for {
		frames, readErr := reader.ReadFrames(257)
		count += len(frames) / 2
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			t.Fatal(readErr)
		}
	}
	if count != 4096 {
		t.Fatalf("frames = %d; want 4096", count)
	}
}
