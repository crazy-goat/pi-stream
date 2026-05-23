package render

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/crazy-goat/pi-stream/internal/event"
	"github.com/crazy-goat/pi-stream/internal/testutil"
)

func TestTextStreamsConcatenated(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.Text("hello")
	r.Text(" world")
	if got, want := buf.String(), "hello world"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestThinkingDimItalic(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.Thinking("hmm")
	got := buf.String()
	if !strings.Contains(got, ansiDimItalic) || !strings.Contains(got, ansiReset) {
		t.Errorf("missing ANSI codes: %q", got)
	}
	if !strings.Contains(got, "hmm") {
		t.Errorf("missing payload: %q", got)
	}
}

func TestThinkingToTextInsertsNewline(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.Thinking("hmm")
	r.Text("answer")
	out := buf.String()
	idxThink := strings.Index(out, "hmm")
	idxText := strings.Index(out, "answer")
	if idxThink < 0 || idxText < 0 || idxThink > idxText {
		t.Fatalf("unexpected ordering in %q", out)
	}
	between := out[idxThink+len("hmm") : idxText]
	if !strings.Contains(between, "\n") {
		t.Errorf("expected newline between sections, got %q", between)
	}
}

func TestToolExecStartBash(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.ToolExecStart("t1", "bash", event.Args{"command": "echo hi"})
	got := buf.String()
	if !strings.Contains(got, "┌─ ⚡ bash") {
		t.Errorf("expected top-of-box header, got %q", got)
	}
	if !strings.Contains(got, "echo hi") {
		t.Errorf("expected literal command, got %q", got)
	}
	if strings.Contains(got, `"command"`) {
		t.Errorf("bash should render literal command, not JSON: %q", got)
	}
}

func TestToolExecStartNonBash(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.ToolExecStart("t1", "http", event.Args{"url": "https://example.com"})
	got := buf.String()
	if !strings.Contains(got, "┌─ ⚡ http ─") {
		t.Errorf("expected header with ─ separator, got %q", got)
	}
	if !strings.Contains(got, `"url":"https://example.com"`) {
		t.Errorf("expected JSON args, got %q", got)
	}
}

func TestToolExecStartNoArgs(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.ToolExecStart("t1", "ping", nil)
	got := buf.String()
	if !strings.Contains(got, "┌─ ⚡ ping") {
		t.Errorf("expected ┌─ ⚡ ping, got %q", got)
	}
	if strings.Contains(got, "{}") {
		t.Errorf("should not render empty args object: %q", got)
	}
}

func TestToolExecUpdateStreamsCompleteLinesOnly(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.ToolExecStart("t1", "bash", event.Args{"command": "x"})

	r.ToolExecUpdate("t1", "line 1\nline ")
	got := testutil.StripANSI(buf.String())
	if !strings.Contains(got, "│ line 1\n") {
		t.Errorf("expected complete line emitted, got %q", got)
	}
	if strings.Contains(got, "│ line \n") || strings.Contains(got, "│ line 2") {
		t.Errorf("partial line should be buffered, got %q", got)
	}

	r.ToolExecUpdate("t1", "line 1\nline 2\n")
	got = testutil.StripANSI(buf.String())
	if !strings.Contains(got, "│ line 2\n") {
		t.Errorf("expected second line emitted after newline arrived, got %q", got)
	}
}

func TestToolExecUpdateIgnoresShrinkingSnapshot(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.ToolExecStart("t1", "bash", event.Args{"command": "x"})
	r.ToolExecUpdate("t1", "a\nb\n")
	beforeLen := buf.Len()
	r.ToolExecUpdate("t1", "a\n") // shorter — pi shouldn't do this, but be safe
	if buf.Len() != beforeLen {
		t.Errorf("shorter snapshot should be ignored, but buffer grew by %d", buf.Len()-beforeLen)
	}
}

func TestToolExecEndFlushesBufferedPartialLine(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.ToolExecStart("t1", "bash", event.Args{"command": "x"})
	r.ToolExecUpdate("t1", "trailing without newline")
	r.ToolExecEnd("t1", false, "trailing without newline")
	got := testutil.StripANSI(buf.String())
	if !strings.Contains(got, "│ trailing without newline\n") {
		t.Errorf("expected buffered partial line flushed on end, got %q", got)
	}
}

func TestToolExecEndConsumesUnreportedTail(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.ToolExecStart("t1", "bash", event.Args{"command": "x"})
	// No update arrived — fullText carries everything.
	r.ToolExecEnd("t1", false, "line A\nline B\n")
	got := testutil.StripANSI(buf.String())
	if !strings.Contains(got, "│ line A\n") || !strings.Contains(got, "│ line B\n") {
		t.Errorf("expected both lines emitted from end-only fullText, got %q", got)
	}
}

func TestToolExecEndSuccessClosesBoxWithGreenCheck(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.ToolExecStart("t1", "bash", event.Args{"command": "x"})
	r.ToolExecEnd("t1", false, "")
	got := buf.String()
	if !strings.Contains(got, "└─") || !strings.Contains(got, "✓ bash") {
		t.Errorf("expected closing line with ✓, got %q", got)
	}
	if !strings.Contains(got, ansiBoldGreen) {
		t.Errorf("expected green status, got %q", got)
	}
}

func TestToolExecEndErrorClosesBoxWithRedCross(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.ToolExecStart("t1", "bash", event.Args{"command": "x"})
	r.ToolExecEnd("t1", true, "")
	got := buf.String()
	if !strings.Contains(got, "✗ bash") {
		t.Errorf("expected ✗ status, got %q", got)
	}
	if !strings.Contains(got, ansiBoldRed) {
		t.Errorf("expected red status, got %q", got)
	}
}

func TestToolExecEndIncludesLineCountWhenNonZero(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.ToolExecStart("t1", "bash", event.Args{"command": "x"})
	r.ToolExecUpdate("t1", "a\nb\nc\n")
	r.ToolExecEnd("t1", false, "a\nb\nc\n")
	got := buf.String()
	if !strings.Contains(got, "3 lines") {
		t.Errorf("expected line count in footer, got %q", got)
	}
}

func TestToolExecFullCycleNoOutput(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.ToolExecStart("t1", "edit", event.Args{"path": "/x"})
	r.ToolExecEnd("t1", false, "")
	got := buf.String()
	if !strings.Contains(got, "┌─ ⚡ edit") || !strings.Contains(got, "└─") {
		t.Errorf("expected both box corners even with no output, got %q", got)
	}
	if strings.Contains(got, "│ ") {
		t.Errorf("no gutter lines expected when output is empty, got %q", got)
	}
}

func TestToolExecFullOutputNotTruncated(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.ToolExecStart("t1", "bash", event.Args{"command": "x"})
	long := strings.Repeat("x", 500) + "\n"
	r.ToolExecUpdate("t1", long)
	r.ToolExecEnd("t1", false, long)
	got := buf.String()
	if strings.Contains(got, "...") {
		t.Errorf("output must not be truncated, got %q", got)
	}
	if !strings.Contains(got, strings.Repeat("x", 500)) {
		t.Error("expected full 500-char line preserved")
	}
}

func TestMarshalJSONNoHTMLEscape(t *testing.T) {
	t.Parallel()
	got := marshalJSON(event.Args{"cmd": "a && b <c>"})
	for _, want := range []string{"&&", "<c>"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected raw %q in %q", want, got)
		}
	}
}

func TestMarshalJSONNil(t *testing.T) {
	t.Parallel()
	got := marshalJSON(nil)
	if got != "null" {
		t.Errorf("marshalJSON(nil) = %q, want %q", got, "null")
	}
}

func TestMarshalJSONUnencodableValue(t *testing.T) {
	t.Parallel()
	got := marshalJSON(event.Args{"ch": make(chan int)})
	if got == "" || strings.Contains(got, "null") {
		t.Errorf("marshalJSON with unencodable value should return fallback, got %q", got)
	}
	if !strings.Contains(got, "ch:") {
		t.Errorf("marshalJSON fallback should contain the key name, got %q", got)
	}
}

func TestEnsureNewlineMidLine(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.Text("hello") // mid-line
	r.ToolExecStart("t1", "ping", nil)
	got := buf.String()
	if !strings.HasPrefix(got, "hello\n") {
		t.Errorf("expected newline injected before tool box, got %q", got)
	}
}

func TestTextAfterToolExecEndNoExtraNewline(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.ToolExecStart("t1", "ping", nil)
	r.ToolExecEnd("t1", false, "")
	r.Text("done")
	got := buf.String()
	if strings.Contains(got, "\n\n") {
		t.Errorf("unexpected blank line: %q", got)
	}
}

func TestTurnAndAgentEndOnFreshLine(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.Text("mid")
	r.TurnEnd()
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Errorf("TurnEnd should end on newline: %q", buf.String())
	}
	r.Text("more")
	r.AgentEnd()
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Errorf("AgentEnd should end on newline: %q", buf.String())
	}
}

func TestTurnStartIdleNoOutput(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.TurnStart()
	if out := buf.String(); out != "" {
		t.Errorf("TurnStart() on idle renderer should produce no output, got %q", out)
	}
}

func TestTurnStartAfterTextInsertsNewline(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.Text("hello")
	r.TurnStart()
	if got, want := buf.String(), "hello\n"; got != want {
		t.Errorf("TurnStart after text = %q, want %q", got, want)
	}
}

func TestTurnStartAfterThinkingInsertsNewline(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.Thinking("hmm")
	r.TurnStart()
	got := buf.String()
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("TurnStart after Thinking should insert newline, got %q", got)
	}
}

func TestTurnStartAfterNewlineNoDoubleNewline(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.Text("hello\n")
	r.TurnStart()
	got := buf.String()
	if strings.HasSuffix(got, "\n\n") {
		t.Errorf("TurnStart after text ending with newline should not double it, got %q", got)
	}
}

func TestParallelToolsRenderSequentially(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	// Two parallel tools: starts arrive before the first end.
	r.ToolExecStart("a", "bash", event.Args{"command": "echo hello"})
	r.ToolExecStart("b", "bash", event.Args{"command": "echo world"})
	r.ToolExecUpdate("a", "hello\n")
	r.ToolExecUpdate("b", "world\n")
	r.ToolExecEnd("a", false, "hello\n")
	r.ToolExecEnd("b", false, "world\n")

	got := testutil.StripANSI(buf.String())
	// First box: header A, hello, footer A. Second box: header B, world, footer B.
	idxHeaderA := strings.Index(got, "echo hello")
	idxHello := strings.Index(got, "│ hello")
	idxFooterA := strings.Index(got, "✓ bash")
	idxHeaderB := strings.Index(got, "echo world")
	idxWorld := strings.Index(got, "│ world")
	idxFooterB := strings.LastIndex(got, "✓ bash")

	if idxHeaderA < 0 || idxHello < 0 || idxFooterA < 0 || idxHeaderB < 0 || idxWorld < 0 {
		t.Fatalf("missing parts in output:\n%s", got)
	}
	if !(idxHeaderA < idxHello && idxHello < idxFooterA && idxFooterA < idxHeaderB && idxHeaderB < idxWorld && idxWorld < idxFooterB) {
		t.Errorf("expected sequential ordering A-header, A-line, A-footer, B-header, B-line, B-footer; got:\n%s", got)
	}
}

func TestParallelToolEndsBeforeFirstFinishes(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	// B starts and ends while A is still active. B's box must be flushed
	// after A's, with its full content.
	r.ToolExecStart("a", "bash", event.Args{"command": "long-running"})
	r.ToolExecStart("b", "bash", event.Args{"command": "quick"})
	r.ToolExecEnd("b", false, "quick output\n")
	// No B output should appear yet — A is still active.
	mid := testutil.StripANSI(buf.String())
	if strings.Contains(mid, "│ quick output") {
		t.Errorf("B output leaked while A still active:\n%s", mid)
	}
	r.ToolExecEnd("a", false, "")
	got := testutil.StripANSI(buf.String())
	if !strings.Contains(got, "│ quick output") {
		t.Errorf("B output should appear after A closes:\n%s", got)
	}
}

func TestQueueNilWhenDrained(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.ToolExecStart("a", "bash", event.Args{"command": "echo a"})
	r.ToolExecStart("b", "bash", event.Args{"command": "echo b"})
	r.ToolExecStart("c", "bash", event.Args{"command": "echo c"})
	// End A — promotes B (ends), promotes C (ends), queue drains to nil.
	r.ToolExecEnd("a", false, "a\n")
	r.ToolExecEnd("b", false, "b\n")
	r.ToolExecEnd("c", false, "c\n")
	if r.queue != nil {
		t.Errorf("queue should be nil when fully drained, got %v", r.queue)
	}
}

func TestQueueNilWhenSingleToolEnds(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.ToolExecStart("a", "bash", event.Args{"command": "echo a"})
	r.ToolExecEnd("a", false, "a\n")
	if r.queue != nil {
		t.Errorf("queue should be nil after single tool ends, got %v", r.queue)
	}
}

func TestPromoteNextOnNilQueueDoesNotPanic(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.ToolExecStart("a", "bash", event.Args{"command": "echo a"})
	r.ToolExecEnd("a", false, "a\n")
	// promoteNext already ran inside ToolExecEnd; calling again on nil
	// queue must not panic (regression guard).
	defer func() {
		if p := recover(); p != nil {
			t.Errorf("promoteNext panicked on nil queue: %v", p)
		}
	}()
	r.promoteNext()
}

func TestQueueNotNilWhenPendingItemsExist(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.ToolExecStart("a", "bash", event.Args{"command": "echo a"})
	r.ToolExecStart("b", "bash", event.Args{"command": "echo b"})
	r.ToolExecStart("c", "bash", event.Args{"command": "echo c"})
	// End A — promotes B (active). B is active, C is still queued.
	r.ToolExecEnd("a", false, "a\n")
	if r.queue == nil {
		t.Errorf("queue should not be nil when items remain pending")
	}
	if len(r.queue) != 1 {
		t.Errorf("expected 1 pending item in queue, got %d", len(r.queue))
	}
}

func TestQueueSliceDoesNotRetainOldEntries(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	n := 100
	for i := range n {
		id := fmt.Sprintf("tool_%d", i)
		r.ToolExecStart(id, "bash", event.Args{"command": fmt.Sprintf("echo %d", i)})
	}
	for i := range n {
		id := fmt.Sprintf("tool_%d", i)
		r.ToolExecEnd(id, false, fmt.Sprintf("output %d\n", i))
	}
	if r.queue != nil {
		t.Errorf("queue should be nil when fully drained, got len=%d cap=%d", len(r.queue), cap(r.queue))
	}
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ms   int
		want string
	}{
		{0, "0ms"},
		{42, "42ms"},
		{999, "999ms"},
		{1000, "1.0s"},
		{1500, "1.5s"},
		{-500, "0ms"},
		{3661000, "3661.0s"},
	}
	for _, c := range cases {
		got := formatDuration(time.Duration(c.ms) * time.Millisecond)
		if got != c.want {
			t.Errorf("formatDuration(%dms) = %q, want %q", c.ms, got, c.want)
		}
	}
}

func TestFormatDurationSubMillisecond(t *testing.T) {
	t.Parallel()
	got := formatDuration(500 * time.Microsecond)
	if got != "0ms" {
		t.Errorf("formatDuration(500µs) = %q, want %q", got, "0ms")
	}
}

func TestAgentEndCleansUpOrphanedTools(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.ToolExecStart("t1", "bash", event.Args{"command": "x"})
	r.ToolExecStart("t2", "bash", event.Args{"command": "y"})
	r.ToolExecStart("t3", "bash", event.Args{"command": "z"})
	if len(r.tools) == 0 {
		t.Fatal("expected tools to be populated before AgentEnd")
	}
	r.AgentEnd()
	if len(r.tools) != 0 {
		t.Errorf("AgentEnd should clear tools map, got %d entries", len(r.tools))
	}
	if r.queue != nil {
		t.Errorf("AgentEnd should reset queue to nil, got %v", r.queue)
	}
	if r.activeID != "" {
		t.Errorf("AgentEnd should clear activeID, got %q", r.activeID)
	}
}

func TestAgentEndWithNoToolsIsNoOp(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.Text("hello")
	r.AgentEnd()
	if len(r.tools) != 0 {
		t.Errorf("tools should be empty, got %d", len(r.tools))
	}
	if r.queue != nil {
		t.Errorf("queue should be nil, got %v", r.queue)
	}
	if r.activeID != "" {
		t.Errorf("activeID should be empty, got %q", r.activeID)
	}
}

func TestResetReturnsToInitialState(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.ToolExecStart("t1", "bash", event.Args{"command": "x"})
	r.ToolExecStart("t2", "bash", event.Args{"command": "y"})
	r.ToolExecUpdate("t1", "line 1\n")
	r.Text("some text")
	r.Thinking("hmm")
	r.Reset()
	if r.State() != StateIdle {
		t.Errorf("Reset should set state to StateIdle, got %v", r.State())
	}
	if len(r.tools) != 0 {
		t.Errorf("Reset should clear tools map, got %d entries", len(r.tools))
	}
	if r.queue != nil {
		t.Errorf("Reset should set queue to nil, got %v", r.queue)
	}
	if !r.atLineStart {
		t.Errorf("Reset should set atLineStart to true")
	}
	if r.activeID != "" {
		t.Errorf("Reset should clear activeID, got %q", r.activeID)
	}
	// After Reset, the renderer should accept new content cleanly
	r.Text("fresh start")
	if got := buf.String(); !strings.Contains(got, "fresh start") {
		t.Errorf("Reset renderer should produce output, got %q", got)
	}
}

func BenchmarkConsumeBoxSmall(b *testing.B) {
	b.ReportAllocs()
	r := New(io.Discard)
	r.ToolExecStart("t1", "bash", event.Args{"command": "echo hi"})
	box := r.tools["t1"]
	snapshot := "line 1\nline 2\nline 3\n"
	b.ResetTimer()
	for range b.N {
		box.snapshot = snapshot
		box.bytesEmitted = 0
		box.lineBuf.Reset()
		box.lineCount = 0
		r.consumeBox(box)
	}
}

func BenchmarkConsumeBoxLarge(b *testing.B) {
	b.ReportAllocs()
	r := New(io.Discard)
	r.ToolExecStart("t1", "bash", event.Args{"command": "echo hi"})
	box := r.tools["t1"]
	var sb strings.Builder
	for i := range 1000 {
		fmt.Fprintf(&sb, "line %d: %s\n", i, strings.Repeat("data ", 20))
	}
	snapshot := sb.String()
	b.ResetTimer()
	for range b.N {
		box.snapshot = snapshot
		box.bytesEmitted = 0
		box.lineBuf.Reset()
		box.lineCount = 0
		r.consumeBox(box)
	}
}

func BenchmarkMarshalJSON(b *testing.B) {
	b.ReportAllocs()
	args := event.Args{
		"command": "echo hello world",
		"path":    "/some/long/path/with/many/segments",
		"timeout": "30s",
	}
	b.ResetTimer()
	for range b.N {
		_ = marshalJSON(args)
	}
}

func BenchmarkMarshalJSONLarge(b *testing.B) {
	b.ReportAllocs()
	args := make(event.Args, 50)
	for i := range 50 {
		args[fmt.Sprintf("key_%d", i)] = strings.Repeat("value", 20)
	}
	b.ResetTimer()
	for range b.N {
		_ = marshalJSON(args)
	}
}

func BenchmarkMarshalJSONUnencodable(b *testing.B) {
	b.ReportAllocs()
	args := event.Args{"ch": make(chan int)}
	b.ResetTimer()
	for range b.N {
		_ = marshalJSON(args)
	}
}

func BenchmarkManyQueuedTools(b *testing.B) {
	b.ReportAllocs()
	var buf bytes.Buffer
	for range b.N {
		buf.Reset()
		r := New(&buf)
		n := 1000
		for i := range n {
			id := fmt.Sprintf("tool_%d", i)
			r.ToolExecStart(id, "bash", event.Args{"command": fmt.Sprintf("echo %d", i)})
		}
		for i := range n {
			id := fmt.Sprintf("tool_%d", i)
			r.ToolExecEnd(id, false, fmt.Sprintf("output %d\n", i))
		}
	}
}

func BenchmarkFullRenderCycleSingle(b *testing.B) {
	b.ReportAllocs()
	var buf bytes.Buffer
	b.ResetTimer()
	for range b.N {
		buf.Reset()
		r := New(&buf)
		r.Thinking("let me think about this carefully")
		r.Text("here is the answer")
		r.ToolExecStart("t1", "bash", event.Args{"command": "echo test"})
		r.ToolExecUpdate("t1", "test\n")
		r.ToolExecEnd("t1", false, "test\n")
		r.TurnEnd()
	}
}

func BenchmarkFullRenderCycleParallelTools(b *testing.B) {
	b.ReportAllocs()
	var buf bytes.Buffer
	b.ResetTimer()
	for range b.N {
		buf.Reset()
		r := New(&buf)
		r.ToolExecStart("a", "bash", event.Args{"command": "long"})
		r.ToolExecStart("b", "bash", event.Args{"command": "quick"})
		r.ToolExecStart("c", "bash", event.Args{"command": "medium"})
		r.ToolExecUpdate("a", "line a1\nline a2\n")
		r.ToolExecUpdate("b", "line b1\n")
		r.ToolExecUpdate("c", "line c1\nline c2\nline c3\n")
		r.ToolExecEnd("b", false, "line b1\n")
		r.ToolExecEnd("c", false, "line c1\nline c2\nline c3\n")
		r.ToolExecEnd("a", false, "line a1\nline a2\n")
		r.AgentEnd()
	}
}

func BenchmarkFormatDuration(b *testing.B) {
	b.ReportAllocs()
	durations := []time.Duration{
		0,
		time.Millisecond,
		500 * time.Millisecond,
		time.Second,
		1500 * time.Millisecond,
		-100 * time.Millisecond,
		3661 * time.Second,
	}
	b.ResetTimer()
	for range b.N {
		for _, d := range durations {
			_ = formatDuration(d)
		}
	}
}
