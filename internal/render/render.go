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
	"time"

	"github.com/crazy-goat/pi-stream/internal/event"
)

// State describes which kind of content the renderer most recently emitted.
type State int

// Renderer state values.
const (
	StateIdle  State = iota // no output yet
	StateThink              // thinking tokens
	StateText               // assistant text
	StateTool               // tool call / tool execution lines
)

// toolBox tracks one in-flight tool execution. pi can interleave events for
// multiple tool calls (e.g. the model issues two bash calls in parallel), so
// we keep per-call state and only stream one box live at a time; others are
// queued and rendered as their turn comes up.
type toolBox struct {
	id            string
	name          string
	args          event.Args
	startTime     time.Time
	snapshot      string // latest cumulative stdout from updates / end
	bytesEmitted  int    // bytes of snapshot already turned into output
	lineBuf       strings.Builder
	lineCount     int
	ended         bool
	isErr         bool
	headerEmitted bool
}

// Renderer streams styled events to an io.Writer while tracking enough
// state to insert section boundaries cleanly.
//
// A Renderer is not safe for concurrent use.
type Renderer struct {
	out         io.Writer
	state       State
	atLineStart bool

	// activeID is the toolCallId currently streaming its box live; "" if
	// no box is open. tools holds every open call (active + queued + ended-
	// but-waiting). queue is the FIFO of toolCallIds waiting for the active
	// box to close.
	activeID string
	tools    map[string]*toolBox
	queue    []string
}

// New returns a Renderer that writes to out. The renderer assumes it starts
// at the beginning of a fresh line.
func New(out io.Writer) *Renderer {
	return &Renderer{out: out, atLineStart: true, tools: map[string]*toolBox{}}
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

// ToolExecStart registers a new tool call. If no other call is currently
// streaming, its box header is emitted immediately and live updates will
// flow into it. Otherwise the call is queued; its header (and accumulated
// output) is emitted once the active box closes.
func (r *Renderer) ToolExecStart(id, name string, args event.Args) {
	box := &toolBox{
		id:        id,
		name:      name,
		args:      args,
		startTime: time.Now(),
	}
	r.tools[id] = box
	if r.activeID == "" {
		r.activeID = id
		r.emitHeader(box)
	} else {
		r.queue = append(r.queue, id)
	}
}

// ToolExecUpdate records a cumulative stdout snapshot for the given call.
// If the call is the active one, newly-completed lines are streamed to the
// terminal right away. Snapshots for queued calls accumulate silently.
func (r *Renderer) ToolExecUpdate(id, snapshot string) {
	box, ok := r.tools[id]
	if !ok {
		return
	}
	box.snapshot = snapshot
	if r.activeID == id {
		r.consumeBox(box)
	}
}

// ToolExecEnd marks the call as finished. If it's the active call, the box
// is closed and the next queued call (if any) is promoted into the live
// slot. If the call was queued, it stays queued (now marked ended) and is
// rendered as a static box when its turn arrives.
func (r *Renderer) ToolExecEnd(id string, isErr bool, fullText string) {
	box, ok := r.tools[id]
	if !ok {
		return
	}
	if fullText != "" {
		box.snapshot = fullText
	}
	box.ended = true
	box.isErr = isErr

	if r.activeID == id {
		r.consumeBox(box)
		r.closeBox(box)
		delete(r.tools, id)
		r.activeID = ""
		r.promoteNext()
	}
}

// promoteNext pulls the next queued call into the live slot, emits its
// header, replays any output it accumulated while queued, and — if it has
// already ended — closes it immediately. Loops so that a chain of already-
// finished calls all flush out at once.
func (r *Renderer) promoteNext() {
	for len(r.queue) > 0 {
		next := r.queue[0]
		r.queue = r.queue[1:]
		box, ok := r.tools[next]
		if !ok {
			continue
		}
		r.activeID = next
		r.emitHeader(box)
		r.consumeBox(box)
		if box.ended {
			r.closeBox(box)
			delete(r.tools, next)
			r.activeID = ""
			continue
		}
		return
	}
}

func (r *Renderer) emitHeader(b *toolBox) {
	r.ensureNewline()
	if cmd, ok := b.args.Command(); ok {
		r.printf("%s┌─ ⚡ %s ─%s %s\n", ansiBoldCyan, b.name, ansiReset, cmd)
	} else if len(b.args) > 0 {
		r.printf("%s┌─ ⚡ %s%s %s\n", ansiBoldCyan, b.name, ansiReset, marshalJSON(b.args))
	} else {
		r.printf("%s┌─ ⚡ %s%s\n", ansiBoldCyan, b.name, ansiReset)
	}
	b.headerEmitted = true
	r.state = StateTool
	r.atLineStart = true
}

func (r *Renderer) consumeBox(b *toolBox) {
	if len(b.snapshot) <= b.bytesEmitted {
		return
	}
	delta := b.snapshot[b.bytesEmitted:]
	b.bytesEmitted = len(b.snapshot)
	b.lineBuf.WriteString(delta)
	s := b.lineBuf.String()
	b.lineBuf.Reset()
	for {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			break
		}
		r.emitToolLine(s[:i])
		b.lineCount++
		s = s[i+1:]
	}
	b.lineBuf.WriteString(s)
}

func (r *Renderer) closeBox(b *toolBox) {
	if rem := b.lineBuf.String(); rem != "" {
		r.emitToolLine(rem)
		b.lineCount++
		b.lineBuf.Reset()
	}
	status, color := "✓", ansiBoldGreen
	if b.isErr {
		status, color = "✗", ansiBoldRed
	}
	info := formatDuration(time.Since(b.startTime))
	if b.lineCount > 0 {
		info = fmt.Sprintf("%s, %d lines", info, b.lineCount)
	}
	r.printf("%s└─%s %s%s %s%s %s(%s)%s\n",
		ansiBoldCyan, ansiReset,
		color, status, b.name, ansiReset,
		ansiDim, info, ansiReset)
	r.state = StateTool
	r.atLineStart = true
}

func (r *Renderer) emitToolLine(line string) {
	r.printf("%s│%s %s\n", ansiDimCyan, ansiReset, line)
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

// marshalJSON renders v as compact JSON without Go's default HTML escaping,
// so &, <, > survive intact in tool argument blobs.
// If v cannot be encoded as JSON (e.g. channels, functions), it falls back
// to fmt.Sprintf so the caller always gets a meaningful representation.
func marshalJSON(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return fmt.Sprintf("%v", v)
	}
	return strings.TrimRight(buf.String(), "\n")
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
