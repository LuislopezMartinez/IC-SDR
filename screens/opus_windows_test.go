//go:build windows

package screens

import (
	"encoding/binary"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"go-zero/internal/resources"
)

func TestBundledOpusEncoder(t *testing.T) {
	if _, err := os.Stat(resources.Path("tools", "digital_voice", "runtime", "bin", "opus.dll")); err != nil {
		t.Skip("Opus runtime not installed")
	}
	encoder, err := newOpusEncoder()
	if err != nil {
		t.Fatal(err)
	}
	defer encoder.Close()
	packet, err := encoder.Encode(make([]float32, 960))
	if err != nil || len(packet) == 0 {
		t.Fatalf("Opus encoding failed: %v, bytes=%d", err, len(packet))
	}
}

func TestLiveOggOpusHTTP(t *testing.T) {
	if _, err := os.Stat(resources.Path("tools", "digital_voice", "runtime", "bin", "opus.dll")); err != nil {
		t.Skip("Opus runtime not installed")
	}
	screen := &MainScreen{audioPlayer: NewAudioPlayer(nil, nil)}
	server, err := StartWebServer(screen, "127.0.0.1:0", "test-password")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	client := &http.Client{Timeout: 5 * time.Second}
	request, _ := http.NewRequest(http.MethodGet, "http://"+server.listener.Addr().String()+"/api/audio/opus", nil)
	request.SetBasicAuth("viewer", "test-password")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status %d", response.StatusCode)
	}
	if got := response.Header.Get("Content-Type"); got != `audio/ogg; codecs="opus"` {
		t.Fatalf("content type %q", got)
	}
	for _, expected := range []string{"OpusHead", "OpusTags"} {
		page := readOggPage(t, response.Body)
		if string(page[28:36]) != expected {
			t.Fatalf("wanted %s", expected)
		}
	}
	server.PublishAudio(make([]float32, 960))
	page := readOggPage(t, response.Body)
	if binary.LittleEndian.Uint64(page[6:]) != 960 {
		t.Fatal("missing live audio packet")
	}
}

func readOggPage(t *testing.T, reader io.Reader) []byte {
	t.Helper()
	header := make([]byte, 27)
	if _, err := io.ReadFull(reader, header); err != nil {
		t.Fatal(err)
	}
	segments := make([]byte, int(header[26]))
	if _, err := io.ReadFull(reader, segments); err != nil {
		t.Fatal(err)
	}
	length := 0
	for _, size := range segments {
		length += int(size)
	}
	packet := make([]byte, length)
	if _, err := io.ReadFull(reader, packet); err != nil {
		t.Fatal(err)
	}
	return append(append(header, segments...), packet...)
}
