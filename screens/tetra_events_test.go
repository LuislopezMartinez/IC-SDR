package screens

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/internal/tetra"
	"go-zero/simpleui"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestTETRAEventFiltering(t *testing.T) {
	v := tetraViewer{snapshot: tetra.LiveSnapshot{Messages: []tetra.Message{{Kind: "D-SETUP"}, {Kind: "SDS TEXTO", SDS: true}}}}
	if len(v.events(true)) != 1 || v.events(true)[0].SDS || len(v.events(false)) != 1 || !v.events(false)[0].SDS {
		t.Fatal("event filtering mixes SDS and signalling")
	}
}
func TestTETRASignalingRender(t *testing.T) {
	dir := os.Getenv("TETRA_RENDER_DIR")
	if dir == "" {
		t.Skip("visual QA not requested")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetConfigFlags(rl.FlagWindowHidden)
	rl.InitWindow(1400, 780, "TETRA QA")
	defer rl.CloseWindow()
	simpleui.SetTextScale(1.55)
	carrier := uint16(832)
	now := time.Now()
	msg := tetra.Message{Time: now, Kind: "D-SETUP", AddressSSI: 500100, PartySSI: 5003014, CallID: 43, Carrier: &carrier, Slot: 1, AssignedSlots: 3, Fields: map[string]uint32{"Transmission_grant": 3, "Basic_service_Communication_type": 1, "Basic_service_Encryption_flag": 0, "Basic_service_Circuit_mode_type": 0}, RawHex: "00112233445566778899AABBCCDDEEFF", RawBits: 128}
	v := tetraViewer{tab: 7, selectedEvent: &msg, nextRead: now.Add(time.Hour), nextSettingsRead: now.Add(time.Hour), snapshot: tetra.LiveSnapshot{Updated: now, FrequencyHz: 420800000, Messages: []tetra.Message{msg}, Positions: []tetra.Position{{SSI: 5003014, Latitude: 40.4, Longitude: -3.7, Time: now}}}}
	os.MkdirAll(dir, 0755)
	target := rl.LoadRenderTexture(1400, 780)
	defer rl.UnloadRenderTexture(target)
	for _, name := range []string{"tetra-signaling.png", "tetra-positions.png"} {
		rl.BeginTextureMode(target)
		v.draw()
		rl.EndTextureMode()
		img := rl.LoadImageFromTexture(target.Texture)
		rl.ImageFlipVertical(img)
		if !rl.ExportImage(*img, filepath.Join(dir, name)) {
			t.Fatal("render failed")
		}
		rl.UnloadImage(img)
		v.tab = 5
	}
}
