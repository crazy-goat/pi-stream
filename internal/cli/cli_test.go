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
		Type:     "tool_execution_end",
		ToolName: "bash",
		IsError:  false,
		Result: &event.Result{Content: []event.ResultContent{
			{Text: "ok"},
		}},
	}, &bytes.Buffer{})
	if got := out.String(); !strings.Contains(got, "✓ bash → ok") {
		t.Errorf("expected ✓ summary, got %q", got)
	}
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

func TestHandleMessageToolCallEndRendersName(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	r := render.New(&out)
	handleMessage(r, &event.AssistantMessageEvent{
		Type:     "toolcall_end",
		ToolCall: &event.ToolCall{Name: "bash", Arguments: map[string]any{"command": "ls"}},
	})
	got := out.String()
	if !strings.Contains(got, "🔧 bash") {
		t.Errorf("expected 🔧 bash, got %q", got)
	}
}

func TestHandleMessageToolCallEndWithoutToolCallIsNoOp(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	r := render.New(&out)
	handleMessage(r, &event.AssistantMessageEvent{Type: "toolcall_end", ToolCall: nil})
	if out.String() != "" {
		t.Errorf("expected no output, got %q", out.String())
	}
}
