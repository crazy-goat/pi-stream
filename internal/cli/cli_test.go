package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/crazy-goat/pi-stream/internal/event"
	"github.com/crazy-goat/pi-stream/internal/render"
	"github.com/crazy-goat/pi-stream/internal/testutil"
)

// testError is a simple error implementation for injecting scanner errors
// into fakeEventSource.
type testError struct{ msg string }

func (e testError) Error() string { return e.msg }

// fakeEventSource implements eventSource for testing streamEvents.
type fakeEventSource struct {
	ch     chan []byte
	errCh  chan error
	killed bool
}

func newFakeEventSource() *fakeEventSource {
	return &fakeEventSource{
		ch:    make(chan []byte, 32),
		errCh: make(chan error, 1),
	}
}

func (f *fakeEventSource) Events() (<-chan []byte, <-chan error) {
	return f.ch, f.errCh
}

func (f *fakeEventSource) Kill() error {
	f.killed = true
	return nil
}

func TestStreamEventsReturnsOKOnChannelClose(t *testing.T) {
	t.Parallel()
	fs := newFakeEventSource()
	close(fs.ch)

	code := streamEvents(context.Background(), fs, &bytes.Buffer{}, &bytes.Buffer{})
	if code != ExitOK {
		t.Errorf("channel close: code=%d, want %d", code, ExitOK)
	}
}

func TestStreamEventsReturnsInterruptOnContextCancel(t *testing.T) {
	t.Parallel()
	fs := newFakeEventSource()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	code := streamEvents(ctx, fs, &bytes.Buffer{}, &bytes.Buffer{})
	if code != ExitInterrupt {
		t.Errorf("ctx cancel: code=%d, want %d", code, ExitInterrupt)
	}
}

func TestStreamEventsSkipsMalformedJSON(t *testing.T) {
	t.Parallel()
	fs := newFakeEventSource()
	fs.ch <- []byte("not json")
	fs.ch <- []byte(`{"type":"agent_end"}`)
	close(fs.ch)

	code := streamEvents(context.Background(), fs, &bytes.Buffer{}, &bytes.Buffer{})
	if code != ExitOK {
		t.Errorf("malformed JSON skipped: code=%d, want %d", code, ExitOK)
	}
}

func TestStreamEventsRendersTextDelta(t *testing.T) {
	t.Parallel()
	fs := newFakeEventSource()
	fs.ch <- []byte(`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"hello world"}}`)
	close(fs.ch)

	var stdout bytes.Buffer
	code := streamEvents(context.Background(), fs, &stdout, &bytes.Buffer{})
	if code != ExitOK {
		t.Errorf("text delta: code=%d, want %d", code, ExitOK)
	}
	if !strings.Contains(stdout.String(), "hello world") {
		t.Errorf("stdout missing text: %q", stdout.String())
	}
}

func TestStreamEventsStopsOnErrorEnvelope(t *testing.T) {
	t.Parallel()
	fs := newFakeEventSource()
	fs.ch <- []byte(`{"type":"error","error":"something went wrong"}`)
	close(fs.ch)

	var stderr bytes.Buffer
	code := streamEvents(context.Background(), fs, &bytes.Buffer{}, &stderr)
	if code != ExitError {
		t.Errorf("error envelope: code=%d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr.String(), "something went wrong") {
		t.Errorf("stderr missing error: %q", stderr.String())
	}
}

func TestStreamEventsStopsOnResponseFailure(t *testing.T) {
	t.Parallel()
	fs := newFakeEventSource()
	fs.ch <- []byte(`{"type":"response","success":false,"error":"permission denied"}`)
	close(fs.ch)

	var stderr bytes.Buffer
	code := streamEvents(context.Background(), fs, &bytes.Buffer{}, &stderr)
	if code != ExitError {
		t.Errorf("response failure: code=%d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr.String(), "permission denied") {
		t.Errorf("stderr missing error: %q", stderr.String())
	}
}

func TestStreamEventsStopsOnScannerError(t *testing.T) {
	t.Parallel()
	fs := newFakeEventSource()
	fs.errCh <- testError{"scanner buffer overflow"}

	var stderr bytes.Buffer
	code := streamEvents(context.Background(), fs, &bytes.Buffer{}, &stderr)
	if code != ExitError {
		t.Errorf("scanner error: code=%d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr.String(), "scanner buffer overflow") {
		t.Errorf("stderr missing scanner error: %q", stderr.String())
	}
}

func TestStreamEventsFullFlow(t *testing.T) {
	t.Parallel()
	fs := newFakeEventSource()
	fs.ch <- []byte(`{"type":"message_update","assistantMessageEvent":{"type":"thinking_delta","delta":"thinking..."}}`)
	fs.ch <- []byte(`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"hello"}}`)
	fs.ch <- []byte(`{"type":"tool_execution_start","toolCallId":"call-1","toolName":"bash","args":{"command":"echo hi"}}`)
	fs.ch <- []byte(`{"type":"tool_execution_update","toolCallId":"call-1","partialResult":{"content":[{"text":"hi\n"}]}}`)
	fs.ch <- []byte(`{"type":"tool_execution_end","toolCallId":"call-1","result":{"content":[{"text":"done"}]}}`)
	fs.ch <- []byte(`{"type":"agent_end"}`)
	close(fs.ch)

	var stdout bytes.Buffer
	code := streamEvents(context.Background(), fs, &stdout, &bytes.Buffer{})
	if code != ExitOK {
		t.Errorf("full flow: code=%d, want %d", code, ExitOK)
	}
	got := testutil.StripANSI(stdout.String())
	if !strings.Contains(got, "hello") {
		t.Errorf("stdout missing text: %q", got)
	}
	if !strings.Contains(got, "thinking...") {
		t.Errorf("stdout missing thinking: %q", got)
	}
	if !strings.Contains(got, "✓ bash") {
		t.Errorf("stdout missing tool footer: %q", got)
	}
}

func TestStreamEventsNilScannerErrorReturnsExitOK(t *testing.T) {
	t.Parallel()
	fs := newFakeEventSource()
	fs.errCh <- nil

	code := streamEvents(context.Background(), fs, &bytes.Buffer{}, &bytes.Buffer{})
	if code != ExitOK {
		t.Errorf("nil scanner error: code=%d, want %d", code, ExitOK)
	}
}

func TestStreamEventsInterruptKillsProcess(t *testing.T) {
	t.Parallel()
	fs := newFakeEventSource()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = streamEvents(ctx, fs, &bytes.Buffer{}, &bytes.Buffer{})
	if !fs.killed {
		t.Error("Kill() was not called on context cancel")
	}
}

func TestHandleEventAgentEndStopsAndSucceeds(t *testing.T) {
	t.Parallel()
	r := render.New(&bytes.Buffer{})
	done, code := handleEvent(r, event.Envelope{Type: event.TypeAgentEnd}, &bytes.Buffer{})
	if !done || code != ExitOK {
		t.Errorf("agent_end: done=%v code=%d, want true,%d", done, code, ExitOK)
	}
}

func TestHandleEventErrorEnvelopeStopsWithErr(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	done, code := handleEvent(render.New(&bytes.Buffer{}), event.Envelope{Type: event.TypeError, Error: "boom"}, &stderr)
	if !done || code != ExitError {
		t.Errorf("error: done=%v code=%d, want true,%d", done, code, ExitError)
	}
	if !strings.Contains(stderr.String(), "boom") {
		t.Errorf("stderr missing message: %q", stderr.String())
	}
}

func TestHandleEventResponseFailureStopsWithErr(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	ok := false
	done, code := handleEvent(
		render.New(&bytes.Buffer{}),
		event.Envelope{Type: event.TypeResponse, Success: &ok, Error: "denied"},
		&stderr,
	)
	if !done || code != ExitError {
		t.Errorf("response fail: done=%v code=%d", done, code)
	}
	if !strings.Contains(stderr.String(), "denied") {
		t.Errorf("stderr missing message: %q", stderr.String())
	}
}

func TestHandleEventTextDeltaRenders(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	r := render.New(&out)
	done, code := handleEvent(r, event.Envelope{
		Type: event.TypeMessageUpdate,
		AssistantMessageEvent: &event.AssistantMessageEvent{
			Type:  event.MsgTypeTextDelta,
			Delta: "hello",
		},
	}, &bytes.Buffer{})
	if done || code != ExitOK {
		t.Errorf("text_delta should not stop stream")
	}
	if got := out.String(); !strings.Contains(got, "hello") {
		t.Errorf("expected text rendered, got %q", got)
	}
}

func TestHandleEventToolExecEndRenders(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	r := render.New(&out)
	_, _ = handleEvent(r, event.Envelope{
		Type:       event.TypeToolExecStart,
		ToolCallID: "call-1",
		ToolName:   "bash",
		Args:       event.Args{"command": "x"},
	}, &bytes.Buffer{})
	_, _ = handleEvent(r, event.Envelope{
		Type:       event.TypeToolExecEnd,
		ToolCallID: "call-1",
		ToolName:   "bash",
		IsError:    false,
		Result: &event.Result{Content: []event.ResultContent{
			{Text: "ok"},
		}},
	}, &bytes.Buffer{})
	got := testutil.StripANSI(out.String())
	if !strings.Contains(got, "│ ok") {
		t.Errorf("expected output line in box, got %q", got)
	}
	if !strings.Contains(got, "✓ bash") {
		t.Errorf("expected ✓ bash footer, got %q", got)
	}
}

func TestHandleEventToolExecUpdateStreamsLines(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	r := render.New(&out)
	r.ToolExecStart("call-1", "bash", event.Args{"command": "seq 1 2"})
	_, _ = handleEvent(r, event.Envelope{
		Type:       event.TypeToolExecUpdate,
		ToolCallID: "call-1",
		ToolName:   "bash",
		PartialResult: &event.Result{Content: []event.ResultContent{
			{Text: "line 1\n"},
		}},
	}, &bytes.Buffer{})
	got := testutil.StripANSI(out.String())
	if !strings.Contains(got, "│ line 1") {
		t.Errorf("expected streamed line, got %q", got)
	}
}

func TestHandleMessageThinkingDelta(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	r := render.New(&out)
	handleMessage(r, &event.AssistantMessageEvent{Type: event.MsgTypeThinkingDelta, Delta: "hmm"})
	if !strings.Contains(out.String(), "hmm") {
		t.Errorf("thinking_delta not rendered: %q", out.String())
	}
}

func TestRunInvalidThinkingFlag(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"--thinking", "banana", "prompt"}, &bytes.Buffer{}, &stderr)
	if code != ExitUsage {
		t.Errorf("invalid thinking: code=%d, want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr.String(), "invalid --thinking") {
		t.Errorf("stderr missing invalid thinking error: %q", stderr.String())
	}
}

func TestRunValidThinkingFlags(t *testing.T) {
	validValues := []string{"off", "minimal", "low", "medium", "high", "xhigh"}
	for _, v := range validValues {
		v := v
		t.Run(v, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			code := Run(ctx, []string{"--thinking", v, "prompt"}, &bytes.Buffer{}, &bytes.Buffer{})
			if code == ExitUsage {
				t.Errorf("valid thinking %q returned ExitUsage", v)
			}
		})
	}
}

func TestRunDefaultThinkingFlag(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	code := Run(ctx, []string{"prompt"}, &bytes.Buffer{}, &bytes.Buffer{})
	if code == ExitUsage {
		t.Errorf("default thinking \"high\" should pass validation, got ExitUsage")
	}
}

func TestHandleEventToolExecUpdateNilPartialResult(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	r := render.New(&out)
	r.ToolExecStart("call-1", "bash", event.Args{"command": "x"})
	done, code := handleEvent(r, event.Envelope{
		Type:          event.TypeToolExecUpdate,
		ToolCallID:    "call-1",
		PartialResult: nil,
	}, &bytes.Buffer{})
	if done || code != ExitOK {
		t.Errorf("nil update should not stop stream: done=%v code=%d", done, code)
	}
	// Verify the box is still open (no footer yet) after nil update.
	_, _ = handleEvent(r, event.Envelope{
		Type:       event.TypeToolExecEnd,
		ToolCallID: "call-1",
		IsError:    false,
		Result:     nil,
	}, &bytes.Buffer{})
	got := testutil.StripANSI(out.String())
	if !strings.Contains(got, "✓ bash") {
		t.Errorf("expected closed box after nil flow, got %q", got)
	}
}

func TestHandleEventToolExecEndNilResult(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		isError   bool
		wantGlyph string
	}{
		{name: "success", isError: false, wantGlyph: "✓"},
		{name: "error", isError: true, wantGlyph: "✗"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			r := render.New(&out)
			done, code := handleEvent(r, event.Envelope{
				Type:       event.TypeToolExecStart,
				ToolCallID: "call-1",
				ToolName:   "bash",
				Args:       event.Args{"command": "x"},
			}, &bytes.Buffer{})
			if done || code != ExitOK {
				t.Fatalf("start should not stop stream: done=%v code=%d", done, code)
			}
			done, code = handleEvent(r, event.Envelope{
				Type:       event.TypeToolExecEnd,
				ToolCallID: "call-1",
				IsError:    tc.isError,
				Result:     nil,
			}, &bytes.Buffer{})
			if done || code != ExitOK {
				t.Errorf("nil result end should not stop stream: done=%v code=%d", done, code)
			}
			got := testutil.StripANSI(out.String())
			if !strings.Contains(got, tc.wantGlyph+" bash") {
				t.Errorf("expected closed box with %q glyph, got %q", tc.wantGlyph, got)
			}
		})
	}
}

func TestHandleMessageToolCallEndIsNoOp(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	r := render.New(&out)
	handleMessage(r, &event.AssistantMessageEvent{
		Type:     "toolcall_end",
		ToolCall: &event.ToolCall{Name: "bash", Arguments: event.Args{"command": "ls"}},
	})
	if out.String() != "" {
		t.Errorf("toolcall_end should not render — box comes from tool_execution_*; got %q", out.String())
	}
}

func TestRunVersionFlag(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"--version"}, &stdout, &stderr)
	if code != ExitOK {
		t.Errorf("--version: code=%d, want %d", code, ExitOK)
	}
	if stdout.String() != Version+"\n" {
		t.Errorf("--version: stdout=%q, want %q", stdout.String(), Version+"\n")
	}
}

func TestRunHelpFlag(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"--help"}, &bytes.Buffer{}, &stderr)
	if code != ExitOK {
		t.Errorf("--help: code=%d, want %d", code, ExitOK)
	}
	if !strings.Contains(stderr.String(), "usage: pi-stream") {
		t.Errorf("--help: stderr should contain usage, got %q", stderr.String())
	}
}

func TestRunNoArgs(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{}, &bytes.Buffer{}, &stderr)
	if code != ExitUsage {
		t.Errorf("no args: code=%d, want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr.String(), "usage: pi-stream") {
		t.Errorf("no args: stderr should contain usage, got %q", stderr.String())
	}
}
