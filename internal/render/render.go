// Package render formats pi RPC events as styled terminal output.
//
// A Renderer is a small state machine that tracks the current "section" of
// output (thinking, text, tool call, ...) so that section boundaries are
// always preceded by a newline, regardless of whether the previous chunk
// already ended on one.
package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// summaryMaxLen caps the rendered tool-result summary so that a chatty
// command doesn't flood the screen.
const summaryMaxLen = 200

// State describes which kind of content the renderer most recently emitted.
type State int

// Renderer state values.
const (
	StateIdle  State = iota // no output yet
	StateThink              // thinking tokens
	StateText               // assistant text
	StateTool               // tool call / tool execution lines
)

// Renderer streams styled events to an io.Writer while tracking enough
// state to insert section boundaries cleanly.
//
// A Renderer is not safe for concurrent use.
type Renderer struct {
	out         io.Writer
	state       State
	atLineStart bool
}

// New returns a Renderer that writes to out. The renderer assumes it starts
// at the beginning of a fresh line.
func New(out io.Writer) *Renderer {
	return &Renderer{out: out, atLineStart: true}
}

// Thinking emits a thinking delta in dim italic. Consecutive deltas stay on
// the same line; a transition from any other state inserts a newline first.
func (r *Renderer) Thinking(delta string) {
	if r.state != StateThink {
		r.ensureNewline()
	}
	r.state = StateThink
	if delta == "" {
		return
	}
	r.printf("%s%s%s", ansiDimItalic, delta, ansiReset)
	r.atLineStart = strings.HasSuffix(delta, "\n")
}

// Text emits an assistant text delta. A pending thinking section is
// flushed onto its own line first.
func (r *Renderer) Text(delta string) {
	if r.state == StateThink {
		r.ensureNewline()
	}
	r.state = StateText
	r.write(delta)
}

// ToolCall renders the "🔧 name args" line emitted when the model finishes
// describing a tool call.
func (r *Renderer) ToolCall(name string, args map[string]any) {
	r.ensureNewline()
	if len(args) > 0 {
		r.printf("%s🔧 %s %s%s\n", ansiBoldBlue, name, marshalJSON(args), ansiReset)
	} else {
		r.printf("%s🔧 %s%s\n", ansiBoldBlue, name, ansiReset)
	}
	r.atLineStart = true
	r.state = StateTool
}

// ToolExecStart renders the "⚡ name: cmd" header for a tool that has begun
// running. For bash-like tools the literal command is rendered; for others
// the full args object is dumped as JSON.
func (r *Renderer) ToolExecStart(name string, args map[string]any) {
	r.ensureNewline()
	if cmd, ok := args["command"].(string); ok {
		r.printf("%s⚡ %s: %s%s\n", ansiBoldYellow, name, cmd, ansiReset)
	} else {
		r.printf("%s⚡ %s %s%s\n", ansiBoldYellow, name, marshalJSON(args), ansiReset)
	}
	r.state = StateTool
	r.atLineStart = true
}

// ToolExecEnd renders the truncated summary line for a finished tool
// execution. isErr controls the ✓/✗ marker.
func (r *Renderer) ToolExecEnd(name string, isErr bool, summary string) {
	r.ensureNewline()
	summary = truncate(strings.TrimSpace(summary), summaryMaxLen)
	status := "✓"
	if isErr {
		status = "✗"
	}
	r.printf("%s  %s %s → %s%s\n", ansiDim, status, name, summary, ansiReset)
	r.atLineStart = true
	r.state = StateTool
}

// TurnStart inserts a newline if the previous section left the cursor mid-line.
func (r *Renderer) TurnStart() {
	if r.state != StateIdle {
		r.ensureNewline()
	}
}

// TurnEnd ensures the turn ends on a fresh line.
func (r *Renderer) TurnEnd() {
	r.ensureNewline()
}

// AgentEnd ensures the overall stream ends on a fresh line.
func (r *Renderer) AgentEnd() {
	r.ensureNewline()
}

// State returns the current renderer state. Intended for tests.
func (r *Renderer) State() State {
	return r.state
}

func (r *Renderer) ensureNewline() {
	if !r.atLineStart {
		r.write("\n")
	}
}

func (r *Renderer) write(s string) {
	if s == "" {
		return
	}
	_, _ = fmt.Fprint(r.out, s)
	r.atLineStart = strings.HasSuffix(s, "\n")
}

// printf is a write-only formatted helper that drops the io error. We only
// ever write to a tty / pipe; if that fails the OS will deliver SIGPIPE.
func (r *Renderer) printf(format string, args ...any) {
	_, _ = fmt.Fprintf(r.out, format, args...)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// marshalJSON renders v as compact JSON without Go's default HTML escaping,
// so &, <, > survive intact in tool argument blobs.
func marshalJSON(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return strings.TrimRight(buf.String(), "\n")
}
