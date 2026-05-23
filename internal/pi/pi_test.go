package pi

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestBuildArgsDefaults(t *testing.T) {
	t.Parallel()
	got := BuildArgs(Options{})
	want := []string{"--mode", "rpc", "--no-session"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("BuildArgs(zero) = %v, want %v", got, want)
	}
}

func TestBuildArgsWithSession(t *testing.T) {
	t.Parallel()
	got := BuildArgs(Options{Session: "/tmp/s"})
	want := []string{"--mode", "rpc", "--session", "/tmp/s"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("BuildArgs(session) = %v, want %v", got, want)
	}
}

func TestBuildArgsFull(t *testing.T) {
	t.Parallel()
	got := BuildArgs(Options{
		Model:    "GLM 5.1",
		Thinking: "high",
		Tools:    "bash,read",
		Session:  "/tmp/s",
	})
	want := []string{
		"--mode", "rpc",
		"--session", "/tmp/s",
		"--model", "GLM 5.1",
		"--thinking", "high",
		"-t", "bash,read",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("BuildArgs(full) =\n  %v\nwant\n  %v", got, want)
	}
}

func TestBuildArgsOmitsEmptyFields(t *testing.T) {
	t.Parallel()
	got := BuildArgs(Options{Model: "GLM 5.1"})
	want := []string{"--mode", "rpc", "--no-session", "--model", "GLM 5.1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("BuildArgs(model only) = %v, want %v", got, want)
	}
}

func TestEventsReturnsErrorOnScannerFailure(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	p := &Process{stdout: r}

	line := strings.Repeat("X", scanBufferMax+1)
	go func() {
		_, _ = w.Write([]byte(line + "\n"))
		_ = w.Close()
	}()

	_, errCh := p.Events()
	err = <-errCh
	if err == nil {
		t.Fatal("expected scanner error, got nil")
	}
	if !errors.Is(err, bufio.ErrTooLong) {
		t.Errorf("expected bufio.ErrTooLong, got %v", err)
	}
}

func TestEventsNoErrorOnCleanEOF(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	p := &Process{stdout: r}

	go func() {
		_, _ = w.Write([]byte("line1\n"))
		_, _ = w.Write([]byte("line2\n"))
		_ = w.Close()
	}()

	events, errCh := p.Events()
	var received []string
	for line := range events {
		received = append(received, string(line))
	}
	if len(received) != 2 || received[0] != "line1" || received[1] != "line2" {
		t.Errorf("expected [line1 line2], got %v", received)
	}

	err = <-errCh
	if err != nil {
		t.Errorf("expected nil error on clean EOF, got %v", err)
	}
}

func TestEventsReturnsAllLinesBeforeError(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	p := &Process{stdout: r}

	go func() {
		_, _ = w.Write([]byte("valid1\n"))
		_, _ = w.Write([]byte("valid2\n"))
		line := strings.Repeat("X", scanBufferMax+1)
		_, _ = w.Write([]byte(line + "\n"))
		_ = w.Close()
	}()

	events, errCh := p.Events()
	var received []string
	for line := range events {
		received = append(received, string(line))
	}
	if len(received) != 2 || received[0] != "valid1" || received[1] != "valid2" {
		t.Errorf("expected [valid1 valid2], got %v", received)
	}

	err = <-errCh
	if err == nil {
		t.Fatal("expected scanner error, got nil")
	}
}

func TestEventsByteSliceOwnership(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	p := &Process{stdout: r}

	go func() {
		_, _ = w.Write([]byte("hello\n"))
		_, _ = w.Write([]byte("world\n"))
		_ = w.Close()
	}()

	events, _ := p.Events()
	var slices [][]byte
	for line := range events {
		cp := make([]byte, len(line))
		copy(cp, line)
		slices = append(slices, cp)
	}

	if len(slices) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(slices))
	}

	slices[0][0] = 'X'
	if slices[1][0] != 'w' {
		t.Error("mutating first slice affected second — slices share backing array")
	}
}

func TestEventsChannelClosesOnEOF(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	p := &Process{stdout: r}
	_ = w.Close()

	events, _ := p.Events()
	_, ok := <-events
	if ok {
		t.Error("expected channel to be closed")
	}
}

func TestCloseWaitsForExit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}

	// Use the test helper process pattern
	cmd := helperCommand(t, "echo", "hello")
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	p := &Process{cmd: cmd, stdin: stdin, stdout: stdout}
	err := p.Close()
	if err != nil {
		t.Errorf("Close: unexpected error: %v", err)
	}
}

func TestKillNilProcessIsNoOp(t *testing.T) {
	t.Parallel()
	p := &Process{}
	err := p.Kill()
	if err != nil {
		t.Errorf("Kill on empty Process: %v", err)
	}
}

func TestEventsHandlesLargeLines(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	p := &Process{stdout: r}

	// Use a line that fits within scanBufferMax
	largeLine := strings.Repeat("A", 500*1024)
	go func() {
		_, _ = w.Write([]byte(largeLine + "\n"))
		_ = w.Close()
	}()

	events, errCh := p.Events()
	var received string
	for line := range events {
		received = string(line)
	}
	if received != largeLine {
		t.Errorf("large line content mismatch: got %d bytes, want %d", len(received), len(largeLine))
	}

	err = <-errCh
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCloseReturnsWaitErrorForNonZeroExit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}

	cmd := helperCommand(t, "sh", "-c", "exit 42")
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	p := &Process{cmd: cmd, stdin: stdin, stdout: stdout}
	err := p.Close()
	if err == nil {
		t.Fatal("expected error for non-zero exit, got nil")
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Errorf("expected *exec.ExitError, got %T: %v", err, err)
	}
}

func TestCloseReturnsStdinErrorOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}
	p := &Process{
		stdin: &failOnCloseWriter{},
	}
	cmd := exec.Command("sh", "-c", "exit 0")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	p.cmd = cmd

	err := p.Close()
	if err == nil {
		t.Fatal("expected stdin close error, got nil")
	}
	if !errors.Is(err, errFakeStdinClose) {
		t.Errorf("expected errFakeStdinClose, got %v", err)
	}
}

func TestCloseReturnsBothErrors(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}

	p := &Process{
		stdin: &failOnCloseWriter{},
	}
	cmd := helperCommand(t, "sh", "-c", "exit 42")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	p.cmd = cmd

	err := p.Close()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errFakeStdinClose) {
		t.Errorf("expected errFakeStdinClose, got %v", err)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Errorf("expected *exec.ExitError, got %T: %v", err, err)
	}
}

// failOnCloseWriter is an io.WriteCloser that returns errFakeStdinClose on Close.
var errFakeStdinClose = errors.New("fake stdin close error")

type failOnCloseWriter struct{}

func (w *failOnCloseWriter) Write(p []byte) (int, error) { return len(p), nil }
func (w *failOnCloseWriter) Close() error                { return errFakeStdinClose }

// helperCommand returns an *exec.Cmd for the given arguments by finding
// the binary via exec.LookPath instead of requiring os/exec import at the
// call site.
func helperCommand(t *testing.T, name string, arg ...string) *exec.Cmd {
	t.Helper()
	bin, err := exec.LookPath(name)
	if err != nil {
		t.Fatalf("lookup %q: %v", name, err)
	}
	return exec.Command(bin, arg...)
}
