//go:build windows

package screens

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"

	"github.com/ebitengine/purego"
	"go-zero/internal/resources"
)

const opusApplicationAudio = 2049

type opusEncoder struct {
	library syscall.Handle
	state   uintptr
	create  func(int32, int32, int32, *int32) uintptr
	destroy func(uintptr)
	encode  func(uintptr, *float32, int32, *byte, int32) int32
	ctlSet  func(uintptr, int32, int32) int32
	ctlGet  func(uintptr, int32, *int32) int32
	preSkip uint16
}

func newOpusEncoder() (*opusEncoder, error) {
	path := resources.Path("tools", "digital_voice", "runtime", "bin", "opus.dll")
	library, err := syscall.LoadLibrary(path)
	if err != nil {
		return nil, fmt.Errorf("cargar opus.dll: %w", err)
	}
	e := &opusEncoder{library: library}
	defer func() {
		if err != nil {
			e.Close()
		}
	}()
	purego.RegisterLibFunc(&e.create, uintptr(library), "opus_encoder_create")
	purego.RegisterLibFunc(&e.destroy, uintptr(library), "opus_encoder_destroy")
	purego.RegisterLibFunc(&e.encode, uintptr(library), "opus_encode_float")
	purego.RegisterLibFunc(&e.ctlSet, uintptr(library), "opus_encoder_ctl")
	purego.RegisterLibFunc(&e.ctlGet, uintptr(library), "opus_encoder_ctl")
	var code int32
	e.state = e.create(48000, 1, opusApplicationAudio, &code)
	if e.state == 0 || code != 0 {
		err = fmt.Errorf("crear codificador Opus: %d", code)
		return nil, err
	}
	if result := e.ctlSet(e.state, 4002, 24000); result != 0 {
		err = fmt.Errorf("configurar bitrate Opus: %d", result)
		return nil, err
	}
	var lookahead int32
	if result := e.ctlGet(e.state, 4027, &lookahead); result != 0 || lookahead < 0 || lookahead > 65535 {
		err = errors.New("no se pudo consultar el retardo Opus")
		return nil, err
	}
	e.preSkip = uint16(lookahead)
	return e, nil
}

func (e *opusEncoder) Encode(samples []float32) ([]byte, error) {
	if len(samples) != 960 {
		return nil, errors.New("Opus requiere tramas de 20 ms")
	}
	packet := make([]byte, 4000)
	length := e.encode(e.state, &samples[0], 960, &packet[0], int32(len(packet)))
	runtime.KeepAlive(samples)
	if length < 0 {
		return nil, fmt.Errorf("opus_encode_float: %d", length)
	}
	return packet[:length], nil
}

func (e *opusEncoder) Close() {
	if e.state != 0 {
		e.destroy(e.state)
		e.state = 0
	}
	if e.library != 0 {
		_ = syscall.FreeLibrary(e.library)
		e.library = 0
	}
}
