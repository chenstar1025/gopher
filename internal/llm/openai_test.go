package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIClient_Chat_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request shape
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected auth header")
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"choices": [{
				"message": {
					"role": "assistant",
					"content": "Hello!",
					"tool_calls": [{
						"id": "call_1",
						"type": "function",
						"function": {
							"name": "read_file",
							"arguments": "{\"path\":\"main.go\"}"
						}
					}]
				},
				"finish_reason": "tool_calls"
			}]
		}`))
	}))
	defer server.Close()

	client := NewOpenAI(server.URL, "test-key", "gpt-4o")
	resp, err := client.Chat(context.Background(), []Message{
		UserMsg("read main.go"),
	}, []ToolDef{
		{Name: "read_file", Description: "read a file"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.FinishReason != "tool_calls" {
		t.Errorf("expected tool_calls, got %s", resp.FinishReason)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "read_file" {
		t.Errorf("expected read_file, got %s", resp.ToolCalls[0].Name)
	}
	if resp.ToolCalls[0].Args["path"] != "main.go" {
		t.Errorf("expected path=main.go, got %v", resp.ToolCalls[0].Args["path"])
	}
}

func TestOpenAIClient_Chat_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"invalid api key","type":"auth_error"}}`))
	}))
	defer server.Close()

	client := NewOpenAI(server.URL, "bad-key", "gpt-4o")
	_, err := client.Chat(context.Background(), []Message{
		UserMsg("hi"),
	}, nil)

	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestOpenAIClient_Chat_Unreachable(t *testing.T) {
	client := NewOpenAI("http://127.0.0.1:99999", "key", "gpt-4o")
	_, err := client.Chat(context.Background(), []Message{
		UserMsg("hi"),
	}, nil)

	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}
