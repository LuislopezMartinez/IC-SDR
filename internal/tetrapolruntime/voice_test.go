package tetrapolruntime

import (
	"strings"
	"testing"
)

func TestVoiceBitsUsesLeastSignificantBitFirstOrder(t *testing.T) {
	bits, err := VoiceBits("010000000000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	if len(bits) != 120 || !strings.HasPrefix(bits, "10000000") {
		t.Fatalf("unexpected voice bits %q", bits)
	}
}

func TestVoiceBitsFromKitJSONFiltersNonVoiceFrames(t *testing.T) {
	line := []byte(`{"event":"frame","frame":{"state":"ok","type":"VOICE","data":{"encoding":"hex","value":"010000000000000000000000000000"}}}`)
	bits, accepted, err := VoiceBitsFromKitJSON(line)
	if err != nil || !accepted || len(bits) != 120 {
		t.Fatalf("bits=%q accepted=%v err=%v", bits, accepted, err)
	}
	_, accepted, err = VoiceBitsFromKitJSON([]byte(`{"event":"frame","frame":{"state":"bad_CRC"}}`))
	if err != nil || accepted {
		t.Fatalf("bad frame accepted=%v err=%v", accepted, err)
	}
}

func TestPCM16LEToFloat32(t *testing.T) {
	got := PCM16LEToFloat32([]byte{0, 0, 0xff, 0x7f, 0, 0x80})
	if len(got) != 3 || got[0] != 0 || got[1] < .999 || got[2] != -1 {
		t.Fatalf("samples=%v", got)
	}
}
