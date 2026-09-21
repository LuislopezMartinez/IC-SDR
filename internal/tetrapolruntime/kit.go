package tetrapolruntime

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// Kit is the external tetrapol_dump process. It accepts demodulated hard bits
// as bytes 0/1 on stdin and emits one frame_json record per line on stdout.
type Kit struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	done    chan error
	running bool
}

func (kit *Kit) Start(executable string, args []string, onRecord func([]byte)) error {
	kit.mu.Lock()
	defer kit.mu.Unlock()
	if kit.running {
		return nil
	}
	if executable == "" {
		return fmt.Errorf("tetrapol_dump executable is not configured")
	}
	cmd := exec.Command(executable, args...)
	applyProcessWindowPolicy(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("create tetrapol_dump input: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create tetrapol_dump output: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start tetrapol_dump: %w", err)
	}
	kit.cmd, kit.stdin, kit.running = cmd, stdin, true
	kit.done = make(chan error, 1)
	go readKitRecords(stdout, onRecord)
	go func() { kit.done <- cmd.Wait() }()
	return nil
}

func readKitRecords(reader io.Reader, onRecord func([]byte)) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024), 128*1024)
	for scanner.Scan() {
		if onRecord != nil {
			onRecord(append([]byte(nil), scanner.Bytes()...))
		}
	}
}

func (kit *Kit) SubmitBits(bits []byte) error {
	if len(bits) == 0 {
		return nil
	}
	kit.mu.Lock()
	defer kit.mu.Unlock()
	if !kit.running || kit.stdin == nil {
		return fmt.Errorf("tetrapol_dump is not running")
	}
	for _, bit := range bits {
		if bit > 1 {
			return fmt.Errorf("invalid GMSK hard bit %d", bit)
		}
	}
	if _, err := kit.stdin.Write(bits); err != nil {
		return fmt.Errorf("send bits to tetrapol_dump: %w", err)
	}
	return nil
}

func (kit *Kit) Stop() error {
	kit.mu.Lock()
	if !kit.running {
		kit.mu.Unlock()
		return nil
	}
	stdin, done := kit.stdin, kit.done
	kit.stdin, kit.running = nil, false
	kit.mu.Unlock()
	_ = stdin.Close()
	return <-done
}

func (kit *Kit) Running() bool {
	kit.mu.Lock()
	defer kit.mu.Unlock()
	return kit.running
}
