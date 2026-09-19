package tetra

import (
	"go-zero/internal/i18n"

	"fmt"
	"syscall"
	"unsafe"
)

type voiceDecoder struct {
	init, cdec, sdec *syscall.LazyProc
	ready            bool
	errText          string
	firstPass        uintptr
	leveler          voiceLeveler
}

func newVoiceDecoder(path string) *voiceDecoder {
	v := &voiceDecoder{errText: i18n.Source("text.85a835b4fee4"), firstPass: 1}
	if path == "" {
		return v
	}
	dll := syscall.NewLazyDLL(path)
	if err := dll.Load(); err != nil {
		v.errText = fmt.Sprintf(i18n.Source("text.acd49dd016f6"), err)
		return v
	}
	v.init, v.cdec, v.sdec = dll.NewProc("tetra_decode_init"), dll.NewProc("tetra_cdec"), dll.NewProc("tetra_sdec")
	if err := v.init.Find(); err != nil {
		v.errText = i18n.Source("text.2b2bd3ae40cb")
		return v
	}
	if err := v.cdec.Find(); err != nil {
		v.errText = i18n.Source("text.eacd3226af8f")
		return v
	}
	if err := v.sdec.Find(); err != nil {
		v.errText = i18n.Source("text.dd1c3496eb2a")
		return v
	}
	v.init.Call()
	v.ready, v.errText = true, ""
	return v
}

func (v *voiceDecoder) reset() {
	if v == nil || !v.ready {
		return
	}
	v.init.Call()
	v.firstPass = 1
	v.leveler.reset()
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
	stolenArg := uintptr(0)
	if stolen {
		stolenArg = 1
	}
	r, _, _ := v.cdec.Call(v.firstPass, uintptr(unsafe.Pointer(&input[0])), uintptr(unsafe.Pointer(&serial[0])), stolenArg)
	if r != 0 {
		return nil, false
	}
	v.firstPass = 0
	if serial[0] != 0 && serial[138] != 0 {
		return nil, false
	}
	var pcm8 [480]int16
	v.sdec.Call(uintptr(unsafe.Pointer(&serial[0])), uintptr(unsafe.Pointer(&pcm8[0])))
	return v.leveler.convert(pcm8[:]), true
}
