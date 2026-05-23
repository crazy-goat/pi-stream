package render

import (
	"bytes"
	"strings"
	"testing"
	"time"

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
	r.ToolExecStart("t1", "bash", map[string]any{"command": "echo hi"})
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
	r.ToolExecStart("t1", "http", map[string]any{"url": "https://example.com"})
	got := buf.String()
	if !strings.Contains(got, "┌─ ⚡ http") {
		t.Errorf("expected top-of-box header, got %q", got)
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
	r.ToolExecStart("t1", "bash", map[string]any{"command": "x"})

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
	r.ToolExecStart("t1", "bash", map[string]any{"command": "x"})
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
	r.ToolExecStart("t1", "bash", map[string]any{"command": "x"})
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
	r.ToolExecStart("t1", "bash", map[string]any{"command": "x"})
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
	r.ToolExecStart("t1", "bash", map[string]any{"command": "x"})
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
	r.ToolExecStart("t1", "bash", map[string]any{"command": "x"})
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
	r.ToolExecStart("t1", "bash", map[string]any{"command": "x"})
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
	r.ToolExecStart("t1", "edit", map[string]any{"path": "/x"})
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
	r.ToolExecStart("t1", "bash", map[string]any{"command": "x"})
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
	got := marshalJSON(map[string]any{"cmd": "a && b <c>"})
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
	got := marshalJSON(map[string]any{"ch": make(chan int)})
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

func TestParallelToolsRenderSequentially(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	// Two parallel tools: starts arrive before the first end.
	r.ToolExecStart("a", "bash", map[string]any{"command": "echo hello"})
	r.ToolExecStart("b", "bash", map[string]any{"command": "echo world"})
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
	r.ToolExecStart("a", "bash", map[string]any{"command": "long-running"})
	r.ToolExecStart("b", "bash", map[string]any{"command": "quick"})
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
