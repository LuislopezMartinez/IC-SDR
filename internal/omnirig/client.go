package omnirig

import (
	"path/filepath"
	"sync"
	"time"
)

const (
	ModeUnknown = ""
	ModeCW      = "CW"
	ModeUSB     = "USB"
	ModeLSB     = "LSB"
	ModeDigital = "DIGITAL"
	ModeAM      = "AM"
	ModeFM      = "FM"
)

type State struct {
	Running       bool
	Online        bool
	FrequencyHz   int64
	Mode          string
	RigType       string
	Status        string
	Error         string
	Updated       time.Time
	SelectedRig   int
	Rig1          RigSummary
	Rig2          RigSummary
	TXReadable    bool
	Transmitting  bool
	DialogOpening bool
}

type RigSummary struct {
	Online       bool
	RigType      string
	Status       string
	TXReadable   bool
	Transmitting bool
}

type command struct {
	frequencyHz int64
	mode        string
	selectRig   int
}

// Client owns the Omni-Rig COM apartment and exposes non-blocking state and
// commands to the UI. All COM calls remain on its dedicated worker thread.
type Client struct {
	mu          sync.RWMutex
	state       State
	commands    chan command
	dialogs     chan bool
	stop        chan struct{}
	done        chan struct{}
	executable  string
	selectedRig int
}

// New creates an Omni-Rig client. runtimeRoot is the portable Windows runtime
// directory; the bundled server is expected below omnirig/OmniRig.exe.
func New(runtimeRoot ...string) *Client {
	client := &Client{selectedRig: 1}
	if len(runtimeRoot) > 0 && runtimeRoot[0] != "" {
		client.executable = filepath.Join(runtimeRoot[0], "omnirig", "OmniRig.exe")
	}
	return client
}

func (client *Client) Start() {
	client.mu.Lock()
	if client.state.Running {
		client.mu.Unlock()
		return
	}
	client.state = State{Running: true, Status: "Iniciando Omni-Rig…", SelectedRig: client.selectedRig}
	client.commands = make(chan command, 8)
	client.dialogs = make(chan bool, 1)
	client.stop = make(chan struct{})
	client.done = make(chan struct{})
	client.mu.Unlock()
	go client.run()
}

func (client *Client) Stop() {
	client.mu.RLock()
	stop, done, running := client.stop, client.done, client.state.Running
	client.mu.RUnlock()
	if !running || stop == nil {
		return
	}
	select {
	case <-stop:
	default:
		close(stop)
	}
	if done != nil {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}
}

func (client *Client) State() State {
	client.mu.RLock()
	defer client.mu.RUnlock()
	return client.state
}

func (client *Client) SetFrequency(frequencyHz int64) {
	if frequencyHz <= 0 || frequencyHz > 2_147_483_647 {
		return
	}
	client.enqueue(command{frequencyHz: frequencyHz})
}

func (client *Client) SetMode(mode string) { client.enqueue(command{mode: mode}) }

// ShowDialog explicitly controls Omni-Rig's own setup window. Normal CAT use
// keeps it hidden; the integrated tool exposes this only as an advanced escape
// hatch for settings that Omni-Rig does not publish through COM.
func (client *Client) ShowDialog(visible bool) {
	if visible {
		// This Win32 path is deliberately attempted before COM. It can restore an
		// existing Omni-Rig form even while its automation worker is temporarily
		// busy, and repeated clicks remain harmless.
		requestOmniRigWindow()
	}
	client.mu.RLock()
	dialogs, running := client.dialogs, client.state.Running
	client.mu.RUnlock()
	if !running || dialogs == nil {
		return
	}
	client.mu.Lock()
	client.state.DialogOpening = visible
	client.state.Error = ""
	client.mu.Unlock()
	// Dialog requests must never compete with high-rate tuning commands. Keep
	// the newest request in its own channel so CONFIG. AVANZADA cannot be lost
	// when the CAT queue is busy.
	select {
	case dialogs <- visible:
	default:
		select {
		case <-dialogs:
		default:
		}
		dialogs <- visible
	}
}

func (client *Client) SelectRig(number int) {
	if number != 1 && number != 2 {
		return
	}
	client.mu.Lock()
	client.selectedRig = number
	running := client.state.Running
	client.state.SelectedRig = number
	client.mu.Unlock()
	if running {
		client.enqueue(command{selectRig: number})
	}
}

func (client *Client) enqueue(value command) {
	client.mu.RLock()
	commands, running := client.commands, client.state.Running
	client.mu.RUnlock()
	if !running || commands == nil {
		return
	}
	select {
	case commands <- value:
	default:
		// Prefer the newest dial value when rapid UI gestures outrun CAT.
		select {
		case <-commands:
		default:
		}
		select {
		case commands <- value:
		default:
		}
	}
}

func (client *Client) publish(state State) {
	client.mu.Lock()
	state.Running = client.state.Running
	client.state = state
	client.mu.Unlock()
}

func (client *Client) finish(err error) {
	client.mu.Lock()
	client.state.Running = false
	client.state.Online = false
	if err != nil {
		client.state.Error = err.Error()
		client.state.Status = "Omni-Rig no disponible"
	}
	done := client.done
	client.mu.Unlock()
	if done != nil {
		close(done)
	}
}
