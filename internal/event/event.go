// Package event defines typed structures for the JSON event stream
// emitted by `pi --mode rpc` over stdout.
//
// Each line on stdout is a JSON object that fits the Envelope shape. The
// embedded fields are sparsely populated depending on the envelope Type.
package event

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
	ToolName string         `json:"toolName,omitempty"`
	Args     map[string]any `json:"args,omitempty"`
	IsError  bool           `json:"isError,omitempty"`
	Result   *Result        `json:"result,omitempty"`
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
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Arguments map[string]any `json:"arguments,omitempty"`
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
	if r == nil {
		return ""
	}
	var b []byte
	for _, c := range r.Content {
		b = append(b, c.Text...)
	}
	return string(b)
}
