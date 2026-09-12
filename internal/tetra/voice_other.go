//go:build !windows

package tetra

type voiceDecoder struct {
	ready   bool
	errText string
}

func newVoiceDecoder(string) *voiceDecoder {
	return &voiceDecoder{errText: "CODEC UNAVAILABLE"}
}

func (*voiceDecoder) reset() {}

func (*voiceDecoder) decode([]byte, bool) ([]float32, bool) {
	return nil, false
}
