package tetra

import (
	"fmt"
	"os"

	"github.com/ebitengine/purego"
)

type voiceDecoder struct {
	init      func()
	cdec      func(int32, *int16, *int16) int32
	sdec      func(*int16, *int16)
	setStolen func(int32)
	ready     bool
	errText   string
	firstPass int32
}

func newVoiceDecoder(path string) *voiceDecoder {
	v := &voiceDecoder{errText: "CODEC MISSING", firstPass: 1}
	if path == "" {
		return v
	}
	if _, err := os.Stat(path); err != nil {
		v.errText = fmt.Sprintf("CODEC MISSING (%v)", err)
		return v
	}
	lib, err := openCodecLibrary(path)
	if err != nil {
		v.errText = fmt.Sprintf("LOAD FAILED (%v)", err)
		return v
	}
	if err = codecSymbol(lib, "tetra_decode_init"); err != nil {
		v.errText = "INIT UNAVAILABLE"
		return v
	}
	if err = codecSymbol(lib, "tetra_cdec"); err != nil {
		v.errText = "CDEC UNAVAILABLE"
		return v
	}
	if err = codecSymbol(lib, "tetra_sdec"); err != nil {
		v.errText = "SDEC UNAVAILABLE"
		return v
	}
	purego.RegisterLibFunc(&v.init, lib, "tetra_decode_init")
	purego.RegisterLibFunc(&v.cdec, lib, "tetra_cdec")
	purego.RegisterLibFunc(&v.sdec, lib, "tetra_sdec")
	if err = codecSymbol(lib, "tetra_set_frame_stealing"); err == nil {
		purego.RegisterLibFunc(&v.setStolen, lib, "tetra_set_frame_stealing")
	}
	v.init()
	v.ready, v.errText = true, ""
	return v
}

func (v *voiceDecoder) reset() {
	if v == nil || !v.ready {
		return
	}
	v.init()
	v.firstPass = 1
}

func (v *voiceDecoder) decode(type4 []byte, stolen bool) ([]float32, bool) {
	if !v.ready || len(type4) != 432 {
		return nil, false
	}
	var input [690]int16
	for i := 0; i < 6; i++ {
		input[115*i] = int16(0x6b21 + i)
	}
	soft := func(bit byte) int16 {
		if bit != 0 {
			return -127
		}
		return 127
	}
	for i := 0; i < 114; i++ {
		input[1+i] = soft(type4[i])
		input[116+i] = soft(type4[114+i])
		input[231+i] = soft(type4[228+i])
	}
	for i := 0; i < 90; i++ {
		input[346+i] = soft(type4[342+i])
	}
	var serial [276]int16
	if v.setStolen != nil {
		if stolen {
			v.setStolen(1)
		} else {
			v.setStolen(0)
		}
	}
	if v.cdec(v.firstPass, &input[0], &serial[0]) != 0 {
		return nil, false
	}
	v.firstPass = 0
	if serial[0] != 0 && serial[138] != 0 {
		return nil, false
	}
	var pcm8 [480]int16
	v.sdec(&serial[0], &pcm8[0])
	pcm48 := make([]float32, len(pcm8)*6)
	for i, sample := range pcm8 {
		value := float32(sample) / 32768
		for n := 0; n < 6; n++ {
			pcm48[i*6+n] = value
		}
	}
	return pcm48, true
}
