package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Script defines a deterministic sequence of responses that the mock LLM
// returns for consecutive Chat calls. Each entry maps 1:1 to a Chat invocation.
type Script struct {
	Responses []Response
}

// MockLLM implements LLM with a scripted, deterministic response sequence.
// It is the key enabler for testing harness mechanisms without a real LLM.
type MockLLM struct {
	mu      sync.Mutex
	calls   int
	history [][]Message
	script  Script
}

// NewMock creates a MockLLM from a script of predetermined responses.
func NewMock(script Script) *MockLLM {
	return &MockLLM{script: script}
}

// Chat returns the next scripted response. It panics if called more times
// than the script has responses, which is a test bug.
func (m *MockLLM) Chat(_ context.Context, messages []Message, _ []ToolDef) (*Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Save a copy of input messages for test inspection
	m.history = append(m.history, append([]Message(nil), messages...))

	if m.calls >= len(m.script.Responses) {
		return nil, fmt.Errorf("mock: no scripted response for call %d (have %d)", m.calls, len(m.script.Responses))
	}

	resp := m.script.Responses[m.calls]
	m.calls++
	return &resp, nil
}

// CallCount returns the number of times Chat was invoked.
func (m *MockLLM) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// History returns all message batches sent to this mock, for test assertions.
func (m *MockLLM) History() [][]Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([][]Message(nil), m.history...)
}

// LastMessages returns the most recent message batch sent to this mock.
func (m *MockLLM) LastMessages() []Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.history) == 0 {
		return nil
	}
	return append([]Message(nil), m.history[len(m.history)-1]...)
}

// Reset clears call history for reuse.
func (m *MockLLM) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = 0
	m.history = nil
}

// Message helpers for constructing mock scripts concisely.

// AssistantMsg creates an assistant message with optional tool calls.
func AssistantMsg(content string, toolCalls ...ToolCall) Message {
	return Message{Role: "assistant", Content: content, ToolCalls: toolCalls}
}

// UserMsg creates a user message.
func UserMsg(content string) Message {
	return Message{Role: "user", Content: content}
}

// ToolResultMsg creates a tool result message.
func ToolResultMsg(toolCallID, output string) Message {
	return Message{Role: "tool", ToolCallID: toolCallID, Content: output}
}

// JSON helpers for constructing ToolCall.Args safely.

// ArgsJSON marshals a Go value into the map[string]any format expected by ToolCall.Args.
func ArgsJSON(v any) map[string]any {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("ArgsJSON: %v", err))
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		panic(fmt.Sprintf("ArgsJSON unmarshal: %v", err))
	}
	return m
}
