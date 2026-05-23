// Package pi launches a `pi --mode rpc` subprocess, sends the user prompt,
// and exposes the resulting JSON event stream as a channel of raw lines.
//
// The subprocess inherits the parent's environment and is wired so that:
//   - the prompt is written to stdin as a single JSON line and stdin is then
//     held open until Close is called;
//   - stdout is line-scanned and forwarded over the Events channel;
//   - stderr is forwarded verbatim to the writer supplied to Start.
//
// Cancellation is driven by the context passed to Start: when the context
// is canceled the subprocess receives a SIGKILL (Go's default for
// exec.CommandContext).
package pi

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
)

// Buffer sizes for the stdout scanner. The initial buffer is small but the
// scanner will grow it on demand up to scanBufferMax before failing.
const (
	scanBufferInit = 64 * 1024
	scanBufferMax  = 1024 * 1024
)

// Options configures a pi invocation. Every field is optional.
type Options struct {
	// Model is forwarded as --model. Empty value omits the flag.
	Model string
	// Thinking is forwarded as --thinking. Empty value omits the flag.
	Thinking string
	// Tools is forwarded as -t (comma-separated allowlist). Empty value
	// omits the flag.
	Tools string
	// Session is forwarded as --session. Empty value sends --no-session
	// so that the subprocess is fully isolated by default.
	Session string
}

// BuildArgs returns the argv (excluding the binary name) for invoking pi
// with the given options. Exposed for testing.
func BuildArgs(opts Options) []string {
	args := []string{"--mode", "rpc"}
	if opts.Session != "" {
		args = append(args, "--session", opts.Session)
	} else {
		args = append(args, "--no-session")
	}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	if opts.Thinking != "" {
		args = append(args, "--thinking", opts.Thinking)
	}
	if opts.Tools != "" {
		args = append(args, "-t", opts.Tools)
	}
	return args
}

// Process is a running pi subprocess together with its event stream.
type Process struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

// Start launches `pi --mode rpc` with the given options, writes the prompt
// to its stdin, and returns a handle that can be used to consume events.
// stderr from the subprocess is copied to stderrOut in a background
// goroutine that exits when the subprocess closes its stderr.
func Start(ctx context.Context, opts Options, prompt string, stderrOut io.Writer) (*Process, error) {
	cmd := exec.CommandContext(ctx, "pi", BuildArgs(opts)...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start pi: %w", err)
	}

	go func() {
		_, _ = io.Copy(stderrOut, stderrPipe)
	}()

	enc := json.NewEncoder(stdin)
	if err := enc.Encode(map[string]any{
		"type":    "prompt",
		"message": prompt,
	}); err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("send prompt: %w", err)
	}

	return &Process{cmd: cmd, stdin: stdin, stdout: stdout}, nil
}

// Events returns two channels: one that emits each line of pi's stdout as a
// freshly-copied byte slice, and one that receives a single scanner error if
// the scan loop encounters one (e.g. line exceeds buffer limit). The data
// channel is closed when stdout reaches EOF. The error channel is closed
// after the scan loop regardless of whether an error occurred.
//
// The returned slices are owned by the caller and survive the next read.
func (p *Process) Events() (<-chan []byte, <-chan error) {
	ch := make(chan []byte, 32)
	errCh := make(chan error, 1)
	go func() {
		defer close(ch)
		defer close(errCh)
		sc := bufio.NewScanner(p.stdout)
		sc.Buffer(make([]byte, 0, scanBufferInit), scanBufferMax)
		for sc.Scan() {
			line := sc.Bytes()
			cp := make([]byte, len(line))
			copy(cp, line)
			ch <- cp
		}
		if err := sc.Err(); err != nil {
			errCh <- fmt.Errorf("stdout scanner: %w", err)
		} else {
			errCh <- nil
		}
	}()
	return ch, errCh
}

// Close closes the subprocess's stdin and waits for it to exit. The
// returned error combines both the stdin close error and the wait error
// (e.g. non-zero exit) via errors.Join.
func (p *Process) Close() error {
	return errors.Join(p.stdin.Close(), p.cmd.Wait())
}

// Kill terminates the subprocess immediately. It is safe to call after
// Close (or if the process never started successfully).
func (p *Process) Kill() error {
	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}
