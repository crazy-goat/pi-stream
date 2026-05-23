package event

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEnvelopeUnmarshalResponseSuccess(t *testing.T) {
	t.Parallel()
	raw := `{"type":"response","success":true}`
	var env Envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Type != TypeResponse {
		t.Errorf("type = %q, want %q", env.Type, TypeResponse)
	}
	if env.Success == nil || !*env.Success {
		t.Errorf("Success = %v, want true", env.Success)
	}
}

func TestEnvelopeUnmarshalResponseFailure(t *testing.T) {
	t.Parallel()
	raw := `{"type":"response","success":false,"error":"boom"}`
	var env Envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Success == nil || *env.Success {
		t.Errorf("Success = %v, want false", env.Success)
	}
	if env.Error != "boom" {
		t.Errorf("Error = %q, want %q", env.Error, "boom")
	}
}

func TestEnvelopeUnmarshalTextDelta(t *testing.T) {
	t.Parallel()
	raw := `{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"hello"}}`
	var env Envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.AssistantMessageEvent == nil {
		t.Fatal("AssistantMessageEvent is nil")
	}
	if env.AssistantMessageEvent.Type != MsgTypeTextDelta {
		t.Errorf("inner type = %q", env.AssistantMessageEvent.Type)
	}
	if env.AssistantMessageEvent.Delta != "hello" {
		t.Errorf("delta = %q", env.AssistantMessageEvent.Delta)
	}
}

func TestEnvelopeUnmarshalToolCallEnd(t *testing.T) {
	t.Parallel()
	raw := `{"type":"message_update","assistantMessageEvent":{"type":"toolcall_end","toolCall":{"name":"bash","arguments":{"command":"ls"}}}}`
	var env Envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	tc := env.AssistantMessageEvent.ToolCall
	if tc == nil {
		t.Fatal("ToolCall is nil")
	}
	if tc.Name != "bash" {
		t.Errorf("name = %q", tc.Name)
	}
	if got, _ := tc.Arguments.Command(); got != "ls" {
		t.Errorf("command = %q", got)
	}
}

func TestEnvelopeUnmarshalToolExecutionEnd(t *testing.T) {
	t.Parallel()
	raw := `{"type":"tool_execution_end","toolName":"bash","isError":false,"result":{"content":[{"text":"hello"},{"text":" world"}]}}`
	var env Envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.ToolName != "bash" {
		t.Errorf("toolName = %q", env.ToolName)
	}
	if got := env.Result.SummaryText(); got != "hello world" {
		t.Errorf("SummaryText = %q", got)
	}
}

func TestResultSummaryTextNil(t *testing.T) {
	t.Parallel()
	var r *Result
	if got := r.SummaryText(); got != "" {
		t.Errorf("nil SummaryText = %q, want \"\"", got)
	}
}

func TestResultSummaryTextEmpty(t *testing.T) {
	t.Parallel()
	r := &Result{}
	if got := r.SummaryText(); got != "" {
		t.Errorf("empty SummaryText = %q, want \"\"", got)
	}
}

func TestResultSummaryTextSingleBlock(t *testing.T) {
	t.Parallel()
	r := &Result{Content: []ResultContent{{Text: "hello"}}}
	if got := r.SummaryText(); got != "hello" {
		t.Errorf("SummaryText = %q, want %q", got, "hello")
	}
}

func TestResultSummaryTextMultipleBlocks(t *testing.T) {
	t.Parallel()
	r := &Result{Content: []ResultContent{{Text: "hello"}, {Text: " world"}, {Text: "!"}}}
	if got := r.SummaryText(); got != "hello world!" {
		t.Errorf("SummaryText = %q, want %q", got, "hello world!")
	}
}

func BenchmarkSummaryTextSingle(b *testing.B) {
	b.ReportAllocs()
	r := &Result{Content: []ResultContent{{Text: "hello world, this is a single block of text"}}}
	b.ResetTimer()
	for range b.N {
		_ = r.SummaryText()
	}
}

func BenchmarkSummaryTextMany(b *testing.B) {
	b.ReportAllocs()
	content := make([]ResultContent, 100)
	for i := range content {
		content[i] = ResultContent{Text: "block of text with some content for testing purposes "}
	}
	r := &Result{Content: content}
	b.ResetTimer()
	for range b.N {
		_ = r.SummaryText()
	}
}

func BenchmarkSummaryTextLarge(b *testing.B) {
	b.ReportAllocs()
	large := make([]ResultContent, 10)
	for i := range large {
		large[i] = ResultContent{Text: strings.Repeat("x", 10000)}
	}
	r := &Result{Content: large}
	b.ResetTimer()
	for range b.N {
		_ = r.SummaryText()
	}
}

func BenchmarkSummaryTextNil(b *testing.B) {
	b.ReportAllocs()
	var r *Result
	b.ResetTimer()
	for range b.N {
		_ = r.SummaryText()
	}
}

func TestEventTypeConstants(t *testing.T) {
	tests := []struct {
		got  string
		want string
	}{
		{TypeResponse, "response"},
		{TypeMessageUpdate, "message_update"},
		{TypeToolExecStart, "tool_execution_start"},
		{TypeToolExecUpdate, "tool_execution_update"},
		{TypeToolExecEnd, "tool_execution_end"},
		{TypeTurnStart, "turn_start"},
		{TypeTurnEnd, "turn_end"},
		{TypeAgentEnd, "agent_end"},
		{TypeError, "error"},
		{MsgTypeThinkingDelta, "thinking_delta"},
		{MsgTypeThinksDelta, "thinks_delta"},
		{MsgTypeTextDelta, "text_delta"},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("constant = %q, want %q", tc.got, tc.want)
		}
	}
}
