package tetra

import "testing"

func TestClearPreservesRunningStreamCoordinates(t *testing.T) {
	d := New(2_048_000, "", nil)
	d.Configure(true)
	d.mu.Lock()
	d.totalBits = 987_654
	d.lastSyncBit = 987_100
	d.mu.Unlock()

	d.Clear()
	d.mu.RLock()
	totalBits, lastSyncBit, running := d.totalBits, d.lastSyncBit, d.running
	d.mu.RUnlock()
	if totalBits != 987_654 || lastSyncBit != 987_100 || !running {
		t.Fatalf("Clear changed live stream state: total=%d sync=%d running=%v", totalBits, lastSyncBit, running)
	}

	// Starting a new decoder loop is the operation that owns a new coordinate
	// epoch and must reset both values together.
	d.Configure(false)
	d.Configure(true)
	d.mu.RLock()
	totalBits, lastSyncBit = d.totalBits, d.lastSyncBit
	d.mu.RUnlock()
	if totalBits != 0 || lastSyncBit != 0 {
		t.Fatalf("new stream retained old coordinates: total=%d sync=%d", totalBits, lastSyncBit)
	}
	d.Configure(false)
}
