package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/crazy-goat/pi-stream/internal/event"
	"github.com/crazy-goat/pi-stream/internal/render"
)

func TestHandleEventAgentEndStopsAndSucceeds(t *testing.T) {
	t.Parallel()
	r := render.New(&bytes.Buffer{})
	done, code := handleEvent(r, event.Envelope{Type: "agent_end"}, &bytes.Buffer{})
	if !done || code != ExitOK {
		t.Errorf("agent_end: done=%v code=%d, want true,%d", done, code, ExitOK)
	}
}

func TestHandleEventErrorEnvelopeStopsWithErr(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	done, code := handleEvent(render.New(&bytes.Buffer{}), event.Envelope{Type: "error", Error: "boom"}, &stderr)
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
		event.Envelope{Type: "response", Success: &ok, Error: "denied"},
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
		Type: "message_update",
		AssistantMessageEvent: &event.AssistantMessageEvent{
			Type:  "text_delta",
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
		Type:       "tool_execution_start",
		ToolCallID: "call-1",
		ToolName:   "bash",
		Args:       map[string]any{"command": "x"},
	}, &bytes.Buffer{})
	_, _ = handleEvent(r, event.Envelope{
		Type:       "tool_execution_end",
		ToolCallID: "call-1",
		ToolName:   "bash",
		IsError:    false,
		Result: &event.Result{Content: []event.ResultContent{
			{Text: "ok"},
		}},
	}, &bytes.Buffer{})
	got := stripANSI(out.String())
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
	r.ToolExecStart("call-1", "bash", map[string]any{"command": "seq 1 2"})
	_, _ = handleEvent(r, event.Envelope{
		Type:       "tool_execution_update",
		ToolCallID: "call-1",
		ToolName:   "bash",
		PartialResult: &event.Result{Content: []event.ResultContent{
			{Text: "line 1\n"},
		}},
	}, &bytes.Buffer{})
	got := stripANSI(out.String())
	if !strings.Contains(got, "│ line 1") {
		t.Errorf("expected streamed line, got %q", got)
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && s[i] >= 0x20 && s[i] < 0x40 {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func TestHandleMessageThinkingDelta(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	r := render.New(&out)
	handleMessage(r, &event.AssistantMessageEvent{Type: "thinking_delta", Delta: "hmm"})
	if !strings.Contains(out.String(), "hmm") {
		t.Errorf("thinking_delta not rendered: %q", out.String())
	}
}

func TestHandleMessageToolCallEndIsNoOp(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	r := render.New(&out)
	handleMessage(r, &event.AssistantMessageEvent{
		Type:     "toolcall_end",
		ToolCall: &event.ToolCall{Name: "bash", Arguments: map[string]any{"command": "ls"}},
	})
	if out.String() != "" {
		t.Errorf("toolcall_end should not render — box comes from tool_execution_*; got %q", out.String())
	}
}
