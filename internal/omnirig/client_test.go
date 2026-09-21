package omnirig

import "testing"

func TestSelectRigIsValidatedAndRememberedBeforeStart(t *testing.T) {
	client := New()
	client.SelectRig(2)
	if got := client.State().SelectedRig; got != 2 {
		t.Fatalf("selected rig = %d, want 2", got)
	}
	client.SelectRig(3)
	if got := client.State().SelectedRig; got != 2 {
		t.Fatalf("invalid selection changed rig to %d", got)
	}
}
