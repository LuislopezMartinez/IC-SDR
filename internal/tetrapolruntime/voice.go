// Package tetrapolruntime bridges decoded TETRAPOL VOICE frames to an external
// RP-CELP decoder. The upstream protocol layer must identify clear frames.
package tetrapolruntime

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const VoiceFrameBytes = 15

// VoiceBits converts the 15-byte, least-significant-bit-first value emitted
// by tetrapol-kit into the 120-character representation expected by rpcelp.
func VoiceBits(value string) (string, error) {
	frame, err := hex.DecodeString(value)
	if err != nil {
		return "", fmt.Errorf("decode TETRAPOL voice frame: %w", err)
	}
	if len(frame) != VoiceFrameBytes {
		return "", fmt.Errorf("TETRAPOL voice frame is %d bytes, want %d", len(frame), VoiceFrameBytes)
	}
	bits := make([]byte, 0, VoiceFrameBytes*8)
	for _, octet := range frame {
		for bit := 0; bit < 8; bit++ {
			bits = append(bits, '0'+((octet>>bit)&1))
		}
	}
	return string(bits), nil
}

type kitFrame struct {
	Event string `json:"event"`
	Frame struct {
		State string `json:"state"`
		Type  string `json:"type"`
		Data  struct {
			Encoding string `json:"encoding"`
			Value    string `json:"value"`
		} `json:"data"`
	} `json:"frame"`
}

// VoiceBitsFromKitJSON accepts one JSON record produced by a tetrapol-kit
// build with frame_json enabled. It filters malformed and non-VOICE records;
// that record does not itself include an encryption-state field.
func VoiceBitsFromKitJSON(line []byte) (string, bool, error) {
	var record kitFrame
	if err := json.Unmarshal(line, &record); err != nil {
		return "", false, fmt.Errorf("parse tetrapol-kit record: %w", err)
	}
	if record.Event != "frame" || record.Frame.State != "ok" || record.Frame.Type != "VOICE" || record.Frame.Data.Encoding != "hex" {
		return "", false, nil
	}
	bits, err := VoiceBits(record.Frame.Data.Value)
	if err != nil {
		return "", false, err
	}
	return bits, true, nil
}

// PCM16LEToFloat32 converts RP-CELP's signed 16-bit mono output to the
// receiver's normalized audio convention.
func PCM16LEToFloat32(pcm []byte) []float32 {
	samples := make([]float32, len(pcm)/2)
	for index := range samples {
		value := int16(uint16(pcm[index*2]) | uint16(pcm[index*2+1])<<8)
		samples[index] = float32(value) / 32768
	}
	return samples
}
