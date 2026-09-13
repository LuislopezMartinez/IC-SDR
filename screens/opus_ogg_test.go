package screens

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestOggOpusLivePages(t *testing.T) {
	stream := oggOpusStream{serial: 1234, preSkip: 312}
	head, tags := stream.headers()
	packet := bytes.Repeat([]byte{0xab}, 300)
	first, second := stream.audio(packet), stream.audio(packet)
	for index, page := range [][]byte{head, tags, first, second} {
		if string(page[:4]) != "OggS" {
			t.Fatalf("page %d: missing capture pattern", index)
		}
		if got := binary.LittleEndian.Uint32(page[18:]); got != uint32(index) {
			t.Fatalf("page %d: sequence %d", index, got)
		}
		crc := binary.LittleEndian.Uint32(page[22:])
		copyPage := append([]byte(nil), page...)
		clear(copyPage[22:26])
		if oggCRC(copyPage) != crc {
			t.Fatalf("page %d: invalid CRC", index)
		}
	}
	if head[5] != 2 || string(head[28:36]) != "OpusHead" {
		t.Fatal("invalid OpusHead")
	}
	if string(tags[28:36]) != "OpusTags" {
		t.Fatal("invalid OpusTags")
	}
	if got := binary.LittleEndian.Uint64(first[6:]); got != 960 {
		t.Fatalf("first granule: %d", got)
	}
	if got := binary.LittleEndian.Uint64(second[6:]); got != 1920 {
		t.Fatalf("second granule: %d", got)
	}
	if first[26] != 2 || first[27] != 255 || first[28] != 45 {
		t.Fatal("incorrect packet lacing")
	}
}
