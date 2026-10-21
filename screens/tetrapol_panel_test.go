package screens

import (
	"testing"
	"time"

	"go-zero/internal/tetrapolruntime"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestTETRAPOLAudioCipherPresentation(t *testing.T) {
	for state, want := range map[string]string{
		"clear":     "AUDIO ABIERTO",
		"encrypted": "AUDIO CIFRADO",
		"":          "SIN IDENTIFICAR",
	} {
		if got, _ := tetrapolAudioCipherPresentation(state); got != want {
			t.Fatalf("state %q label = %q, want %q", state, got, want)
		}
	}
}

func assertRectangle(t *testing.T, got, want rl.Rectangle) {
	t.Helper()
	if got != want {
		t.Fatalf("bounds = %+v, want %+v", got, want)
	}
}

func TestTETRAPOLFollowReturnsToCCHAfterAcquisitionTimeout(t *testing.T) {
	screen := NewMainScreen(nil)
	screen.frequencyHz = 390_000_000
	screen.spanHz = 500_000
	panel := NewTETRAPOLPanel(screen)
	panel.enabled = true
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	panel.handleChannelFollowAt(tetrapolruntime.Telemetry{ChannelAction: "follow", ChannelID: 2528, ChannelEvent: 1, CipherState: "clear"}, now)
	if !panel.following || panel.channelType != "TCH" || panel.followState != "ADQUIRIENDO TCH" {
		t.Fatalf("follow state = following %v type %s state %s", panel.following, panel.channelType, panel.followState)
	}
	if panel.tchCipherState != "clear" {
		t.Fatalf("followed cipher state = %q, want clear", panel.tchCipherState)
	}
	if screen.spanHz != 500_000 {
		t.Fatalf("TCH follow changed span to %d, want 500000", screen.spanHz)
	}
	panel.handleChannelFollowAt(tetrapolruntime.Telemetry{}, now.Add(tetrapolTCHTimeout))
	if panel.following || panel.channelType != "CCH" || screen.frequencyHz != 390_000_000 {
		t.Fatalf("timeout did not return to CCH: following=%v type=%s frequency=%d", panel.following, panel.channelType, screen.frequencyHz)
	}
	if panel.tchCipherState != "" {
		t.Fatalf("cipher state was not cleared on return: %q", panel.tchCipherState)
	}
	if screen.spanHz != 500_000 {
		t.Fatalf("return to CCH changed span to %d, want 500000", screen.spanHz)
	}
}

func TestTETRAPOLFollowTemporarilyBlocksStaleReactivation(t *testing.T) {
	screen := NewMainScreen(nil)
	screen.frequencyHz = 390_000_000
	panel := NewTETRAPOLPanel(screen)
	panel.enabled = true
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	panel.handleChannelFollowAt(tetrapolruntime.Telemetry{ChannelAction: "follow", ChannelID: 2528, ChannelEvent: 1}, now)
	panel.handleChannelFollowAt(tetrapolruntime.Telemetry{ChannelAction: "return", ChannelEvent: 2}, now.Add(time.Second))
	panel.handleChannelFollowAt(tetrapolruntime.Telemetry{ChannelAction: "follow", ChannelID: 2528, ChannelEvent: 3}, now.Add(2*time.Second))
	if panel.following || panel.channelType != "CCH" {
		t.Fatal("stale TCH activation bypassed rearm delay")
	}
	panel.handleChannelFollowAt(tetrapolruntime.Telemetry{ChannelAction: "follow", ChannelID: 2528, ChannelEvent: 4}, now.Add(5*time.Second))
	if !panel.following || panel.channelType != "TCH" {
		t.Fatal("fresh TCH activation was not accepted after rearm delay")
	}
}

func TestTETRAPOLFollowAcceptsExplicitEndWithReusedEventNumber(t *testing.T) {
	screen := NewMainScreen(nil)
	screen.frequencyHz = 390_000_000
	panel := NewTETRAPOLPanel(screen)
	panel.enabled = true
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	panel.handleChannelFollowAt(tetrapolruntime.Telemetry{ChannelAction: "follow", ChannelID: 2528, ChannelEvent: 2}, now)
	panel.handleChannelFollowAt(tetrapolruntime.Telemetry{ChannelAction: "return", ChannelEvent: 2, TCHSessionState: 2}, now.Add(time.Second))
	if panel.following || panel.channelType != "CCH" {
		t.Fatal("explicit TCH end was ignored because its event number matched the CCH activation")
	}
}

func TestTETRAPOLFollowIgnoresUnrelatedFrameTelemetryForTimeout(t *testing.T) {
	screen := NewMainScreen(nil)
	screen.frequencyHz = 390_000_000
	panel := NewTETRAPOLPanel(screen)
	panel.enabled = true
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	panel.handleChannelFollowAt(tetrapolruntime.Telemetry{ChannelAction: "follow", ChannelID: 2528, ChannelEvent: 1}, now)
	// Physical frames and CRC changes may continue on an idle TCH. They must
	// not be mistaken for voice and keep the follower away from the CCH.
	for second := 1; second <= 7 && panel.following; second++ {
		panel.handleChannelFollowAt(tetrapolruntime.Telemetry{
			Synchronized: true,
			ValidFrames:  uint64(second),
			CRCFailures:  uint64(second),
			LastRecord:   now.Add(time.Duration(second) * time.Second),
		}, now.Add(time.Duration(second)*time.Second))
	}
	if panel.following || panel.channelType != "CCH" {
		t.Fatal("unrelated TCH frame telemetry indefinitely postponed return to CCH")
	}
}

func TestTETRAPOLFollowExtendsTimeoutOnlyForAudioProgress(t *testing.T) {
	screen := NewMainScreen(nil)
	screen.frequencyHz = 390_000_000
	panel := NewTETRAPOLPanel(screen)
	panel.enabled = true
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	panel.handleChannelFollowAt(tetrapolruntime.Telemetry{ChannelAction: "follow", ChannelID: 2528, ChannelEvent: 1}, now)
	panel.handleChannelFollowAt(tetrapolruntime.Telemetry{AudioDecoded: 1, VoiceActive: true}, now.Add(5*time.Second))
	panel.handleChannelFollowAt(tetrapolruntime.Telemetry{AudioDecoded: 1}, now.Add(10*time.Second))
	if !panel.following {
		t.Fatal("active RP-CELP audio did not extend the TCH timeout")
	}
	panel.handleChannelFollowAt(tetrapolruntime.Telemetry{AudioDecoded: 1}, now.Add(11*time.Second))
	if panel.following || panel.channelType != "CCH" {
		t.Fatal("TCH did not return after six seconds without new audio")
	}
}

func TestTETRAPOLManualSpectrumTuneInvalidatesPreviousSession(t *testing.T) {
	panel := NewTETRAPOLPanel(NewMainScreen(nil))
	panel.enabled = true
	panel.channelType = "TCH"
	panel.following = true
	panel.followedChannel = 2528
	panel.controlFrequencyHz = 390_000_000
	panel.lastChannelEvent = 9
	panel.tchCipherState = "clear"
	panel.tchActivitySeen = true
	panel.blockedChannel = 2528
	panel.latestTelemetry = tetrapolruntime.Telemetry{CipherState: "clear", ValidFrames: 12}
	panel.networkSnapshot = panel.latestTelemetry

	panel.resetAfterManualTune()

	if panel.following || panel.followedChannel != -1 || panel.controlFrequencyHz != 0 || panel.lastChannelEvent != 0 {
		t.Fatalf("manual tune retained follow state: %+v", panel)
	}
	if panel.tchCipherState != "" || panel.latestTelemetry.CipherState != "" || panel.latestTelemetry.ValidFrames != 0 {
		t.Fatalf("manual tune retained telemetry or cipher state: cipher=%q telemetry=%+v", panel.tchCipherState, panel.latestTelemetry)
	}
	if panel.tchActivitySeen || panel.blockedChannel != -1 || panel.followState != "TCH MANUAL" {
		t.Fatalf("manual tune did not return to fresh acquisition state: activity=%v blocked=%d state=%q", panel.tchActivitySeen, panel.blockedChannel, panel.followState)
	}
}

func TestTETRAPOLAcquisitionFeedbackClearsWhenSynchronized(t *testing.T) {
	panel := NewTETRAPOLPanel(NewMainScreen(nil))
	panel.feedback = "Buscando sincronismo en la nueva frecuencia…"
	panel.reconcileAcquisitionFeedback(tetrapolruntime.Telemetry{})
	if panel.feedback == "" {
		t.Fatal("search feedback cleared before synchronization")
	}
	panel.reconcileAcquisitionFeedback(tetrapolruntime.Telemetry{Synchronized: true})
	if panel.feedback != "" {
		t.Fatalf("search feedback remained after synchronization: %q", panel.feedback)
	}
}

func TestTETRAPOLPanelUsesNarrowbandChannelStep(t *testing.T) {
	screen := NewMainScreen(nil)
	screen.tuningStepHz = 1_000
	screen.waterfallVisible = true
	panel := NewTETRAPOLPanel(screen)

	panel.Enter()
	if got, want := screen.tuningStepHz, int64(12_500); got != want {
		t.Fatalf("TETRAPOL tuning step = %d, want %d", got, want)
	}
	if screen.waterfallVisible {
		t.Fatal("TETRAPOL panel left waterfall controls active")
	}
	if got, want := len(panel.controls), 16; got != want {
		t.Fatalf("TETRAPOL controls = %d, want %d", got, want)
	}
	if panel.start.Bounds().X < toolContentX {
		t.Fatalf("start button is hidden outside tool content: x=%.0f", panel.start.Bounds().X)
	}
	panel.Leave()
}

func TestTETRAPOLAFCStartsEnabledWithCarrierEstimator(t *testing.T) {
	panel := NewTETRAPOLPanel(NewMainScreen(nil))
	if !panel.autoCenter.Enabled() || !panel.autoCenter.Active() {
		t.Fatal("AFC must start enabled now that the decoder estimates carrier frequency")
	}
}

func TestTETRAPOLVHFProfileProvidesTwoDocumentedRanges(t *testing.T) {
	screen := NewMainScreen(nil)
	panel := NewTETRAPOLPanel(screen)
	panel.SetVisible(true)
	panel.setBand("VHF")

	if panel.bandType != "VHF" || screen.tetrapolBand != "VHF" {
		t.Fatalf("profile = %q/%q; want VHF", panel.bandType, screen.tetrapolBand)
	}
	if got := panel.presets[0].Label(); got != "68–88 MHz" {
		t.Fatalf("first VHF preset = %q", got)
	}
	if got := panel.presets[1].Label(); got != "150–174 MHz" {
		t.Fatalf("second VHF preset = %q", got)
	}
	if panel.presets[2].Visible() {
		t.Fatal("third preset remains visible for VHF")
	}
	panel.activatePreset(1)
	if screen.frequencyHz != 162_000_000 {
		t.Fatalf("VHF preset tuned %d; want 162000000", screen.frequencyHz)
	}
}

func TestTETRAPOLBandPresetTunesChannelRaster(t *testing.T) {
	screen := NewMainScreen(nil)
	panel := NewTETRAPOLPanel(screen)
	panel.tune(420_000_000)
	if screen.frequencyHz != 420_000_000 || screen.centerFrequencyHz != 420_000_000 {
		t.Fatalf("preset tuning = %d/%d", screen.frequencyHz, screen.centerFrequencyHz)
	}
	if screen.spanHz != 250_000 || screen.tuningStepHz != 12_500 {
		t.Fatalf("preset raster = span %d step %d", screen.spanHz, screen.tuningStepHz)
	}
}

func TestTETRAPOLTenKHzPlanUpdatesReceiverRaster(t *testing.T) {
	screen := NewMainScreen(nil)
	panel := NewTETRAPOLPanel(screen)
	panel.setPlan(tetrapolPlanUHF10)
	if panel.planType != tetrapolPlanUHF10 || panel.bandType != "UHF" {
		t.Fatalf("plan = %q band = %q", panel.planType, panel.bandType)
	}
	if screen.tetrapolPlan != tetrapolPlanUHF10 || screen.tuningStepHz != 10_000 || screen.demodBandwidthHz != 10_000 {
		t.Fatalf("screen plan/raster = %q/%d/%d", screen.tetrapolPlan, screen.tuningStepHz, screen.demodBandwidthHz)
	}
}

func TestTETRAPOLPresetRefreshKeepsCollapsedWorkspaceLayout(t *testing.T) {
	screen := NewMainScreen(nil)
	panel := NewTETRAPOLPanel(screen)
	panel.SetVisible(true)
	screen.utilitiesCollapsed = true

	panel.setBand("UHF")
	assertRectangle(t, panel.presets[0].Bounds(), collapsedToolBounds(rl.Rectangle{X: 704, Y: toolY + 50, Width: 116, Height: 34}))
	assertRectangle(t, panel.presets[2].Bounds(), collapsedToolBounds(rl.Rectangle{X: 952, Y: toolY + 50, Width: 116, Height: 34}))

	panel.setBand("VHF")
	assertRectangle(t, panel.presets[0].Bounds(), collapsedToolBounds(rl.Rectangle{X: 704, Y: toolY + 50, Width: 178, Height: 34}))
	assertRectangle(t, panel.presets[1].Bounds(), collapsedToolBounds(rl.Rectangle{X: 890, Y: toolY + 50, Width: 178, Height: 34}))
}

func TestTETRAPOLNetworkModalFreezesAndResumesTelemetry(t *testing.T) {
	panel := NewTETRAPOLPanel(NewMainScreen(nil))
	panel.latestTelemetry = tetrapolruntime.Telemetry{
		TSDUCount: 7,
		Channels:  []tetrapolruntime.ChannelObservation{{ChannelID: 42}},
		Groups:    []tetrapolruntime.GroupObservation{{GroupID: 9}},
	}
	panel.openNetwork()
	if !panel.OverlayOpen() || !panel.networkLive || panel.networkTab != 0 {
		t.Fatalf("unexpected initial network modal state: open=%v live=%v tab=%d", panel.OverlayOpen(), panel.networkLive, panel.networkTab)
	}
	panel.networkLive = false
	panel.networkSnapshot = cloneTETRAPOLTelemetry(panel.latestTelemetry)
	panel.latestTelemetry.TSDUCount = 8
	panel.latestTelemetry.Channels[0].ChannelID = 99
	got := panel.networkTelemetry()
	if got.TSDUCount != 7 || got.Channels[0].ChannelID != 42 {
		t.Fatalf("paused modal did not retain snapshot: %+v", got)
	}
	panel.networkLive = true
	if got := panel.networkTelemetry(); got.TSDUCount != 8 || got.Channels[0].ChannelID != 99 {
		t.Fatalf("live modal did not resume current telemetry: %+v", got)
	}
}
