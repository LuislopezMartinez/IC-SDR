package tetra

import (
	"testing"
	"time"
)

func TestAudioPolicyNeverRoutesConfirmedEncryption(t *testing.T) {
	d := &Decoder{clearAudioOnly: false, allowUnconfirmedAudio: true}
	d.slotEncrypted[1] = 1
	if got := d.audioRouteLocked(1); got != audioRouteEncrypted {
		t.Fatalf("confirmed encryption route = %v, want blocked", got)
	}
}

func TestAudioPolicyNeedsConfirmationOrExplicitUnknownOptIn(t *testing.T) {
	d := &Decoder{clearAudioOnly: true}
	d.slotEncrypted[0] = -1
	d.slotUnknownSince[0] = time.Now().Add(-unknownAudioGrace - time.Millisecond)
	if got := d.audioRouteLocked(0); got != audioRoutePending {
		t.Fatalf("strict unknown route = %v, want pending", got)
	}
	d.allowUnconfirmedAudio = true
	if got := d.audioRouteLocked(0); got != audioRouteUnconfirmed {
		t.Fatalf("opted-in unknown route = %v, want unconfirmed", got)
	}
	d.slotEncrypted[0] = 0
	if got := d.audioRouteLocked(0); got != audioRouteClear {
		t.Fatalf("clear route = %v, want clear", got)
	}
}

func TestPendingAudioFlushesOnlyAfterClearConfirmation(t *testing.T) {
	var delivered [][]float32
	d := &Decoder{clearAudioOnly: true, onAudio: func(pcm []float32) {
		delivered = append(delivered, append([]float32(nil), pcm...))
	}}
	d.slotEncrypted[0] = -1
	d.slotUnknownSince[0] = time.Now()
	d.emitAudioLocked(0, []float32{0.1, 0.2})
	if len(delivered) != 0 || len(d.pendingAudio[0]) != 2 {
		t.Fatalf("ambiguous audio escaped before confirmation: delivered=%d pending=%d", len(delivered), len(d.pendingAudio[0]))
	}
	d.slotEncrypted[0] = 0
	d.emitAudioLocked(0, []float32{0.3})
	if len(delivered) != 2 || len(delivered[0]) != 2 || len(delivered[1]) != 1 {
		t.Fatalf("clear confirmation did not flush buffered audio: %#v", delivered)
	}
}
