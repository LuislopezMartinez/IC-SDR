//go:build windows

package omnirig

import (
	"fmt"
	"runtime"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

const (
	rigOnline = 4
	pmFreq    = 2
	pmFreqA   = 4
	pmFreqB   = 8
	pmCWU     = 8_388_608
	pmCWL     = 16_777_216
	pmSSBU    = 33_554_432
	pmSSBL    = 67_108_864
	pmDIGU    = 134_217_728
	pmDIGL    = 268_435_456
	pmRX      = 2_097_152
	pmTX      = 4_194_304
	pmAM      = 536_870_912
	pmFM      = 1_073_741_824
)

func (client *Client) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		if oleErr, ok := err.(*ole.OleError); !ok || oleErr.Code() != 1 { // S_FALSE is also success.
			client.finish(fmt.Errorf("no se pudo iniciar COM: %w", err))
			return
		}
	}
	defer ole.CoUninitialize()

	unknown, err := oleutil.CreateObject(omniRigProgID)
	if err != nil {
		activationErr := err
		if registerErr := registerPortableServer(client.executable); registerErr != nil {
			client.finish(fmt.Errorf("Omni-Rig no está instalado y no se pudo activar la copia portable (%v): %w", registerErr, activationErr))
			return
		}
		unknown, err = oleutil.CreateObject(omniRigProgID)
		if err != nil {
			client.finish(fmt.Errorf("la copia portable de Omni-Rig no pudo iniciarse: %w", err))
			return
		}
	}
	defer unknown.Release()
	app, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		client.finish(fmt.Errorf("interfaz de Omni-Rig no disponible: %w", err))
		return
	}
	defer app.Release()
	rig1Value, err := oleutil.GetProperty(app, "Rig1")
	if err != nil {
		client.finish(fmt.Errorf("no se pudo abrir RIG 1: %w", err))
		return
	}
	rig1 := rig1Value.ToIDispatch()
	defer rig1.Release()
	rig2Value, err := oleutil.GetProperty(app, "Rig2")
	if err != nil {
		client.finish(fmt.Errorf("no se pudo abrir RIG 2: %w", err))
		return
	}
	rig2 := rig2Value.ToIDispatch()
	defer rig2.Release()
	_, _ = oleutil.PutProperty(app, "DialogVisible", false)
	rigs := [2]*ole.IDispatch{rig1, rig2}
	selected := client.State().SelectedRig
	if selected != 2 {
		selected = 1
	}

	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	client.readState(rigs, selected)
	for {
		select {
		case <-client.stop:
			client.finish(nil)
			return
		case value := <-client.commands:
			err = nil
			if value.selectRig == 1 || value.selectRig == 2 {
				selected = value.selectRig
				client.readState(rigs, selected)
				continue
			}
			if value.dialog != nil {
				_, err = oleutil.PutProperty(app, "DialogVisible", *value.dialog)
			}
			if value.frequencyHz > 0 {
				err = setRigFrequency(rigs[selected-1], value.frequencyHz)
			}
			if value.mode != "" {
				if raw := modeToParam(value.mode); raw != 0 {
					_, err = oleutil.PutProperty(rigs[selected-1], "Mode", raw)
				}
			}
			if err != nil {
				state := client.State()
				state.Error = fmt.Sprintf("orden CAT rechazada: %v", err)
				client.publish(state)
			}
		case <-ticker.C:
			client.readState(rigs, selected)
		}
	}
}

func setRigFrequency(rig *ole.IDispatch, frequencyHz int64) error {
	writeable, err := oleutil.GetProperty(rig, "WriteableParams")
	if err != nil {
		return fmt.Errorf("no se pudieron consultar los parámetros CAT escribibles: %w", err)
	}
	mask := int32(writeable.Val)
	_ = writeable.Clear()
	property := writableFrequencyProperty(mask)
	if property == "" {
		return fmt.Errorf("el perfil de radio no permite escribir la frecuencia")
	}
	_, err = oleutil.PutProperty(rig, property, int32(frequencyHz))
	if err != nil {
		return fmt.Errorf("no se pudo escribir %s: %w", property, err)
	}
	return nil
}

func writableFrequencyProperty(mask int32) string {
	switch {
	case mask&pmFreq != 0:
		return "Freq"
	case mask&pmFreqA != 0:
		return "FreqA"
	case mask&pmFreqB != 0:
		return "FreqB"
	default:
		return ""
	}
}

func (client *Client) readState(rigs [2]*ole.IDispatch, selected int) {
	state := State{Running: true, Updated: time.Now(), SelectedRig: selected}
	state.Rig1 = readRigSummary(rigs[0])
	state.Rig2 = readRigSummary(rigs[1])
	selectedSummary := state.Rig1
	if selected == 2 {
		selectedSummary = state.Rig2
	}
	state.TXReadable = selectedSummary.TXReadable
	state.Transmitting = selectedSummary.Transmitting
	rig := rigs[selected-1]
	status, err := oleutil.GetProperty(rig, "Status")
	if err != nil {
		state.Error = fmt.Sprintf("no se pudo consultar RIG 1: %v", err)
		client.publish(state)
		return
	}
	state.Online = int(status.Val) == rigOnline
	_ = status.Clear()
	if value, getErr := oleutil.GetProperty(rig, "StatusStr"); getErr == nil {
		state.Status = value.ToString()
		_ = value.Clear()
	}
	if value, getErr := oleutil.GetProperty(rig, "RigType"); getErr == nil {
		state.RigType = value.ToString()
		_ = value.Clear()
	}
	if state.Online {
		if value, callErr := oleutil.CallMethod(rig, "GetRxFrequency"); callErr == nil {
			state.FrequencyHz = int64(int32(value.Val))
			_ = value.Clear()
		} else if value, getErr := oleutil.GetProperty(rig, "Freq"); getErr == nil {
			state.FrequencyHz = int64(int32(value.Val))
			_ = value.Clear()
		}
		if value, getErr := oleutil.GetProperty(rig, "Mode"); getErr == nil {
			state.Mode = paramToMode(int32(value.Val))
			_ = value.Clear()
		}
	}
	client.publish(state)
}

func readRigSummary(rig *ole.IDispatch) RigSummary {
	var summary RigSummary
	if value, err := oleutil.GetProperty(rig, "Status"); err == nil {
		summary.Online = int(value.Val) == rigOnline
		_ = value.Clear()
	}
	if value, err := oleutil.GetProperty(rig, "StatusStr"); err == nil {
		summary.Status = value.ToString()
		_ = value.Clear()
	}
	if value, err := oleutil.GetProperty(rig, "RigType"); err == nil {
		summary.RigType = value.ToString()
		_ = value.Clear()
	}
	if value, err := oleutil.CallMethod(rig, "IsParamReadable", int32(pmTX)); err == nil {
		summary.TXReadable = value.Val != 0
		_ = value.Clear()
	}
	if summary.TXReadable {
		if value, err := oleutil.GetProperty(rig, "Tx"); err == nil {
			summary.Transmitting = int32(value.Val) == pmTX
			_ = value.Clear()
		}
	}
	return summary
}

func paramToMode(value int32) string {
	switch value {
	case pmCWU, pmCWL:
		return ModeCW
	case pmSSBU:
		return ModeUSB
	case pmSSBL:
		return ModeLSB
	case pmDIGU, pmDIGL:
		return ModeDigital
	case pmAM:
		return ModeAM
	case pmFM:
		return ModeFM
	default:
		return ModeUnknown
	}
}

func modeToParam(mode string) int32 {
	switch mode {
	case ModeCW:
		return pmCWU
	case ModeUSB:
		return pmSSBU
	case ModeLSB:
		return pmSSBL
	case ModeDigital:
		return pmDIGU
	case ModeAM:
		return pmAM
	case ModeFM:
		return pmFM
	default:
		return 0
	}
}
