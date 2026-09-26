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

func TestDialogRequestDoesNotDependOnCATCommandQueue(t *testing.T) {
	client := New()
	client.state.Running = true
	client.commands = make(chan command, 1)
	client.commands <- command{frequencyHz: 145_500_000}
	client.dialogs = make(chan bool, 1)

	client.ShowDialog(true)
	select {
	case visible := <-client.dialogs:
		if !visible {
			t.Fatal("dialog request asked to hide the window")
		}
	default:
		t.Fatal("dialog request was lost while CAT queue was full")
	}
}

func TestRepeatedDialogClicksCoalesceWithoutBlocking(t *testing.T) {
	client := New()
	client.state.Running = true
	client.dialogs = make(chan bool, 1)

	for range 20 {
		client.ShowDialog(true)
	}
	if len(client.dialogs) != 1 {
		t.Fatalf("repeated clicks left %d dialog requests, want one", len(client.dialogs))
	}
	if visible := <-client.dialogs; !visible {
		t.Fatal("coalesced dialog request did not retain the latest visible state")
	}
}
