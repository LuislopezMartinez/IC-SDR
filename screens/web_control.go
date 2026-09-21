package screens

import (
	"go-zero/internal/i18n"

	"crypto/subtle"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go-zero/internal/tetra"
)

const remoteControlLease = 20 * time.Second

// Commands cross the HTTP/UI boundary through this queue. Raylib and receiver
// controls are only touched by the UI thread while draining it.
type webControlCommand struct {
	Action          string       `json:"action"`
	FrequencyHz     int64        `json:"frequencyHz,omitempty"`
	CenterHz        int64        `json:"centerHz,omitempty"`
	Steps           int64        `json:"steps,omitempty"`
	Mode            string       `json:"mode,omitempty"`
	Tool            string       `json:"tool,omitempty"`
	AudioSlot       string       `json:"audioSlot,omitempty"`
	TetraEnabled    *bool        `json:"tetraEnabled,omitempty"`
	TetraBand       *int         `json:"tetraBand,omitempty"`
	TetraAutoCenter *bool        `json:"tetraAutoCenter,omitempty"`
	TetraClearOnly  *bool        `json:"tetraClearOnly,omitempty"`
	FilterIndex     int          `json:"filterIndex,omitempty"`
	BandwidthHz     int          `json:"bandwidthHz,omitempty"`
	MemoryIndex     int          `json:"memoryIndex,omitempty"`
	MemoryName      string       `json:"memoryName,omitempty"`
	Memory          *MemoryEntry `json:"memory,omitempty"`
	Group           string       `json:"group,omitempty"`
	NewGroup        string       `json:"newGroup,omitempty"`
	SpanHz          int64        `json:"spanHz,omitempty"`
	StepHz          int64        `json:"stepHz,omitempty"`
	Fixed           *bool        `json:"fixed,omitempty"`
	SquelchEnabled  *bool        `json:"squelchEnabled,omitempty"`
	SquelchDBm      *int         `json:"squelchDBm,omitempty"`
	ScannerEnabled  *bool        `json:"scannerEnabled,omitempty"`
	ScannerVisible  *bool        `json:"scannerVisible,omitempty"`
	ScannerMemory   *bool        `json:"scannerMemory,omitempty"`
	ScannerResume   string       `json:"scannerResume,omitempty"`
	token           string
}

func (command webControlCommand) validate() error {
	switch command.Action {
	case "frequency", "tuneAt":
		if command.FrequencyHz < 100_000 || command.FrequencyHz > 6_000_000_000 {
			return errors.New(i18n.Source("text.f74f3e59118e"))
		}
	case "panCenter":
		if command.CenterHz < 100_000 || command.CenterHz > 6_000_000_000 {
			return errors.New(i18n.Source("text.2d11652243f2"))
		}
	case "step":
		if command.Steps != -1 && command.Steps != 1 {
			return errors.New(i18n.Source("text.947fccf72dad"))
		}
	case "mode":
		allowed := map[string]bool{"AM": true, i18n.Source("text.0896d612d497"): true, i18n.Source("text.6b742bac3eb4"): true, i18n.Source("text.61f0acff1735"): true, i18n.Source("text.6323db4948ad"): true, "CW": true, i18n.Source("text.2604864ce4d3"): true, i18n.Source("text.7866f9f32e66"): true, i18n.Source("text.72c048cb5100"): true, i18n.Source("text.f69d86a86926"): true}
		if !allowed[command.Mode] {
			return errors.New(i18n.Source("text.546540304080"))
		}
	case "tool":
		if command.Tool != i18n.Source("text.93239b223632") && command.Tool != i18n.Source("text.f69d86a86926") {
			return errors.New(i18n.Source("text.aea1f5575a08"))
		}
	case "dmrAudioSlot":
		if command.AudioSlot != i18n.Source("text.6ea56fae9eac") && command.AudioSlot != "TS1" && command.AudioSlot != "TS2" {
			return errors.New(i18n.Source("text.37e347e1574e"))
		}
	case "tetraRunning":
		if command.TetraEnabled == nil {
			return errors.New(i18n.Source("text.898041b7efaf"))
		}
	case "tetraBand":
		if command.TetraBand == nil || *command.TetraBand < 0 || *command.TetraBand > 2 {
			return errors.New(i18n.Source("text.7fc4689651b3"))
		}
	case "tetraAutoCenter":
		if command.TetraAutoCenter == nil {
			return errors.New(i18n.Source("text.1886a6138cb7"))
		}
	case "tetraClearOnly":
		if command.TetraClearOnly == nil {
			return errors.New(i18n.Source("text.7bb60680d0f6"))
		}
	case "filter":
		if command.FilterIndex < 0 || command.FilterIndex > 3 {
			return errors.New(i18n.Source("text.e2b0b1aae913"))
		}
	case "customFilter":
		if command.BandwidthHz < 300 || command.BandwidthHz > 2_048_000 {
			return errors.New(i18n.Source("text.59eabd522589"))
		}
	case "span":
		allowed := map[int64]bool{50_000: true, 100_000: true, 250_000: true, 500_000: true, 1_000_000: true, 2_000_000: true}
		if !allowed[command.SpanHz] {
			return errors.New(i18n.Source("text.1d5b966b62ab"))
		}
	case "stepSize":
		valid := false
		for _, step := range tuningStepsHz {
			if command.StepHz == step {
				valid = true
				break
			}
		}
		if !valid {
			return errors.New(i18n.Source("text.947fccf72dad"))
		}
	case "fixed":
		if command.Fixed == nil {
			return errors.New(i18n.Source("text.82e96755034e"))
		}
	case "memoryTune", "memoryDelete":
		if command.MemoryIndex < 0 || command.MemoryName == "" {
			return errors.New(i18n.Source("text.08c6e503c144"))
		}
	case "memorySave":
		if command.Memory == nil || errInvalidMemory(*command.Memory) {
			return errors.New(i18n.Source("text.7d17e159a71c"))
		}
		if command.MemoryIndex < -1 {
			return errors.New(i18n.Source("text.2792ba0996a1"))
		}
		if command.MemoryIndex >= 0 && command.MemoryName == "" {
			return errors.New(i18n.Source("text.210e8644c2ae"))
		}
	case "groupAdd":
		if !validMemoryGroup(command.Group) {
			return errors.New(i18n.Source("text.a0682da0dcbb"))
		}
	case "groupRename":
		if !validMemoryGroup(command.Group) || !validMemoryGroup(command.NewGroup) {
			return errors.New(i18n.Source("text.a0682da0dcbb"))
		}
	case "squelch":
		if command.SquelchEnabled == nil {
			return errors.New(i18n.Source("text.0adaa883f442"))
		}
	case "squelchLevel":
		if command.SquelchDBm == nil || *command.SquelchDBm < -160 || *command.SquelchDBm > 20 {
			return errors.New(i18n.Source("text.762c40a296d7"))
		}
	case "scannerRunning":
		if command.ScannerEnabled == nil {
			return errors.New(i18n.Source("text.559cd8513fdb"))
		}
	case "scannerOverlay":
		if command.ScannerVisible == nil {
			return errors.New(i18n.Source("text.42f7d9b27bc6"))
		}
	case "scannerMemory":
		if command.ScannerMemory == nil {
			return errors.New(i18n.Source("text.da098328cbcb"))
		}
	case "scannerResume":
		if command.ScannerResume != i18n.Source("text.6ea56fae9eac") && command.ScannerResume != i18n.Source("text.85135a165905") && command.ScannerResume != i18n.Source("text.aacf94b7be62") {
			return errors.New(i18n.Source("text.0dcd158b4183"))
		}
	case "scannerRangeFFT":
	default:
		return errors.New(i18n.Source("text.92fbf44e348c"))
	}
	return nil
}

func (service *WebServer) RemoteActive() bool {
	service.mu.RLock()
	active := service.controlToken != "" && time.Now().Before(service.controlUntil)
	service.mu.RUnlock()
	return active
}

func (service *WebServer) controlAction(w http.ResponseWriter, r *http.Request) {
	if service.controlCommands == nil {
		http.Error(w, i18n.Source("text.6c2af67d4e70"), http.StatusServiceUnavailable)
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Host != r.Host || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			http.Error(w, i18n.Source("text.876893a4e122"), http.StatusForbidden)
			return
		}
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, i18n.Source("text.51bd4ca71ea4"), http.StatusUnsupportedMediaType)
		return
	}
	cookie, err := r.Cookie("icsdr_control")
	if err != nil {
		http.Error(w, i18n.Source("text.9f56a34a4830"), http.StatusForbidden)
		return
	}
	service.mu.RLock()
	valid := service.controlToken != "" && time.Now().Before(service.controlUntil) && subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(service.controlToken)) == 1
	service.mu.RUnlock()
	if !valid {
		http.Error(w, i18n.Source("text.9f56a34a4830"), http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2048)
	var command webControlCommand
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&command) != nil || command.validate() != nil {
		http.Error(w, i18n.Source("text.0b507f3bbc7e"), http.StatusBadRequest)
		return
	}
	command.token = cookie.Value
	select {
	case service.controlCommands <- command:
		w.WriteHeader(http.StatusAccepted)
	default:
		http.Error(w, i18n.Source("text.c19841c89b88"), http.StatusServiceUnavailable)
	}
}

func (service *WebServer) DrainControl(screen *MainScreen) {
	for count := 0; count < 16; count++ {
		select {
		case command := <-service.controlCommands:
			service.mu.RLock()
			valid := command.token != "" && service.controlToken != "" && time.Now().Before(service.controlUntil) && subtle.ConstantTimeCompare([]byte(command.token), []byte(service.controlToken)) == 1
			service.mu.RUnlock()
			if valid {
				screen.applyWebControl(command)
			}
		default:
			return
		}
	}
}

func (screen *MainScreen) applyWebControl(command webControlCommand) {
	switch command.Action {
	case "frequency":
		screen.setRemoteFrequency(command.FrequencyHz)
	case "tuneAt":
		screen.tuneRemoteAt(command.FrequencyHz)
	case "panCenter":
		screen.panRemoteCenter(command.CenterHz)
	case "step":
		if screen.scanPanel != nil && screen.scanPanel.running {
			screen.scanPanel.Stop()
		}
		previousFrequency := screen.frequencyHz
		if screen.centerMode {
			screen.tuneCenteredBySteps(command.Steps)
		} else {
			screen.tuneFixedBySteps(command.Steps)
		}
		if screen.receiver != nil {
			screen.receiver.SetCenterFrequency(screen.centerFrequencyHz)
			screen.receiver.SetDemodulator(screen.receiverDemodMode(), screen.frequencyHz, screen.demodBandwidthHz)
		}
		screen.resetTETRAAfterManualSpectrumTune(previousFrequency)
	case "mode":
		screen.selectMode(command.Mode)
		if command.Mode == i18n.Source("text.2604864ce4d3") && screen.activeTool != i18n.Source("text.7a1580c49e45") {
			screen.selectTool(i18n.Source("text.93239b223632"))
		} else if command.Mode != i18n.Source("text.2604864ce4d3") && screen.activeTool == i18n.Source("text.93239b223632") {
			screen.selectTool(i18n.Source("text.a42c60257b01"))
		}
	case "tool":
		if screen.scanPanel != nil && screen.scanPanel.running {
			screen.scanPanel.Stop()
		}
		if screen.activeTool != command.Tool {
			screen.selectTool(command.Tool)
		}
		if screen.mode != nil {
			expected := i18n.Source("text.f69d86a86926")
			if command.Tool == i18n.Source("text.93239b223632") {
				expected = i18n.Source("text.2604864ce4d3")
			}
			if screen.mode.SelectedText() != expected {
				screen.selectMode(expected)
			}
		}
	case "dmrAudioSlot":
		if screen.activeTool != i18n.Source("text.93239b223632") {
			return
		}
		if screen.dmrPanel != nil {
			screen.dmrPanel.selectSlot(command.AudioSlot)
		} else {
			screen.dmrAudioSlot = command.AudioSlot
			if screen.receiver != nil {
				screen.receiver.SetDMRAudioSlot(command.AudioSlot)
			}
			screen.markSettingsDirty()
		}
	case "tetraRunning", "tetraBand", "tetraAutoCenter", "tetraClearOnly":
		if screen.activeTool != i18n.Source("text.f69d86a86926") || screen.tetraPanel == nil {
			return
		}
		panel := screen.tetraPanel
		switch command.Action {
		case "tetraRunning":
			if panel.enabled != *command.TetraEnabled {
				panel.enabled = *command.TetraEnabled
				panel.apply()
			}
		case "tetraBand":
			bands := [...]int64{tetra.DefaultFrequencyHz, 420_000_000, 460_000_000}
			panel.tune(bands[*command.TetraBand])
		case "tetraAutoCenter":
			panel.autoCenter = *command.TetraAutoCenter
			if panel.autoCenterSwitch != nil {
				panel.autoCenterSwitch.SetActive(panel.autoCenter)
			}
		case "tetraClearOnly":
			panel.clearOnly = *command.TetraClearOnly
			if panel.clearOnlySwitch != nil {
				panel.clearOnlySwitch.SetActive(panel.clearOnly)
			}
			panel.applyAudioPolicy()
		}
	case "filter":
		if screen.filterSelector != nil && screen.mode != nil {
			screen.selectFilter(screen.filterSelector.SelectPreset(screen.mode.SelectedText(), command.FilterIndex))
		}
	case "customFilter":
		if screen.filterSelector != nil && screen.mode != nil {
			mode := filterMode(screen.mode.SelectedText())
			minimum, maximum, step := customFilterRange(mode)
			if command.BandwidthHz < minimum || command.BandwidthHz > maximum || (command.BandwidthHz-minimum)%step != 0 {
				return
			}
			filterCatalog[mode][3].BandwidthHz = command.BandwidthHz
			screen.selectFilter(screen.filterSelector.SelectPreset(mode, 3))
		}
	case "span":
		for screen.spanHz != command.SpanHz {
			if screen.spanHz < command.SpanHz {
				screen.changeSpan(1)
			} else {
				screen.changeSpan(-1)
			}
		}
	case "stepSize":
		screen.selectTuningStep(command.StepHz)
	case "fixed":
		screen.centerMode = !*command.Fixed
		if screen.vfoModeSwitch != nil {
			screen.vfoModeSwitch.SetActive(*command.Fixed)
		}
		if screen.centerMode {
			screen.centerFrequencyHz = screen.frequencyHz
			if screen.receiver != nil {
				screen.receiver.SetCenterFrequency(screen.centerFrequencyHz)
			}
			if screen.waterfall != nil {
				screen.waterfall.Reset()
			}
		}
		if screen.receiver != nil {
			screen.receiver.SetDemodulator(screen.receiverDemodMode(), screen.frequencyHz, screen.demodBandwidthHz)
		}
		screen.markSettingsDirty()
	case "memoryTune", "memoryDelete", "memorySave", "groupAdd", "groupRename":
		screen.applyWebMemoryControl(command)
	case "squelch":
		screen.squelchEnabled = *command.SquelchEnabled
		if screen.squelchSwitch != nil {
			screen.squelchSwitch.SetActive(screen.squelchEnabled)
		}
		screen.applySquelch()
		screen.markSettingsDirty()
	case "squelchLevel":
		level := *command.SquelchDBm
		level = min(max(level, int(math.Ceil(float64(screen.spectrumMinimumDB)))), int(math.Floor(float64(screen.spectrumMaximumDB))))
		screen.squelchThreshold = level
		if screen.squelchSlider != nil {
			screen.squelchSlider.SetValue(float32(level))
		}
		if screen.squelchLabel != nil {
			screen.squelchLabel.SetText(i18n.Source("text.6466f20e2210") + strconv.Itoa(level) + i18n.Source("text.7d97ab01faba"))
		}
		screen.applySquelch()
		screen.markSettingsDirty()
	case "scannerRunning", "scannerOverlay", "scannerMemory", "scannerResume", "scannerRangeFFT":
		panel := screen.scanPanel
		if panel == nil {
			return
		}
		switch command.Action {
		case "scannerRunning":
			if panel.running != *command.ScannerEnabled {
				panel.ToggleRunning()
			}
		case "scannerOverlay":
			panel.overlayVisible = *command.ScannerVisible
		case "scannerMemory":
			panel.centerToMemory = *command.ScannerMemory
		case "scannerResume":
			panel.resume = command.ScannerResume
		case "scannerRangeFFT":
			if screen.spanHz <= 0 {
				return
			}
			panel.minimumHz = screen.centerFrequencyHz - screen.spanHz*4/10
			panel.maximumHz = screen.centerFrequencyHz + screen.spanHz*4/10
		}
		screen.markSettingsDirty()
	}
}

func (screen *MainScreen) panRemoteCenter(hz int64) {
	if hz < 100_000 || hz > 6_000_000_000 || screen.spanHz <= 0 {
		return
	}
	if screen.scanPanel != nil && screen.scanPanel.running {
		screen.scanPanel.Stop()
	}
	previousFrequency := screen.frequencyHz
	screen.centerFrequencyHz = max((hz/1_000)*1_000, 1_000)
	if screen.centerMode {
		screen.frequencyHz = screen.centerFrequencyHz
	}
	if screen.receiver != nil {
		screen.receiver.SetCenterFrequency(screen.centerFrequencyHz)
		screen.receiver.SetDemodulator(screen.receiverDemodMode(), screen.frequencyHz, screen.demodBandwidthHz)
	}
	screen.resetTETRAAfterManualSpectrumTune(previousFrequency)
	screen.markSettingsDirty()
}

func (screen *MainScreen) tuneRemoteAt(hz int64) {
	if hz < 100_000 || hz > 6_000_000_000 {
		return
	}
	if step := screen.activeTuningStepHz(); step > 0 {
		hz = int64(math.Round(float64(hz)/float64(step))) * step
	}
	if screen.centerMode || screen.spanHz <= 0 || screen.activeTool == i18n.Source("text.8be70e7cb2c4") {
		screen.setRemoteFrequency(hz)
		return
	}
	if screen.scanPanel != nil && screen.scanPanel.running {
		screen.scanPanel.Stop()
	}
	previousFrequency, previousCenter := screen.frequencyHz, screen.centerFrequencyHz
	screen.frequencyHz = hz
	halfSpan := screen.spanHz / 2
	guard := max(int64(math.Round(float64(screen.spanHz)*.05)), int64(screen.demodBandwidthHz/2))
	if screen.mode != nil && (screen.mode.SelectedText() == i18n.Source("text.61f0acff1735") || screen.mode.SelectedText() == i18n.Source("text.6323db4948ad")) {
		guard = max(guard, int64(screen.demodBandwidthHz))
	}
	guard = min(guard, int64(math.Round(float64(screen.spanHz)*.40)))
	if hz < screen.centerFrequencyHz-halfSpan+guard {
		screen.centerFrequencyHz = hz + halfSpan - guard
	}
	if hz > screen.centerFrequencyHz+halfSpan-guard {
		screen.centerFrequencyHz = hz - halfSpan + guard
	}
	screen.centerFrequencyHz = max(screen.centerFrequencyHz, halfSpan)
	if screen.receiver != nil {
		if screen.centerFrequencyHz != previousCenter {
			screen.receiver.SetCenterFrequency(screen.centerFrequencyHz)
		}
		screen.receiver.SetDemodulator(screen.receiverDemodMode(), hz, screen.demodBandwidthHz)
	}
	if screen.centerFrequencyHz != previousCenter && screen.waterfall != nil {
		screen.waterfall.Reset()
	}
	screen.resetTETRAAfterManualSpectrumTune(previousFrequency)
	screen.markSettingsDirty()
}

func (screen *MainScreen) setRemoteFrequency(hz int64) {
	if hz < 100_000 || hz > 6_000_000_000 {
		return
	}
	if screen.activeTool == i18n.Source("text.8be70e7cb2c4") && screen.rtl433Panel != nil {
		screen.rtl433Panel.selectFrequency(hz, true)
		return
	}
	if screen.scanPanel != nil && screen.scanPanel.running {
		screen.scanPanel.Stop()
	}
	previous := screen.frequencyHz
	screen.frequencyHz = hz
	screen.centerFrequencyHz = hz
	screen.draggingSpectrum = false
	if screen.waterfall != nil {
		screen.waterfall.Reset()
	}
	if screen.receiver != nil {
		screen.receiver.SetCenterFrequency(hz)
		screen.receiver.SetDemodulator(screen.receiverDemodMode(), hz, screen.demodBandwidthHz)
	}
	screen.resetTETRAAfterManualSpectrumTune(previous)
	screen.markSettingsDirty()
}
