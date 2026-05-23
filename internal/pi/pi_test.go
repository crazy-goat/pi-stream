package pi

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") == "1" {
		fakePiMain()
		return
	}
	os.Exit(m.Run())
}

// fakePiMain simulates pi --mode rpc for testing.
// Controlled by FAKE_PI_MODE env var:
//   - "echo" (default): read stdin, write each line to stdout
//   - "stderr": write each line to stderr instead of stdout
//   - "longrunning": read stdin until EOF (never exits on its own)
//   - "exit42": exit with code 42
func fakePiMain() {
	scanner := bufio.NewScanner(os.Stdin)
	mode := os.Getenv("FAKE_PI_MODE")
	switch mode {
	case "longrunning":
		_, _ = io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	case "exit42":
		os.Exit(42)
	case "stderr":
		for scanner.Scan() {
			_, _ = fmt.Fprintln(os.Stderr, scanner.Text())
		}
		os.Exit(0)
	default:
		for scanner.Scan() {
			_, _ = fmt.Fprintln(os.Stdout, scanner.Text())
		}
		os.Exit(0)
	}
}

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

	// Use a 5 MB line to verify the 10 MB buffer handles large tool outputs.
	largeLine := strings.Repeat("A", 5*1024*1024)
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

func BenchmarkEventsLargeLine(b *testing.B) {
	for _, size := range []int{1, 5, 10} {
		b.Run(fmt.Sprintf("%dMB", size), func(b *testing.B) {
			// Subtract 2 to leave room for "\n" so even the 10MB case
			// stays within scanBufferMax.
			line := []byte(strings.Repeat("X", size*1024*1024-2) + "\n")
			b.ReportAllocs()
			b.SetBytes(int64(len(line)))

			b.StopTimer()
			for i := 0; i < b.N; i++ {
				r, w, err := os.Pipe()
				if err != nil {
					b.Fatal(err)
				}
				p := &Process{stdout: r}

				go func() {
					_, _ = w.Write(line)
					_ = w.Close()
				}()

				b.StartTimer()
				events, errCh := p.Events()
				for range events {
				}
				if err := <-errCh; err != nil {
					b.Error(err)
				}
				b.StopTimer()
				_ = r.Close()
			}
		})
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

func TestStartReturnsErrorWhenBinaryNotFound(t *testing.T) {
	t.Parallel()

	orig := piBinary
	piBinary = "/nonexistent/pi-binary"
	defer func() { piBinary = orig }()

	ctx := context.Background()
	var buf bytes.Buffer
	_, err := Start(ctx, Options{}, "prompt", &buf)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "start pi") {
		t.Errorf("expected error containing 'start pi', got: %v", err)
	}
}

func TestKillTerminatesProcess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}

	cmd := helperCommand(t, "sh", "-c", "while true; do sleep 1; done")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	p := &Process{cmd: cmd, stdin: stdin, stdout: stdout}

	if err := p.Kill(); err != nil {
		t.Errorf("Kill: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for process to exit after kill")
	}
}

func TestStderrForwarding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}

	orig := piBinary
	piBinary = os.Args[0]
	defer func() { piBinary = orig }()

	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("FAKE_PI_MODE", "stderr")

	ctx := context.Background()
	var stderrBuf bytes.Buffer
	p, err := Start(ctx, Options{}, "hello", &stderrBuf)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	err = p.Close()
	t.Logf("Close: %v", err)

	if stderrBuf.Len() == 0 {
		t.Error("expected stderr output, got none")
	}
	if !strings.Contains(stderrBuf.String(), "hello") {
		t.Errorf("expected stderr to contain 'hello', got: %s", stderrBuf.String())
	}
}

func TestStartToCloseFullLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}

	orig := piBinary
	piBinary = os.Args[0]
	defer func() { piBinary = orig }()

	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("FAKE_PI_MODE", "echo")

	ctx := context.Background()
	var stderrBuf bytes.Buffer
	p, err := Start(ctx, Options{}, "hello", &stderrBuf)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	events, errCh := p.Events()

	eventsDone := make(chan struct{})
	var received []string
	var scanErr error
	go func() {
		defer close(eventsDone)
		for line := range events {
			received = append(received, string(line))
		}
		scanErr = <-errCh
	}()

	if err := p.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	<-eventsDone

	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d: %v", len(received), received)
	}
	if !strings.Contains(received[0], "hello") {
		t.Errorf("expected event to contain 'hello', got: %s", received[0])
	}
	if scanErr != nil {
		t.Errorf("events error: %v", scanErr)
	}
}

func TestStartWithContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}

	orig := piBinary
	piBinary = os.Args[0]
	defer func() { piBinary = orig }()

	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("FAKE_PI_MODE", "longrunning")

	ctx, cancel := context.WithCancel(context.Background())
	var stderrBuf bytes.Buffer
	p, err := Start(ctx, Options{}, "prompt", &stderrBuf)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	cancel()

	done := make(chan error, 1)
	go func() {
		done <- p.cmd.Wait()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for process to exit after context cancellation")
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
