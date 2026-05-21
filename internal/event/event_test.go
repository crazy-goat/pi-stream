package event

import (
	"encoding/json"
	"testing"
)

func TestEnvelopeUnmarshalResponseSuccess(t *testing.T) {
	t.Parallel()
	raw := `{"type":"response","success":true}`
	var env Envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Type != "response" {
		t.Errorf("type = %q, want %q", env.Type, "response")
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
	if env.AssistantMessageEvent.Type != "text_delta" {
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
	if got, _ := tc.Arguments["command"].(string); got != "ls" {
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
		t.Errorf("empty SummaryText = %q", got)
	}
}
