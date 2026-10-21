package tetrapolruntime

import (
	"testing"
	"time"
)

func TestSubmitIQAppliesBackpressureInsteadOfDroppingAFullQueue(t *testing.T) {
	decoder := &Native{
		running: true,
		iq:      make(chan []float32, 1),
		stop:    make(chan struct{}),
	}
	decoder.iq <- []float32{0, 0}

	started := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		close(started)
		finished <- decoder.SubmitIQ([]float32{1, -1})
	}()
	<-started

	select {
	case err := <-finished:
		t.Fatalf("SubmitIQ returned while its queue was full: %v", err)
	case <-time.After(25 * time.Millisecond):
	}

	<-decoder.iq
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("SubmitIQ did not resume after queue capacity became available")
	}
	if decoder.droppedIQ != 0 {
		t.Fatalf("dropped IQ blocks = %d, want 0", decoder.droppedIQ)
	}
}

func TestNativeTelemetryKeepsVoiceActiveDuringPLC(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	decoder := &Native{iq: make(chan []float32, 1), lastTCHSession: -1}
	value := Telemetry{LogicalChannel: "TCH", AudioDecoded: 1}
	decoder.reconcileTelemetry(&value, now)
	decoder.telemetry = value
	if !value.VoiceActive {
		t.Fatal("a decoded RP-CELP frame did not activate voice")
	}

	value = Telemetry{LogicalChannel: "TCH", AudioDecoded: 1, AudioConcealed: 1}
	decoder.reconcileTelemetry(&value, now.Add(250*time.Millisecond))
	if !value.VoiceActive {
		t.Fatal("a PLC replacement frame did not preserve continuous voice activity")
	}
}

func TestNativeTelemetryPublishesExplicitTCHSessionEnd(t *testing.T) {
	decoder := &Native{iq: make(chan []float32, 1), lastTCHSession: -1}
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	value := Telemetry{LogicalChannel: "TCH", TCHSessionState: 0}
	decoder.reconcileTelemetry(&value, now)
	decoder.telemetry = value

	value = Telemetry{LogicalChannel: "TCH", TCHSessionState: 2}
	decoder.reconcileTelemetry(&value, now.Add(time.Second))
	if value.ChannelAction != "return" || value.ChannelEvent == 0 {
		t.Fatalf("explicit TCH end was not published: %+v", value)
	}
}
