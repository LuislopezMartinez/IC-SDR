//go:build windows

package tetrapolruntime

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/ebitengine/purego"
)

type nativeAPI struct {
	library           syscall.Handle
	callback          uintptr
	create            func(int) uintptr
	createConfigured  func(int, int, int) uintptr
	destroy           func(uintptr)
	setSampleRate     func(uintptr, float64)
	process           func(uintptr, []float32, int)
	setCallback       func(uintptr, uintptr, uintptr)
	getDiagBasic      func(uintptr, *nativeBasicDiag) int
	getSyncState      func(uintptr) int
	getChannelMode    func(uintptr) int
	getTCHSession     func(uintptr) int
	getAudioStats     func(uintptr, *int64, *int64, *int32) int
	getCCHSnapshot    func(uintptr, *nativeCCHSnapshot) int
	setCCHCell        func(uintptr, string)
	setChannelID      func(uintptr, int)
	clearNetworkState func(uintptr)
}

type nativeBasicDiag struct {
	IQSamples, BitsOut, FramesOK, FramesBad int64
	FrameSync, SCR                          int32
	SampleRate, SamplesPerSymbol            float64
	AFCFrequencyHz, AFCErrorHz              float64
	AFCLocked, AFCMode, ChannelMode         int32
}

type nativeCCHSnapshot struct {
	CellID        [32]byte
	Network       [16]byte
	Valid         int32
	TSDUCount     int32
	LastCODOP     int32
	LastLogCH     int32
	Channel       int32
	GSSI          int32
	Encrypted     int32
	Active        int32
	AgeSeconds    int32
	ActivationSeq uint64
}

func loadNativeAPI(path string) (nativeAPI, error) {
	if path == "" {
		return nativeAPI{}, fmt.Errorf("liliPOL x64 DLL is not configured")
	}
	handle, err := syscall.LoadLibrary(path)
	if err != nil {
		return nativeAPI{}, fmt.Errorf("load liliPOL x64 DLL: %w", err)
	}
	api := nativeAPI{library: handle}
	defer func() {
		if err != nil {
			_ = syscall.FreeLibrary(handle)
		}
	}()
	register := func(target any, name string) error {
		address, findErr := syscall.GetProcAddress(handle, name)
		if findErr != nil {
			return fmt.Errorf("liliPOL x64 missing %s: %w", name, findErr)
		}
		purego.RegisterFunc(target, address)
		return nil
	}
	for _, item := range []struct {
		name   string
		target any
	}{
		{"tp_create", &api.create}, {"tp_create_configured", &api.createConfigured}, {"tp_destroy", &api.destroy},
		{"tp_set_sample_rate", &api.setSampleRate}, {"tp_process_iq", &api.process},
		{"tp_set_audio_callback", &api.setCallback}, {"tp_get_diag_basic", &api.getDiagBasic},
		{"tp_get_sync_state", &api.getSyncState}, {"tp_get_channel_mode", &api.getChannelMode},
		{"tp_get_tch_session_state", &api.getTCHSession},
		{"tp_get_audio_stats", &api.getAudioStats},
		{"tp_get_cch_snapshot", &api.getCCHSnapshot}, {"tp_set_cch_cell_id", &api.setCCHCell},
		{"tp_set_my_channel_id", &api.setChannelID},
		{"tp_clear_network_state", &api.clearNetworkState},
	} {
		if err = register(item.target, item.name); err != nil {
			return nativeAPI{}, err
		}
	}
	return api, nil
}

func (api *nativeAPI) setContext(handle uintptr, cellID string, channelID int) {
	if cellID != "" {
		api.setCCHCell(handle, cellID+"\x00")
	}
	if channelID >= 0 {
		api.setChannelID(handle, channelID)
	}
}

func (api *nativeAPI) close() {
	if api.library != 0 {
		_ = syscall.FreeLibrary(api.library)
		api.library = 0
	}
}

func (api *nativeAPI) setAudioCallback(handle uintptr, emit func([]float32)) {
	api.callback = purego.NewCallback(func(samples uintptr, count int, _ uintptr) uintptr {
		if samples == 0 || count <= 0 {
			return 0
		}
		emit(unsafe.Slice((*float32)(unsafe.Pointer(samples)), count))
		return 0
	})
	api.setCallback(handle, api.callback, 0)
}

func (api *nativeAPI) clearAudioCallback(handle uintptr) {
	api.setCallback(handle, 0, 0)
	api.callback = 0
}

func (api *nativeAPI) processIQ(handle uintptr, samples []float32) {
	if len(samples) < 2 {
		return
	}
	api.process(handle, samples, len(samples)/2)
}

func (api *nativeAPI) snapshot(handle uintptr) (Telemetry, error) {
	var diag nativeBasicDiag
	ok := api.getDiagBasic(handle, &diag)
	if ok == 0 {
		return Telemetry{}, fmt.Errorf("liliPOL x64 diagnostic query failed")
	}
	var decoded, plc int64
	var corDrop int32
	_ = api.getAudioStats(handle, &decoded, &plc, &corDrop)
	total := diag.FramesOK + diag.FramesBad
	afcMode := "ACQ"
	if diag.AFCMode != 0 {
		afcMode = "TRACK"
	}
	value := Telemetry{
		PhysicalSync:           diag.FrameSync != 0,
		IQSamples:              uint64(max(0, diag.IQSamples)),
		BitsOut:                uint64(max(0, diag.BitsOut)),
		ValidFrames:            uint64(max(0, diag.FramesOK)),
		CRCFailures:            uint64(max(0, diag.FramesBad)),
		SCR:                    int(diag.SCR),
		SCRKnown:               diag.SCR >= 0,
		AudioDecoded:           uint64(max(0, decoded)),
		AudioConcealed:         uint64(max(0, plc)),
		CorrelationDropPercent: int(max(0, corDrop)),
		TCHSessionState:        api.getTCHSession(handle),
		AFCLocked:              diag.AFCLocked != 0,
		AFCMode:                afcMode,
		AFCFrequencyHz:         diag.AFCFrequencyHz,
		AFCErrorHz:             diag.AFCErrorHz,
		NativeRate:             diag.SampleRate,
		NativeSPS:              diag.SamplesPerSymbol,
	}
	value.Synchronized = api.getSyncState(handle) != 0
	if total > 0 {
		value.QualityKnown = true
		value.QualityPercent = 100 * float64(diag.FramesOK) / float64(total)
		value.QualitySamples = uint64(total)
	}
	if api.getChannelMode(handle) == 0 {
		value.LogicalChannel = "CCH"
	} else {
		value.LogicalChannel = "TCH"
	}
	var cch nativeCCHSnapshot
	if api.getCCHSnapshot(handle, &cch) != 0 {
		value.CellID = cString(cch.CellID[:])
		value.Network = cString(cch.Network[:])
		value.TSDUCount = uint64(max(0, cch.TSDUCount))
		// A decoded TSDU is definitive evidence of at least one valid physical
		// frame. Keep the public counter useful even when the native rolling
		// frame-stat bucket was consumed between diagnostic polls.
		if value.ValidFrames < value.TSDUCount {
			value.ValidFrames = value.TSDUCount
		}
		value.LastCODOP, value.CODOPKnown = int(cch.LastCODOP), cch.TSDUCount > 0
		if cch.Active != 0 && cch.Channel >= 0 {
			value.ChannelAction = "follow"
			value.ChannelKind = "group"
			value.ChannelID = int(cch.Channel)
			value.ChannelEvent = cch.ActivationSeq
			if cch.Encrypted == 0 {
				value.CipherState = "clear"
			} else if cch.Encrypted > 0 {
				value.CipherState = "encrypted"
			} else {
				value.CipherState = "unknown"
			}
			observation := ChannelObservation{ChannelID: int(cch.Channel), Kind: "group", Logical: "TCH", GroupID: int(cch.GSSI), CipherState: value.CipherState, Active: true, Observations: 1}
			value.Channels = []ChannelObservation{observation}
			if cch.GSSI >= 0 {
				value.Groups = []GroupObservation{{GroupID: int(cch.GSSI), ChannelID: int(cch.Channel), CipherState: value.CipherState, Observations: 1}}
			}
		}
	}
	return value, nil
}

func cString(value []byte) string {
	for index, current := range value {
		if current == 0 {
			return string(value[:index])
		}
	}
	return string(value)
}
