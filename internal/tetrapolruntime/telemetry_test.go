package tetrapolruntime

import (
	"testing"
	"time"
)

func TestTelemetryTracksFramesAndTSDU(t *testing.T) {
	var tracker telemetryTracker
	now := time.Unix(100, 0)
	tracker.update([]byte(`{"event":"frame","rx_offs":440,"rx_time":"2026-09-20T10-00-00.000000","frame":{"frame_no":17,"state":"ok","type":"VOICE","bits_fixed":2,"asb":[1,0]}}`), now)
	tracker.update([]byte(`{"event":"frame","frame":{"state":"bad_CRC","bits_fixed":1}}`), now)
	tracker.update([]byte(`{"event":"scr","scr":67}`), now)
	tracker.update([]byte(`{"event":"tsdu","tsdu":{"frame_no":18,"log_ch":"VCH","addr":{"z":1,"y":2,"x":3},"tsap_id":4,"cipher":{"state":"clear","key_type":0,"key_index":0}}}`), now)
	got := tracker.snapshot(now.Add(100 * time.Millisecond))
	if !got.Synchronized || !got.VoiceActive || !got.FrameKnown || got.FrameNumber != 18 || got.LogicalChannel != "VCH" {
		t.Fatalf("unexpected telemetry: %+v", got)
	}
	if got.ValidFrames != 1 || got.CRCFailures != 1 || got.BitsFixed != 2 {
		t.Fatalf("unexpected counters: %+v", got)
	}
	if !got.SCRKnown || got.SCR != 67 || !got.ASBKnown || got.ASB != [2]int{1, 0} || got.RXOffset != 440 {
		t.Fatalf("missing protocol details: %+v", got)
	}
	if got.QualityKnown || got.QualityPercent != 0 || got.CRCPerSecond != .2 || len(got.RecentEvents) != 5 || got.CipherState != "clear" {
		t.Fatalf("unexpected derived telemetry: %+v", got)
	}
}

func TestTelemetryQualityUsesRecentFrameWindow(t *testing.T) {
	var tracker telemetryTracker
	now := time.Unix(100, 0)
	for i := 0; i < 4; i++ {
		tracker.update([]byte(`{"event":"frame","frame":{"state":"ok"}}`), now)
	}
	tracker.update([]byte(`{"event":"frame","frame":{"state":"bad_CRC"}}`), now)
	if got := tracker.snapshot(now); !got.QualityKnown || got.QualityPercent != 80 {
		t.Fatalf("recent quality = %+v, want known 80%%", got)
	}
	if got := tracker.snapshot(now.Add(6 * time.Second)); got.QualityKnown || got.QualityPercent != 0 {
		t.Fatalf("expired quality remains visible: %+v", got)
	}
}

func TestTelemetryClearOnlyVoicePolicy(t *testing.T) {
	var tracker telemetryTracker
	if tracker.voiceAllowed(true) {
		t.Fatal("unknown cipher state was accepted by clear-only policy")
	}
	tracker.update([]byte(`{"event":"tsdu","tsdu":{"cipher":{"state":"clear","key_type":0,"key_index":0}}}`), time.Now())
	if !tracker.voiceAllowed(true) {
		t.Fatal("clear call was rejected")
	}
	tracker.update([]byte(`{"event":"tsdu","tsdu":{"cipher":{"state":"encrypted","key_type":1,"key_index":2}}}`), time.Now())
	if tracker.voiceAllowed(true) {
		t.Fatal("encrypted call was accepted")
	}
	if !tracker.voiceAllowed(false) {
		t.Fatal("disabled clear-only policy rejected voice")
	}
}

func TestTelemetryTracksChannelFollowingActions(t *testing.T) {
	var tracker telemetryTracker
	now := time.Unix(100, 0)
	tracker.update([]byte(`{"event":"tsdu","tsdu":{"channel_action":{"action":"follow","kind":"group","channel_id":2882}}}`), now)
	got := tracker.snapshot(now)
	if got.ChannelAction != "follow" || got.ChannelKind != "group" || got.ChannelID != 2882 || got.ChannelEvent != 1 {
		t.Fatalf("unexpected follow action: %+v", got)
	}
	tracker.update([]byte(`{"event":"tsdu","tsdu":{"channel_action":{"action":"return"}}}`), now)
	got = tracker.snapshot(now)
	if got.ChannelAction != "return" || got.ChannelEvent != 2 {
		t.Fatalf("unexpected return action: %+v", got)
	}
}

func TestTelemetrySyncAndVoiceExpire(t *testing.T) {
	var tracker telemetryTracker
	now := time.Unix(100, 0)
	tracker.update([]byte(`{"event":"frame","frame":{"state":"ok","type":"VOICE"}}`), now)
	got := tracker.snapshot(now.Add(3 * time.Second))
	if got.Synchronized || got.VoiceActive {
		t.Fatalf("stale telemetry remained active: %+v", got)
	}
}

func TestTelemetryUsesExplicitSyncEvents(t *testing.T) {
	var tracker telemetryTracker
	now := time.Unix(100, 0)
	tracker.update([]byte(`{"event":"sync","state":"acquired","rx_offs":123}`), now)
	if got := tracker.snapshot(now.Add(3 * time.Second)); !got.Synchronized || got.RXOffset != 123 {
		t.Fatalf("explicit acquisition not retained: %+v", got)
	}
	tracker.update([]byte(`{"event":"sync","state":"lost","rx_offs":456}`), now.Add(4*time.Second))
	got := tracker.snapshot(now.Add(4 * time.Second))
	if got.Synchronized || got.RXOffset != 456 {
		t.Fatalf("explicit loss not applied: %+v", got)
	}
}
