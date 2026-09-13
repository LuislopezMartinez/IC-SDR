package screens

import (
	"encoding/binary"
	"errors"
)

// oggOpusStream packages independently encoded Opus packets into a live Ogg
// logical stream. Each web client gets its own serial/sequence and headers.
type oggOpusStream struct {
	serial   uint32
	sequence uint32
	granule  uint64
	preSkip  uint16
}

func (stream *oggOpusStream) headers() ([]byte, []byte) {
	head := make([]byte, 19)
	copy(head, "OpusHead")
	head[8] = 1
	head[9] = 1
	binary.LittleEndian.PutUint16(head[10:], stream.preSkip)
	binary.LittleEndian.PutUint32(head[12:], 48000)
	tags := make([]byte, 8+4+len("GO-Zero")+4)
	copy(tags, "OpusTags")
	binary.LittleEndian.PutUint32(tags[8:], uint32(len("GO-Zero")))
	copy(tags[12:], "GO-Zero")
	return stream.page(head, 0x02, 0), stream.page(tags, 0, 0)
}

func (stream *oggOpusStream) audio(packet []byte) []byte {
	stream.granule += 960
	return stream.page(packet, 0, stream.granule)
}

func (stream *oggOpusStream) page(packet []byte, flags byte, granule uint64) []byte {
	segments := len(packet)/255 + 1
	if segments > 255 {
		panic(errors.New("Ogg packet too large"))
	}
	page := make([]byte, 27+segments+len(packet))
	copy(page, "OggS")
	page[4] = 0
	page[5] = flags
	binary.LittleEndian.PutUint64(page[6:], granule)
	binary.LittleEndian.PutUint32(page[14:], stream.serial)
	binary.LittleEndian.PutUint32(page[18:], stream.sequence)
	page[26] = byte(segments)
	for i := 0; i < segments; i++ {
		remaining := len(packet) - i*255
		if remaining > 255 {
			remaining = 255
		}
		page[27+i] = byte(remaining)
	}
	copy(page[27+segments:], packet)
	binary.LittleEndian.PutUint32(page[22:], oggCRC(page))
	stream.sequence++
	return page
}

func oggCRC(data []byte) uint32 {
	var crc uint32
	for _, value := range data {
		crc ^= uint32(value) << 24
		for i := 0; i < 8; i++ {
			if crc&0x80000000 != 0 {
				crc = (crc << 1) ^ 0x04c11db7
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}
