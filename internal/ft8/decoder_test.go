package ft8

import (
	"math"
	"testing"
	"time"
)

func TestCaptureDownsamplesCompleteUTCSlot(t *testing.T) {
	d := New("missing")
	d.running.Store(true)
	d.enabled = true
	d.captureSlot = 1_700_000_000
	input := make([]float32, InputRate*SlotSeconds)
	for i := range input {
		input[i] = float32(math.Sin(2 * math.Pi * 1000 * float64(i) / InputRate))
	}
	d.ProcessAudio(input)
	select {
	case packet := <-d.queue:
		if len(packet.samples) != SlotSamples {
			t.Fatalf("slot samples = %d, want %d", len(packet.samples), SlotSamples)
		}
		if packet.start != 1_700_000_000 {
			t.Fatalf("slot start = %d", packet.start)
		}
	default:
		t.Fatal("complete slot was not queued")
	}
}

func TestResetCaptureTargetsNextUTCBoundary(t *testing.T) {
	d := New("missing")
	now := time.Unix(1_700_000_007, 500_000_000)
	d.resetCaptureLocked(now)
	if d.captureSlot%SlotSeconds != 0 || d.captureSlot <= now.Unix() {
		t.Fatalf("capture slot %d is not the next UTC boundary", d.captureSlot)
	}
	if !d.status.Waiting || !d.status.WaitUntil.Equal(time.Unix(d.captureSlot, 0)) || d.status.Progress != 0 {
		t.Fatalf("reset did not arm a clean slot: %+v", d.status)
	}
}

func TestCaptureDiscardsPartialAudioBeforeArmedBoundary(t *testing.T) {
	d := New("missing")
	d.running.Store(true)
	d.enabled = true
	start := time.Unix(1_700_000_000, 0)
	d.captureSlot = start.Add(10 * time.Millisecond).Unix()
	d.captureArmedAt = start
	d.audioCursor = start
	d.waitUntil = start.Add(10 * time.Millisecond)
	d.status.Waiting = true
	d.processAudioAt(make([]float32, 960), start.Add(20*time.Millisecond)) // Only the final 10 ms belong to the slot.
	status := d.Snapshot()
	if status.Waiting || status.State != "ESCUCHANDO" {
		t.Fatalf("decoder did not enter the clean slot: %+v", status)
	}
	// 480 accepted input samples downsample by four to 120 decoder samples.
	want := float32(120) / SlotSamples
	if math.Abs(float64(status.Progress-want)) > 1e-7 {
		t.Fatalf("progress = %.8f; want %.8f", status.Progress, want)
	}
}

func TestDepthProfilesMapToBoundedRuntimeBudgets(t *testing.T) {
	for _, test := range []struct{ depth, budget string }{
		{DepthFast, "2"}, {DepthBalanced, "5"}, {DepthDeep, "9"}, {"invalid", "5"},
	} {
		if got := depthBudget(test.depth); got != test.budget {
			t.Fatalf("depthBudget(%q) = %q, want %q", test.depth, got, test.budget)
		}
	}
}

func TestClockOffsetUsesMedianAndRejectsOutliers(t *testing.T) {
	decodes := []Decode{{DT: .2}, {DT: .4}, {DT: 2.4}, {DT: 9}, {DT: -.1}}
	offset, samples := estimateClockOffset(decodes, 20)
	if samples != 4 || math.Abs(offset-.3) > .0001 {
		t.Fatalf("offset = %.3f from %d samples, want .300 from 4", offset, samples)
	}
}

func TestInputContinuityDetectsARealPauseButAllowsCallbackCadence(t *testing.T) {
	previous := time.Unix(1_700_000_000, 0)
	if drift, resync := inputDiscontinuity(previous, 20*time.Millisecond, previous.Add(45*time.Millisecond)); resync {
		t.Fatalf("normal callback jitter requested resync: %v", drift)
	}
	drift, resync := inputDiscontinuity(previous, 20*time.Millisecond, previous.Add(900*time.Millisecond))
	if !resync || drift != 880*time.Millisecond {
		t.Fatalf("pause = %v, %v; want 880ms resync", drift, resync)
	}
}

func TestCursorContinuityDetectsAccumulatedLagAndLead(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	if drift, resync := cursorDiscontinuity(now.Add(-400*time.Millisecond), now); resync || drift != 400*time.Millisecond {
		t.Fatalf("normal cursor offset = %v, %v", drift, resync)
	}
	if drift, resync := cursorDiscontinuity(now.Add(-700*time.Millisecond), now); !resync || drift != 700*time.Millisecond {
		t.Fatalf("accumulated lag = %v, %v; want resync", drift, resync)
	}
	if drift, resync := cursorDiscontinuity(now.Add(700*time.Millisecond), now); !resync || drift != -700*time.Millisecond {
		t.Fatalf("buffer lead = %v, %v; want resync", drift, resync)
	}
}

func TestQueueKeepsNewestCompleteSlots(t *testing.T) {
	d := New("missing")
	d.enqueueLatest(slotPacket{start: 15})
	d.enqueueLatest(slotPacket{start: 30})
	d.enqueueLatest(slotPacket{start: 45})
	first, second := <-d.queue, <-d.queue
	if first.start != 30 || second.start != 45 || d.dropped.Load() != 1 {
		t.Fatalf("queue contains %d, %d; dropped=%d", first.start, second.start, d.dropped.Load())
	}
}

func TestProcessAudioResynchronizesAfterInputPause(t *testing.T) {
	d := New("missing")
	d.running.Store(true)
	d.enabled = true
	d.captureSlot = time.Now().UTC().Truncate(15 * time.Second).Unix()
	d.lastInputAt = time.Now().Add(-time.Second)
	d.lastInputDuration = 20 * time.Millisecond
	d.processAudioAt(make([]float32, 960), time.Now())
	status := d.Snapshot()
	if status.ResyncCount != 1 || status.State != "ESPERANDO CICLO" || !status.Waiting {
		t.Fatalf("status after pause = %+v", status)
	}
	if status.Slot.Unix()%SlotSeconds != 0 || status.Progress != 0 {
		t.Fatalf("resynchronized capture = %v progress %.3f", status.Slot, status.Progress)
	}
}
