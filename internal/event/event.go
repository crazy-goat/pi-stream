// Package event defines typed structures for the JSON event stream
// emitted by `pi --mode rpc` over stdout.
//
// Each line on stdout is a JSON object that fits the Envelope shape. The
// embedded fields are sparsely populated depending on the envelope Type.
package event

import "strings"

// Args represents tool arguments as a JSON object whose keys vary by tool
// type (e.g. bash has "command", http has "url"). Use accessor methods
// instead of raw map lookups with type assertions.
type Args map[string]any

// Command returns the value of the "command" key, which bash tools populate.
func (a Args) Command() (string, bool) {
	s, ok := a["command"].(string)
	return s, ok
}

// Envelope is the top-level JSON object produced by pi for every event.
// Most fields are optional and only populated for the matching Type.
type Envelope struct {
	Type string `json:"type"`

	// Populated for type=="response".
	Success *bool  `json:"success,omitempty"`
	Error   string `json:"error,omitempty"`

	// Populated for type=="message_update".
	AssistantMessageEvent *AssistantMessageEvent `json:"assistantMessageEvent,omitempty"`

	// Populated for type=="tool_execution_*".
	ToolCallID    string  `json:"toolCallId,omitempty"`
	ToolName      string  `json:"toolName,omitempty"`
	Args          Args    `json:"args,omitempty"`
	IsError       bool    `json:"isError,omitempty"`
	Result        *Result `json:"result,omitempty"`
	PartialResult *Result `json:"partialResult,omitempty"`
}

// AssistantMessageEvent describes a single token-level event emitted by the
// model: a thinking chunk, a text chunk, or a tool-call lifecycle event.
type AssistantMessageEvent struct {
	Type     string    `json:"type"`
	Delta    string    `json:"delta,omitempty"`
	ToolCall *ToolCall `json:"toolCall,omitempty"`
	Error    string    `json:"error,omitempty"`
}

// ToolCall represents a tool invocation the model has decided to make.
// Arguments are only present once the call is complete (toolcall_end).
type ToolCall struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments Args   `json:"arguments,omitempty"`
}

// Result wraps the response returned by a tool execution.
type Result struct {
	Content []ResultContent `json:"content"`
}

// ResultContent is a single content block inside a tool result. Only the
// text variant is consumed by the renderer today.
type ResultContent struct {
	Text string `json:"text,omitempty"`
}

// SummaryText concatenates all text content blocks into a single string.
// Non-text blocks (if pi ever adds them) are ignored.
func (r *Result) SummaryText() string {
	if r == nil || len(r.Content) == 0 {
		return ""
	}
	if len(r.Content) == 1 {
		return r.Content[0].Text
	}
	var sb strings.Builder
	for _, c := range r.Content {
		sb.WriteString(c.Text)
	}
	return sb.String()
}
