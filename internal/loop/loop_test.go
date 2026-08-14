package loop

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chenstar1025/gopher/internal/config"
	"github.com/chenstar1025/gopher/internal/feedback"
	"github.com/chenstar1025/gopher/internal/guard"
	"github.com/chenstar1025/gopher/internal/llm"
	"github.com/chenstar1025/gopher/internal/memory"
	"github.com/chenstar1025/gopher/internal/tools"
)

func setupAgent(mock *llm.MockLLM, confirmInput string) *Agent {
	reg := tools.NewRegistry()
	reg.Register(tools.ReadFileTool{})
	reg.Register(tools.WriteFileTool{})
	reg.Register(tools.ShellTool{})
	reg.Register(tools.TestTool{})

	g := guard.New(strings.NewReader(confirmInput), 5*time.Second, nil)

	tr := feedback.NewTracker(3)
	mem, _ := memory.Load(".")

	cfg := config.Defaults()
	cfg.MaxRounds = 10

	var reader bytes.Buffer
	return New(mock, reg, g, tr, mem, cfg, &reader)
}

func TestLoop_StopsWhenNoToolCalls(t *testing.T) {
	script := llm.Script{
		Responses: []llm.Response{
			{
				Messages:     []llm.Message{llm.AssistantMsg("All done!")},
				FinishReason: "stop",
			},
		},
	}
	mock := llm.NewMock(script)
	agent := setupAgent(mock, "")

	sess, err := agent.Run(context.Background(), "say hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.Status != StatusDone {
		t.Errorf("expected done, got %s", sess.Status)
	}
	if sess.Round != 1 {
		t.Errorf("expected 1 round, got %d", sess.Round)
	}
	if mock.CallCount() != 1 {
		t.Errorf("expected 1 LLM call, got %d", mock.CallCount())
	}
}

func TestLoop_ExecutesToolCalls(t *testing.T) {
	script := llm.Script{
		Responses: []llm.Response{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "1", Name: "read_file", Args: llm.ArgsJSON(map[string]string{"path": "testdata/hello.txt"})},
				},
				FinishReason: "tool_calls",
			},
			{
				Messages:     []llm.Message{llm.AssistantMsg("done")},
				FinishReason: "stop",
			},
		},
	}
	mock := llm.NewMock(script)
	agent := setupAgent(mock, "")

	sess, err := agent.Run(context.Background(), "read hello.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.Status != StatusDone {
		t.Errorf("expected done, got %s", sess.Status)
	}
	if mock.CallCount() != 2 {
		t.Errorf("expected 2 LLM calls, got %d", mock.CallCount())
	}
}

func TestLoop_MultipleRounds(t *testing.T) {
	dir := t.TempDir()
	script := llm.Script{
		Responses: []llm.Response{
			{ToolCalls: []llm.ToolCall{{ID: "1", Name: "read_file", Args: llm.ArgsJSON(map[string]string{"path": dir + "/main.go"})}}, FinishReason: "tool_calls"},
			{ToolCalls: []llm.ToolCall{{ID: "2", Name: "write_file", Args: llm.ArgsJSON(map[string]string{"path": dir + "/main.go", "content": "fixed"})}}, FinishReason: "tool_calls"},
			{ToolCalls: []llm.ToolCall{{ID: "3", Name: "run_test", Args: llm.ArgsJSON(map[string]string{"pkg": "nonexistent"})}}, FinishReason: "tool_calls"},
			{Messages: []llm.Message{llm.AssistantMsg("fixed!")}, FinishReason: "stop"},
		},
	}
	mock := llm.NewMock(script)
	agent := setupAgent(mock, "")

	sess, err := agent.Run(context.Background(), "fix main.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.Status != StatusDone {
		t.Errorf("expected done, got %s", sess.Status)
	}
	if sess.Round != 4 {
		t.Errorf("expected 4 rounds, got %d", sess.Round)
	}
}

func TestLoop_MaxRounds(t *testing.T) {
	// Create a script that always returns a tool call, exceeding MaxRounds
	var responses []llm.Response
	for i := 0; i < 15; i++ {
		responses = append(responses, llm.Response{
			ToolCalls:    []llm.ToolCall{{ID: "x", Name: "read_file", Args: llm.ArgsJSON(map[string]string{"path": "main.go"})}},
			FinishReason: "tool_calls",
		})
	}
	script := llm.Script{Responses: responses}
	mock := llm.NewMock(script)
	agent := setupAgent(mock, "")

	sess, err := agent.Run(context.Background(), "loop test")
	if err == nil {
		t.Fatal("expected error for max rounds exceeded")
	}
	if sess.Status != StatusFailed {
		t.Errorf("expected failed, got %s", sess.Status)
	}
}

func TestLoop_ContextCancellation(t *testing.T) {
	script := llm.Script{
		Responses: []llm.Response{
			{ToolCalls: []llm.ToolCall{{ID: "1", Name: "read_file", Args: llm.ArgsJSON(map[string]string{"path": "main.go"})}}, FinishReason: "tool_calls"},
		},
	}
	mock := llm.NewMock(script)
	agent := setupAgent(mock, "")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := agent.Run(ctx, "test")
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestLoop_GuardBlocksDangerousCommand(t *testing.T) {
	script := llm.Script{
		Responses: []llm.Response{
			{
				ToolCalls:    []llm.ToolCall{{ID: "1", Name: "run_shell", Args: llm.ArgsJSON(map[string]string{"command": "rm -rf /"})}},
				FinishReason: "tool_calls",
			},
			{
				Messages:     []llm.Message{llm.AssistantMsg("tried but blocked")},
				FinishReason: "stop",
			},
		},
	}
	mock := llm.NewMock(script)
	// Empty reader → Confirm returns false
	agent := setupAgent(mock, "")

	sess, err := agent.Run(context.Background(), "delete root")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Guard should have blocked rm -rf, agent continued to next round
	if sess.Status != StatusDone {
		t.Errorf("expected done, got %s", sess.Status)
	}
}

func TestLoop_FeedbackInjectionOnTestFailure(t *testing.T) {
	dir := t.TempDir()
	script := llm.Script{
		Responses: []llm.Response{
			{
				ToolCalls:    []llm.ToolCall{{ID: "1", Name: "run_test", Args: llm.ArgsJSON(map[string]string{"pkg": "nonexistent"})}},
				FinishReason: "tool_calls",
			},
			{
				ToolCalls: []llm.ToolCall{{
					ID: "2", Name: "write_file",
					Args: llm.ArgsJSON(map[string]string{"path": dir + "/main.go", "content": "fixed"}),
				}},
				FinishReason: "tool_calls",
			},
			{
				ToolCalls:    []llm.ToolCall{{ID: "3", Name: "run_test", Args: llm.ArgsJSON(map[string]string{"pkg": "nonexistent"})}},
				FinishReason: "tool_calls",
			},
			{Messages: []llm.Message{llm.AssistantMsg("all good now")}, FinishReason: "stop"},
		},
	}
	mock := llm.NewMock(script)
	agent := setupAgent(mock, "")

	sess, err := agent.Run(context.Background(), "fix the code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.Status != StatusDone {
		t.Errorf("expected done, got %s", sess.Status)
	}
	// Should have run 3+ rounds (test → fix → test → done)
	if sess.Round < 3 {
		t.Errorf("expected at least 3 rounds, got %d", sess.Round)
	}
}
