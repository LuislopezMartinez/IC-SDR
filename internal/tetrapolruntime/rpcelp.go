package tetrapolruntime

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// RPCELP is a supervised external decoder. The command must read one 120-bit
// validated clear VOICE frame per text line from stdin and write signed
// PCM16LE mono at 8 kHz to stdout, matching the maintained rpcelp fork.
type RPCELP struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	done    chan error
	running bool
}

func (decoder *RPCELP) Start(executable string, args []string, emit func([]float32)) error {
	decoder.mu.Lock()
	defer decoder.mu.Unlock()
	if decoder.running {
		return nil
	}
	if executable == "" {
		return fmt.Errorf("RP-CELP executable is not configured")
	}
	cmd := exec.Command(executable, args...)
	applyProcessWindowPolicy(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("create RP-CELP input: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create RP-CELP output: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start RP-CELP: %w", err)
	}
	decoder.cmd, decoder.stdin, decoder.running = cmd, stdin, true
	decoder.done = make(chan error, 1)
	go decoder.readPCM(stdout, emit)
	go func() { decoder.done <- cmd.Wait() }()
	return nil
}

func (decoder *RPCELP) readPCM(reader io.Reader, emit func([]float32)) {
	if emit == nil {
		_, _ = io.Copy(io.Discard, reader)
		return
	}
	buffer := make([]byte, 4096)
	carry := make([]byte, 0, 1)
	for {
		read, err := reader.Read(buffer)
		if read > 0 {
			chunk := append(carry, buffer[:read]...)
			complete := len(chunk) &^ 1
			if complete > 0 {
				emit(PCM16LEToFloat32(chunk[:complete]))
			}
			carry = append(carry[:0], chunk[complete:]...)
		}
		if err != nil {
			return
		}
	}
}

func (decoder *RPCELP) SubmitKitJSON(line []byte) (bool, error) {
	bits, accepted, err := VoiceBitsFromKitJSON(line)
	if err != nil || !accepted {
		return accepted, err
	}
	decoder.mu.Lock()
	defer decoder.mu.Unlock()
	if !decoder.running || decoder.stdin == nil {
		return false, fmt.Errorf("RP-CELP is not running")
	}
	if _, err := io.WriteString(decoder.stdin, bits+"\n"); err != nil {
		return false, fmt.Errorf("send VOICE frame to RP-CELP: %w", err)
	}
	return true, nil
}

func (decoder *RPCELP) Stop() error {
	decoder.mu.Lock()
	if !decoder.running {
		decoder.mu.Unlock()
		return nil
	}
	stdin, done := decoder.stdin, decoder.done
	decoder.stdin, decoder.running = nil, false
	decoder.mu.Unlock()
	_ = stdin.Close()
	return <-done
}

func (decoder *RPCELP) Running() bool {
	decoder.mu.Lock()
	defer decoder.mu.Unlock()
	return decoder.running
}

// Scanner is exported for callers that wish to parse line-oriented
// tetrapol-kit JSON output without retaining arbitrarily large records.
func Scanner(reader io.Reader) *bufio.Scanner { return bufio.NewScanner(reader) }
