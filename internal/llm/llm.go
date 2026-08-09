// Package llm provides the LLM abstraction layer (SPEC §3.3).
// It defines a common interface for both real and mock LLM backends.
package llm

import "context"

// Message represents a chat message.
type Message struct {
	Role       string     // system | user | assistant | tool
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
}

// ToolCall represents a tool invocation requested by the LLM.
type ToolCall struct {
	ID   string
	Name string
	Args map[string]any
}

// ToolDef describes a tool available to the LLM.
type ToolDef struct {
	Name        string
	Description string
	Parameters  map[string]any // JSON Schema
}

// Response is the LLM's reply to a Chat request.
type Response struct {
	Messages   []Message
	ToolCalls  []ToolCall
	FinishReason string // stop | tool_calls | length | error
}

// LLM is the common interface for LLM backends.
type LLM interface {
	Chat(ctx context.Context, messages []Message, tools []ToolDef) (*Response, error)
}
