package llm

import (
	"context"
	"testing"
)

func TestMockLLM_ReturnsScriptedResponses(t *testing.T) {
	script := Script{
		Responses: []Response{
			{
				ToolCalls:    []ToolCall{{ID: "1", Name: "read_file", Args: ArgsJSON(map[string]string{"path": "main.go"})}},
				FinishReason: "tool_calls",
			},
			{
				Messages:     []Message{{Role: "assistant", Content: "done"}},
				FinishReason: "stop",
			},
		},
	}

	mock := NewMock(script)
	ctx := context.Background()
	msgs := []Message{UserMsg("fix main.go")}

	// First call — should return tool_calls
	resp1, err := mock.Chat(ctx, msgs, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp1.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp1.ToolCalls))
	}
	if resp1.ToolCalls[0].Name != "read_file" {
		t.Errorf("expected tool read_file, got %s", resp1.ToolCalls[0].Name)
	}

	// Second call — should return stop
	resp2, err := mock.Chat(ctx, msgs, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp2.FinishReason != "stop" {
		t.Errorf("expected finish_reason stop, got %s", resp2.FinishReason)
	}
}

func TestMockLLM_PanicsOnExhaustedScript(t *testing.T) {
	script := Script{
		Responses: []Response{{FinishReason: "stop"}},
	}
	mock := NewMock(script)
	ctx := context.Background()

	_, err := mock.Chat(ctx, nil, nil)
	if err != nil {
		t.Fatalf("first call should succeed: %v", err)
	}

	_, err = mock.Chat(ctx, nil, nil)
	if err == nil {
		t.Fatal("expected error on exhausted script")
	}
}

func TestMockLLM_TracksCallsAndHistory(t *testing.T) {
	script := Script{
		Responses: []Response{
			{FinishReason: "stop"},
			{FinishReason: "stop"},
		},
	}
	mock := NewMock(script)
	ctx := context.Background()

	mock.Chat(ctx, []Message{UserMsg("hello")}, nil)
	if mock.CallCount() != 1 {
		t.Errorf("expected 1 call, got %d", mock.CallCount())
	}

	mock.Chat(ctx, []Message{UserMsg("world")}, nil)
	if mock.CallCount() != 2 {
		t.Errorf("expected 2 calls, got %d", mock.CallCount())
	}

	history := mock.History()
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}

	last := mock.LastMessages()
	if len(last) != 1 || last[0].Content != "world" {
		t.Errorf("last messages mismatch")
	}
}

func TestMockLLM_Reset(t *testing.T) {
	script := Script{
		Responses: []Response{{FinishReason: "stop"}},
	}
	mock := NewMock(script)
	ctx := context.Background()

	mock.Chat(ctx, nil, nil)
	mock.Reset()

	if mock.CallCount() != 0 {
		t.Errorf("expected 0 after reset, got %d", mock.CallCount())
	}
	if len(mock.History()) != 0 {
		t.Error("expected empty history after reset")
	}

	// Should work again
	_, err := mock.Chat(ctx, nil, nil)
	if err != nil {
		t.Fatalf("should work after reset: %v", err)
	}
}

func TestArgsJSON(t *testing.T) {
	args := ArgsJSON(map[string]string{"path": "main.go"})
	if args["path"] != "main.go" {
		t.Errorf("expected path=main.go, got %v", args["path"])
	}
}

func TestMessageHelpers(t *testing.T) {
	userMsg := UserMsg("hello")
	if userMsg.Role != "user" || userMsg.Content != "hello" {
		t.Error("UserMsg mismatch")
	}

	assistantMsg := AssistantMsg("hi")
	if assistantMsg.Role != "assistant" || assistantMsg.Content != "hi" {
		t.Error("AssistantMsg mismatch")
	}

	toolMsg := ToolResultMsg("call_1", "output")
	if toolMsg.Role != "tool" || toolMsg.ToolCallID != "call_1" || toolMsg.Content != "output" {
		t.Error("ToolResultMsg mismatch")
	}
}
