package render

import (
	"bytes"
	"strings"
	"testing"
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

func TestToolCallWithArgs(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.ToolCall("bash", map[string]any{"command": "ls -la"})
	got := buf.String()
	if !strings.Contains(got, "🔧 bash") {
		t.Errorf("missing prefix: %q", got)
	}
	if !strings.Contains(got, `"command":"ls -la"`) {
		t.Errorf("missing args JSON: %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("output should end with newline: %q", got)
	}
}

func TestToolCallNoArgs(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.ToolCall("ping", nil)
	got := buf.String()
	if !strings.Contains(got, "🔧 ping") {
		t.Errorf("missing prefix: %q", got)
	}
	if strings.Contains(got, "{}") {
		t.Errorf("should not render empty args object: %q", got)
	}
}

func TestToolExecStartBash(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.ToolExecStart("bash", map[string]any{"command": "echo hi"})
	got := buf.String()
	if !strings.Contains(got, "⚡ bash: echo hi") {
		t.Errorf("expected ⚡ bash: echo hi, got %q", got)
	}
	if strings.Contains(got, `"command"`) {
		t.Errorf("bash should render literal command, not JSON: %q", got)
	}
}

func TestToolExecStartNonBash(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.ToolExecStart("http", map[string]any{"url": "https://example.com"})
	got := buf.String()
	if !strings.Contains(got, "⚡ http ") {
		t.Errorf("expected ⚡ http prefix, got %q", got)
	}
	if !strings.Contains(got, `"url":"https://example.com"`) {
		t.Errorf("expected JSON args, got %q", got)
	}
}

func TestToolExecEndSuccess(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.ToolExecEnd("bash", false, "  hello  ")
	got := buf.String()
	if !strings.Contains(got, "✓ bash → hello") {
		t.Errorf("expected ✓ marker and trimmed summary, got %q", got)
	}
}

func TestToolExecEndError(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.ToolExecEnd("bash", true, "boom")
	got := buf.String()
	if !strings.Contains(got, "✗ bash → boom") {
		t.Errorf("expected ✗ marker, got %q", got)
	}
}

func TestToolExecEndTruncates(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	long := strings.Repeat("a", 500)
	r.ToolExecEnd("bash", false, long)
	got := buf.String()
	if !strings.Contains(got, "...") {
		t.Errorf("expected truncation marker, got %q", got)
	}
	if strings.Count(got, "a") > summaryMaxLen+10 {
		t.Errorf("summary not truncated; len(a)=%d in %q", strings.Count(got, "a"), got)
	}
}

func TestMarshalJSONNoHTMLEscape(t *testing.T) {
	t.Parallel()
	// With HTML escaping disabled, raw &, <, > survive verbatim.
	// If escaping were enabled they would become &, <, >
	// and "&&" / "<c>" would not appear as substrings.
	got := marshalJSON(map[string]any{"cmd": "a && b <c>"})
	for _, want := range []string{"&&", "<c>"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected raw %q in %q", want, got)
		}
	}
}

func TestEnsureNewlineMidLine(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.Text("hello")         // mid-line
	r.ToolCall("ping", nil) // must inject \n first
	got := buf.String()
	if !strings.HasPrefix(got, "hello\n") {
		t.Errorf("expected newline injected before tool call, got %q", got)
	}
}

func TestTextAfterToolCallNoExtraNewline(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := New(&buf)
	r.ToolCall("ping", nil) // ends with \n
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

func TestTruncate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in     string
		maxLen int
		want   string
	}{
		{"abc", 10, "abc"},
		{"abcdef", 3, "abc..."},
		{"", 5, ""},
	}
	for _, c := range cases {
		if got := truncate(c.in, c.maxLen); got != c.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", c.in, c.maxLen, got, c.want)
		}
	}
}
