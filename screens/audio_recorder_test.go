package screens

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAudioRecorderWritesValidWAV(t *testing.T) {
	recorder := newAudioRecorder(t.TempDir())
	recorder.SetFormat(recorderFormatWAV)
	recorder.Configure(446_093_750, "PMR446", "NFM")
	recorder.Start()
	recorder.Submit([]float32{0, .25, -.25, .5, -.5}, false, true)
	recorder.Stop()
	deadline := time.Now().Add(time.Second)
	for len(recorder.State().RecentFiles) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	state := recorder.State()
	if len(state.RecentFiles) != 1 {
		recorder.Close()
		t.Fatal("recording was not finalized")
	}
	data, err := os.ReadFile(state.RecentFiles[0])
	if err != nil {
		recorder.Close()
		t.Fatal(err)
	}
	if len(data) != 44+10 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		recorder.Close()
		t.Fatalf("invalid WAV: bytes=%d", len(data))
	}
	if got := binary.LittleEndian.Uint32(data[24:28]); got != audioSampleRate {
		recorder.Close()
		t.Fatalf("sample rate = %d", got)
	}
	if err := recorder.DeleteFile(state.RecentFiles[0]); err != nil {
		recorder.Close()
		t.Fatalf("delete recording: %v", err)
	}
	if _, err := os.Stat(state.RecentFiles[0]); !os.IsNotExist(err) {
		recorder.Close()
		t.Fatalf("recording still exists after delete: %v", err)
	}
	if len(recorder.State().RecentFiles) != 0 {
		recorder.Close()
		t.Fatal("deleted recording remains in recent list")
	}
	recorder.Close()
}

func TestAudioRecorderWritesMP3WithBundledLAME(t *testing.T) {
	recorder := newAudioRecorder(t.TempDir())
	recorder.SetFormat(recorderFormatMP3)
	recorder.Configure(145_800_000, "SAT", "FM")
	recorder.Start()
	samples := make([]float32, audioSampleRate*2)
	for i := range samples {
		samples[i] = float32(.4 * math.Sin(2*math.Pi*1000*float64(i)/audioSampleRate))
	}
	recorder.Submit(samples, false, true)
	recorder.Stop()
	deadline := time.Now().Add(3 * time.Second)
	for len(recorder.State().RecentFiles) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	state := recorder.State()
	if len(state.RecentFiles) != 1 {
		recorder.Close()
		t.Fatal("MP3 recording was not finalized")
	}
	path := state.RecentFiles[0]
	data, err := os.ReadFile(path)
	if err != nil {
		recorder.Close()
		t.Fatal(err)
	}
	if filepath.Ext(path) != ".mp3" || len(data) < 4 || data[0] != 0xff || data[1]&0xe0 != 0xe0 {
		recorder.Close()
		t.Fatalf("invalid MP3 output %q (%d bytes)", path, len(data))
	}
	frames, duration, err := inspectMP3Frames(data)
	if err != nil {
		recorder.Close()
		t.Fatalf("corrupt MP3 stream: %v", err)
	}
	if frames < 80 || duration < 2*time.Second || duration > 2100*time.Millisecond {
		recorder.Close()
		t.Fatalf("MP3 frames=%d duration=%v, want approximately 2 seconds", frames, duration)
	}
	if matches, _ := filepath.Glob(path + ".wav.part"); len(matches) != 0 {
		recorder.Close()
		t.Fatal("temporary WAV was not removed")
	}
	recorder.Close()
}

func inspectMP3Frames(data []byte) (frames int, duration time.Duration, err error) {
	for len(data) > 0 {
		length, frameErr := mp3FrameBytes(data)
		if frameErr != nil {
			return frames, duration, frameErr
		}
		header := binary.BigEndian.Uint32(data[:4])
		version := (header >> 19) & 3
		samples := 1152
		if version != 3 {
			samples = 576
		}
		sampleRateIndex := (header >> 10) & 3
		rates := map[uint32][3]int{3: {44100, 48000, 32000}, 2: {22050, 24000, 16000}, 0: {11025, 12000, 8000}}
		duration += time.Duration(samples) * time.Second / time.Duration(rates[version][sampleRateIndex])
		frames++
		data = data[length:]
	}
	return frames, duration, nil
}

func TestAudioRecorderCanSkipClosedSquelch(t *testing.T) {
	recorder := newAudioRecorder(t.TempDir())
	recorder.SetSkipSquelchSilence(true)
	recorder.Start()
	recorder.Submit(make([]float32, 100), true, false)
	state := recorder.State()
	if state.DurationSeconds != 0 || !state.WaitingForSquelch {
		recorder.Close()
		t.Fatalf("closed squelch state = %#v", state)
	}
	recorder.Close()
}
