package screens

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/simpleui"
)

func TestWebControlRejectsUnauthenticatedAndInvalidCommands(t *testing.T) {
	service := &WebServer{controlToken: "owner", controlUntil: time.Now().Add(time.Minute), controlCommands: make(chan webControlCommand, 2)}
	request := func(body, token string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/control/action", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.AddCookie(&http.Cookie{Name: "icsdr_control", Value: token})
		}
		response := httptest.NewRecorder()
		service.controlAction(response, req)
		return response.Code
	}
	if got := request(`{"action":"frequency","frequencyHz":446193750}`, ""); got != http.StatusForbidden {
		t.Fatalf("spectator accepted: %d", got)
	}
	if got := request(`{"action":"frequency","frequencyHz":446193750}`, "other"); got != http.StatusForbidden {
		t.Fatalf("other client accepted: %d", got)
	}
	if got := request(`{"action":"memoryDelete","memoryIndex":0,"memoryName":"PRUEBA"}`, ""); got != http.StatusForbidden {
		t.Fatalf("spectator memory delete accepted: %d", got)
	}
	if got := request(`{"action":"scannerRunning","scannerEnabled":true}`, ""); got != http.StatusForbidden {
		t.Fatalf("spectator scanner command accepted: %d", got)
	}
	if got := request(`{"action":"scannerResume","scannerResume":"INVALID"}`, "owner"); got != http.StatusBadRequest {
		t.Fatalf("invalid scanner resume accepted: %d", got)
	}
	if got := request(`{"action":"scannerRunning"}`, "owner"); got != http.StatusBadRequest {
		t.Fatalf("scanner command without state accepted: %d", got)
	}
	if got := request(`{"action":"tool","tool":"TETRA"}`, ""); got != http.StatusForbidden {
		t.Fatalf("spectator tool selection accepted: %d", got)
	}
	if got := request(`{"action":"tool","tool":"WEB_SERVER"}`, "owner"); got != http.StatusBadRequest {
		t.Fatalf("unavailable tool accepted: %d", got)
	}
	if got := request(`{"action":"tool","tool":"DMR_MONITOR"}`, "owner"); got != http.StatusAccepted {
		t.Fatalf("DMR selection denied: %d", got)
	}
	for _, body := range []string{
		`{"action":"dmrAudioSlot","audioSlot":"TS3"}`,
		`{"action":"tetraRunning"}`,
		`{"action":"tetraBand","tetraBand":3}`,
		`{"action":"tetraAutoCenter"}`,
		`{"action":"tetraClearOnly"}`,
	} {
		if got := request(body, "owner"); got != http.StatusBadRequest {
			t.Fatalf("invalid digital command accepted (%s): %d", body, got)
		}
	}
	if got := request(`{"action":"tetraRunning","tetraEnabled":true}`, ""); got != http.StatusForbidden {
		t.Fatalf("spectator TETRA command accepted: %d", got)
	}
	if got := request(`{"action":"frequency","frequencyHz":-1}`, "owner"); got != http.StatusBadRequest {
		t.Fatalf("invalid frequency accepted: %d", got)
	}
	if got := request(`{"action":"deleteEverything"}`, "owner"); got != http.StatusBadRequest {
		t.Fatalf("unknown action accepted: %d", got)
	}
	crossOrigin := httptest.NewRequest(http.MethodPost, "/api/control/action", strings.NewReader(`{"action":"step","steps":1}`))
	crossOrigin.Header.Set("Content-Type", "application/json")
	crossOrigin.Header.Set("Origin", "http://attacker.example")
	crossOrigin.AddCookie(&http.Cookie{Name: "icsdr_control", Value: "owner"})
	response := httptest.NewRecorder()
	service.controlAction(response, crossOrigin)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin command accepted: %d", response.Code)
	}
	if got := request(`{"action":"frequency","frequencyHz":446193750}`, "owner"); got != http.StatusAccepted {
		t.Fatalf("controller denied: %d", got)
	}
	if len(service.controlCommands) != 2 {
		t.Fatal("accepted command not queued")
	}
}

func TestWebDigitalControlsApplyOnlyToActiveTool(t *testing.T) {
	screen := &MainScreen{activeTool: "DMR_MONITOR"}
	screen.applyWebControl(webControlCommand{Action: "dmrAudioSlot", AudioSlot: "TS2"})
	if screen.dmrAudioSlot != "TS2" {
		t.Fatal("DMR audio slot not selected")
	}
	screen.activeTool = "TETRA"
	screen.applyWebControl(webControlCommand{Action: "dmrAudioSlot", AudioSlot: "TS1"})
	if screen.dmrAudioSlot != "TS2" {
		t.Fatal("DMR command applied to inactive tool")
	}
	panel := &TETRAPanel{screen: screen, autoCenter: true, clearOnly: true}
	screen.tetraPanel = panel
	disabled := false
	screen.applyWebControl(webControlCommand{Action: "tetraAutoCenter", TetraAutoCenter: &disabled})
	screen.applyWebControl(webControlCommand{Action: "tetraClearOnly", TetraClearOnly: &disabled})
	if panel.autoCenter || panel.clearOnly {
		t.Fatal("TETRA switches not applied")
	}
	screen.activeTool = "DMR_MONITOR"
	enabled := true
	screen.applyWebControl(webControlCommand{Action: "tetraAutoCenter", TetraAutoCenter: &enabled})
	if panel.autoCenter {
		t.Fatal("TETRA command applied to inactive tool")
	}
}

func TestWebScannerControls(t *testing.T) {
	screen := &MainScreen{centerFrequencyHz: 446_100_000, spanHz: 250_000, centerMode: true}
	screen.scanPanel = NewScanPanel(screen)
	enabled, disabled := true, false
	screen.applyWebControl(webControlCommand{Action: "scannerMemory", ScannerMemory: &enabled})
	screen.applyWebControl(webControlCommand{Action: "scannerOverlay", ScannerVisible: &disabled})
	screen.applyWebControl(webControlCommand{Action: "scannerResume", ScannerResume: "HOLD"})
	screen.applyWebControl(webControlCommand{Action: "scannerRangeFFT"})
	if !screen.scanPanel.centerToMemory || screen.scanPanel.overlayVisible || screen.scanPanel.resume != "HOLD" {
		t.Fatal("scanner options not applied")
	}
	if screen.scanPanel.minimumHz != 446_000_000 || screen.scanPanel.maximumHz != 446_200_000 {
		t.Fatalf("wrong FFT scan range: %d–%d", screen.scanPanel.minimumHz, screen.scanPanel.maximumHz)
	}
	screen.applyWebControl(webControlCommand{Action: "scannerRunning", ScannerEnabled: &enabled})
	screen.applyWebControl(webControlCommand{Action: "scannerRunning", ScannerEnabled: &enabled})
	if !screen.scanPanel.running || screen.centerMode {
		t.Fatal("scanner did not start in fixed mode")
	}
	screen.applyWebControl(webControlCommand{Action: "scannerRunning", ScannerEnabled: &disabled})
	if screen.scanPanel.running {
		t.Fatal("scanner did not stop")
	}
}

func TestWebControlRunsOnUIThreadAndDiscardsOldOwner(t *testing.T) {
	screen := &MainScreen{frequencyHz: 100_000_000, centerFrequencyHz: 100_000_000, spanHz: 250_000, tuningStepHz: 6_250, spectrumMinimumDB: -120, spectrumMaximumDB: 0}
	service := &WebServer{controlToken: "owner", controlUntil: time.Now().Add(time.Minute), controlCommands: make(chan webControlCommand, 3)}
	service.controlCommands <- webControlCommand{Action: "frequency", FrequencyHz: 446_193_750, token: "owner"}
	if screen.frequencyHz != 100_000_000 {
		t.Fatal("HTTP goroutine mutated screen")
	}
	service.DrainControl(screen)
	if screen.frequencyHz != 446_193_750 || screen.centerFrequencyHz != 446_193_750 {
		t.Fatalf("UI did not tune: %d / %d", screen.frequencyHz, screen.centerFrequencyHz)
	}
	service.controlCommands <- webControlCommand{Action: "frequency", FrequencyHz: 123_000_000, token: "owner"}
	service.mu.Lock()
	service.controlToken = "new-owner"
	service.mu.Unlock()
	service.DrainControl(screen)
	if screen.frequencyHz != 446_193_750 {
		t.Fatal("old owner's queued command applied")
	}
}

func TestRemoteLockEndsWhenLeaseExpires(t *testing.T) {
	service := &WebServer{controlToken: "owner", controlUntil: time.Now().Add(time.Minute)}
	screen := &MainScreen{webServer: service}
	if !NewRemoteLockOverlay(screen).OverlayOpen() {
		t.Fatal("desktop not locked")
	}
	service.controlUntil = time.Now().Add(-time.Second)
	if NewRemoteLockOverlay(screen).OverlayOpen() {
		t.Fatal("desktop remained locked after expiry")
	}
}

func TestRemoteLockEndsOnOwnerLogout(t *testing.T) {
	service := &WebServer{controlToken: "owner", controlUntil: time.Now().Add(time.Minute)}
	screen := &MainScreen{webServer: service}
	request := httptest.NewRequest(http.MethodPost, "/api/control/logout", nil)
	request.AddCookie(&http.Cookie{Name: "icsdr_control", Value: "owner"})
	service.controlLogout(httptest.NewRecorder(), request)
	if NewRemoteLockOverlay(screen).OverlayOpen() {
		t.Fatal("desktop remained locked after owner logout")
	}
}

func TestRemoteOverlayBlocksAndRestoresDesktopClicks(t *testing.T) {
	service := &WebServer{controlToken: "owner", controlUntil: time.Now().Add(time.Minute)}
	screen := &MainScreen{webServer: service}
	manager := simpleui.NewManager()
	button := simpleui.NewButton("remoteLockTestButton", 10, 10, 80, 30, "TEST", 12)
	clicks := 0
	button.OnClick(func() { clicks++ })
	manager.Add(button)
	manager.Add(NewRemoteLockOverlay(screen))
	input := simpleui.Input{Pointer: rl.Vector2{X: 20, Y: 20}, PointerInCanvas: true}
	input.Pressed = true
	manager.Update(input)
	input.Pressed = false
	input.Released = true
	manager.Update(input)
	if clicks != 0 {
		t.Fatal("desktop clicked through remote lock")
	}
	service.controlUntil = time.Now().Add(-time.Second)
	input.Pressed = true
	input.Released = false
	manager.Update(input)
	input.Pressed = false
	input.Released = true
	manager.Update(input)
	if clicks != 1 {
		t.Fatalf("desktop not restored after expiry: %d clicks", clicks)
	}
}

func TestWebControlModeFilterSpanAndSquelch(t *testing.T) {
	screen := &MainScreen{frequencyHz: 100_000_000, centerFrequencyHz: 100_000_000, spanHz: 250_000, spectrumMinimumDB: -120, spectrumMaximumDB: 0}
	screen.mode = simpleui.NewDropdown("testRemoteMode", 0, 0, 100, 30, "MODE", []string{"AM", "NFM", "WFM", "USB", "LSB", "CW", "DMR BETA", "ADS-B", "UAT", "TETRA"}, 12)
	screen.filterSelector = NewFilterSelector(screen.selectFilter)
	screen.filter = simpleui.NewButton("testRemoteFilter", 0, 0, 100, 30, "FIL2", 12)
	screen.applyWebControl(webControlCommand{Action: "mode", Mode: "NFM"})
	if got := screen.mode.SelectedText(); got != "NFM" {
		t.Fatalf("mode = %s", got)
	}
	screen.applyWebControl(webControlCommand{Action: "filter", FilterIndex: 2})
	if screen.demodBandwidthHz != 8500 || screen.filterSelector.selected["NFM"] != 2 {
		t.Fatalf("filter = %d Hz", screen.demodBandwidthHz)
	}
	screen.applyWebControl(webControlCommand{Action: "span", SpanHz: 100_000})
	if screen.spanHz != 100_000 {
		t.Fatalf("span = %d", screen.spanHz)
	}
	enabled, level := true, -35
	screen.applyWebControl(webControlCommand{Action: "squelch", SquelchEnabled: &enabled})
	screen.applyWebControl(webControlCommand{Action: "squelchLevel", SquelchDBm: &level})
	if !screen.squelchEnabled || screen.squelchThreshold != -35 {
		t.Fatalf("SQL = %v / %d", screen.squelchEnabled, screen.squelchThreshold)
	}
}

func TestWebControlFFTTuneCenterAndFixed(t *testing.T) {
	screen := &MainScreen{frequencyHz: 100_000_000, centerFrequencyHz: 100_000_000, spanHz: 250_000, tuningStepHz: 1_000, frequencyDigitExponent: -1, centerMode: true}
	screen.applyWebControl(webControlCommand{Action: "tuneAt", FrequencyHz: 100_050_400})
	if screen.frequencyHz != 100_050_000 || screen.centerFrequencyHz != 100_050_000 {
		t.Fatalf("CENTER: frequency=%d center=%d", screen.frequencyHz, screen.centerFrequencyHz)
	}
	fixed := true
	screen.applyWebControl(webControlCommand{Action: "fixed", Fixed: &fixed})
	screen.applyWebControl(webControlCommand{Action: "tuneAt", FrequencyHz: 100_075_200})
	if screen.centerMode || screen.frequencyHz != 100_075_000 || screen.centerFrequencyHz != 100_050_000 {
		t.Fatalf("FIX: frequency=%d center=%d centerMode=%v", screen.frequencyHz, screen.centerFrequencyHz, screen.centerMode)
	}
	screen.applyWebControl(webControlCommand{Action: "tuneAt", FrequencyHz: 100_250_000})
	if screen.centerFrequencyHz == 100_050_000 {
		t.Fatal("FIX did not move capture center at its edge")
	}
	screen.applyWebControl(webControlCommand{Action: "stepSize", StepHz: 6_250})
	if screen.tuningStepHz != 6_250 {
		t.Fatalf("step = %d", screen.tuningStepHz)
	}
	before := screen.frequencyHz
	screen.applyWebControl(webControlCommand{Action: "step", Steps: 1})
	if screen.frequencyHz != before+6_250 {
		t.Fatalf("wheel step = %d, want %d", screen.frequencyHz, before+6_250)
	}
}

func TestWebControlCustomFilter(t *testing.T) {
	original := filterCatalog["NFM"][3].BandwidthHz
	defer func() { filterCatalog["NFM"][3].BandwidthHz = original }()
	screen := &MainScreen{frequencyHz: 100_000_000, centerFrequencyHz: 100_000_000, spanHz: 250_000}
	screen.mode = simpleui.NewDropdown("testRemoteCustomMode", 0, 0, 100, 30, "MODE", []string{"AM", "NFM"}, 12)
	screen.filterSelector = NewFilterSelector(screen.selectFilter)
	screen.filter = simpleui.NewButton("testRemoteCustomFilter", 0, 0, 100, 30, "FIL2", 12)
	screen.mode.SetSelected(1)
	screen.applyWebControl(webControlCommand{Action: "customFilter", BandwidthHz: 11_000})
	if screen.demodBandwidthHz != 11_000 || screen.filterSelector.selected["NFM"] != 3 {
		t.Fatalf("custom filter = %d, index = %d", screen.demodBandwidthHz, screen.filterSelector.selected["NFM"])
	}
	screen.applyWebControl(webControlCommand{Action: "customFilter", BandwidthHz: 11_100})
	if screen.demodBandwidthHz != 11_000 {
		t.Fatal("accepted non-step custom filter value")
	}
}

func TestWebControlPanCenterAndFix(t *testing.T) {
	screen := &MainScreen{frequencyHz: 100_000_000, centerFrequencyHz: 100_000_000, spanHz: 250_000, centerMode: true}
	screen.applyWebControl(webControlCommand{Action: "panCenter", CenterHz: 100_050_600})
	if screen.centerFrequencyHz != 100_050_000 || screen.frequencyHz != 100_050_000 {
		t.Fatalf("CENTER pan: frequency=%d center=%d", screen.frequencyHz, screen.centerFrequencyHz)
	}
	screen.centerMode = false
	screen.applyWebControl(webControlCommand{Action: "panCenter", CenterHz: 100_110_000})
	if screen.centerFrequencyHz != 100_110_000 || screen.frequencyHz != 100_050_000 {
		t.Fatalf("FIX pan: frequency=%d center=%d", screen.frequencyHz, screen.centerFrequencyHz)
	}
	if err := (webControlCommand{Action: "panCenter", CenterHz: 0}).validate(); err == nil {
		t.Fatal("accepted invalid pan center")
	}
}
