package tetra

import (
	"testing"
	"time"
)

func TestDestinationFilterDisabledAllowsUnknownCall(t *testing.T) {
	d := &Decoder{}
	d.SetDestinationFilter(false, nil)
	d.mu.RLock()
	allowed := d.destinationAllowedCallLocked(Call{}, false)
	d.mu.RUnlock()
	if !allowed {
		t.Fatal("disabled destination filter rejected an unknown call")
	}
}

func TestDestinationFilterEnabledEmptyFailsClosed(t *testing.T) {
	d := &Decoder{}
	d.SetDestinationFilter(true, nil)
	d.mu.RLock()
	allowed := d.destinationAllowedCallLocked(Call{SSI: 1001}, true)
	d.mu.RUnlock()
	if allowed {
		t.Fatal("enabled empty destination filter allowed audio")
	}
}

func TestDestinationFilterAllowsOnlyConfiguredSSI(t *testing.T) {
	d := &Decoder{}
	d.SetDestinationFilter(true, []uint32{1001, 5000, 1001, MaxSSI + 1})
	d.mu.RLock()
	allowed := d.destinationAllowedCallLocked(Call{SSI: 5000}, true)
	denied := d.destinationAllowedCallLocked(Call{SSI: 5001}, true)
	count := len(d.destinationFilter)
	d.mu.RUnlock()
	if !allowed || denied || count != 2 {
		t.Fatalf("unexpected allow-list result: allowed=%v denied=%v count=%d", allowed, denied, count)
	}
}

func TestDestinationFilterRejectsUnknownVoiceBeforeVocoder(t *testing.T) {
	d := &Decoder{onAudio: func([]float32) {}, calls: map[uint16]Call{}}
	d.SetDestinationFilter(true, []uint32{1001})
	d.mu.Lock()
	d.activeAudioSlot = 1
	d.activeAudioSeen = time.Now()
	d.processVoiceLocked(make([]byte, 432), SystemInfo{}, 0)
	rejected := d.audioRejectedFiltered
	active := d.activeAudioSlot
	d.mu.Unlock()
	if rejected != 1 || active != 0 {
		t.Fatalf("unknown voice was not rejected safely: rejected=%d active=%d", rejected, active)
	}
}

func TestDestinationFilterRevokesReusedSlotImmediately(t *testing.T) {
	now := time.Now()
	d := &Decoder{
		destinationFilterEnabled: true,
		destinationFilter:        map[uint32]bool{1001: true},
		calls: map[uint16]Call{
			1: {ID: 1, SSI: 1001, Active: true, LastSeen: now},
			2: {ID: 2, SSI: 5000, Active: true, LastSeen: now},
		},
		activeAudioSlot: 1,
		activeAudioSeen: now,
		activeCallID:    1,
	}
	d.mu.Lock()
	newCall := d.calls[2]
	if d.destinationAllowedCallLocked(newCall, true) {
		d.mu.Unlock()
		t.Fatal("unlisted replacement call was allowed")
	}
	d.revokeDestinationLocked(0)
	activeSlot, activeCall := d.activeAudioSlot, d.activeCallID
	d.mu.Unlock()
	if activeSlot != 0 || activeCall != 0 {
		t.Fatalf("reused slot retained previous authorization: slot=%d call=%d", activeSlot, activeCall)
	}
}
