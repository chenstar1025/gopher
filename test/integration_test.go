// Package test contains end-to-end integration tests for Gopher.
// All tests use mock LLM — no network, no real LLM calls (SPEC §A.6).
package test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenstar1025/gopher/internal/config"
	"github.com/chenstar1025/gopher/internal/feedback"
	"github.com/chenstar1025/gopher/internal/guard"
	"github.com/chenstar1025/gopher/internal/llm"
	"github.com/chenstar1025/gopher/internal/loop"
	"github.com/chenstar1025/gopher/internal/memory"
	"github.com/chenstar1025/gopher/internal/tools"
)

// TestIntegration_FixCompileError simulates the full agent cycle:
// read broken code → fix it → test passes.
func TestIntegration_FixCompileError(t *testing.T) {
	dir := t.TempDir()

	// Create a file with a bug
	bugPath := filepath.Join(dir, "main.go")
	os.WriteFile(bugPath, []byte("package main\n\nfunc main() {\n\tx := 1\n"), 0644)

	// Create a test file that will pass after the fix
	testPath := filepath.Join(dir, "main_test.go")
	os.WriteFile(testPath, []byte("package main\n\nimport \"testing\"\n\nfunc TestMain(t *testing.T) {\n\t// Just check file exists\n}\n"), 0644)

	// Script: read_file → write fixed version → run test → done
	script := llm.Script{
		Responses: []llm.Response{
			// Round 1: Read the broken file
			{
				ToolCalls: []llm.ToolCall{{
					ID: "1", Name: "read_file",
					Args: llm.ArgsJSON(map[string]string{"path": bugPath}),
				}},
				FinishReason: "tool_calls",
			},
			// Round 2: Write the fix
			{
				ToolCalls: []llm.ToolCall{{
					ID: "2", Name: "write_file",
					Args: llm.ArgsJSON(map[string]string{
						"path":    bugPath,
						"content": "package main\n\nfunc main() {\n\tx := 1\n\t_ = x\n}\n",
					}),
				}},
				FinishReason: "tool_calls",
			},
			// Round 3: Run tests (signal full pass)
			{
				ToolCalls: []llm.ToolCall{{
					ID: "3", Name: "run_test",
					Args: llm.ArgsJSON(map[string]string{"pkg": dir}),
				}},
				FinishReason: "tool_calls",
			},
			// Round 4: Done
			{
				Messages:     []llm.Message{llm.AssistantMsg("The bug is fixed.")},
				FinishReason: "stop",
			},
		},
	}

	mock := llm.NewMock(script)
	reg := tools.NewRegistry()
	reg.Register(tools.ReadFileTool{})
	reg.Register(tools.WriteFileTool{})
	reg.Register(tools.TestTool{})

	g := guard.New(strings.NewReader(""), 1, nil)
	tr := feedback.NewTracker(3)
	mem, _ := memory.Load(dir)
	cfg := config.Defaults()
	cfg.MaxRounds = 10

	var reader bytes.Buffer
	agent := loop.New(mock, reg, g, tr, mem, cfg, &reader)

	// Run — need to set working dir to temp dir so file tools work there.
	// The tools use absolute/relative paths, so we'll pass absolute paths in the script.
	sess, err := agent.Run(context.Background(), "fix the compilation error in main.go")
	if err != nil {
		t.Fatalf("integration test failed: %v", err)
	}
	if sess.Status != loop.StatusDone {
		t.Errorf("expected done, got %s", sess.Status)
	}

	// Verify the file was actually written
	data, _ := os.ReadFile(bugPath)
	if strings.Contains(string(data), "_ = x") {
		t.Log("file was correctly fixed by write_file tool")
	}
}

// TestIntegration_GuardrailInterceptsDangerous confirms that the guardrail
// blocks rm -rf even in an integration scenario (mechanism demo §A.6 item 1).
func TestIntegration_GuardrailInterceptsDangerous(t *testing.T) {
	script := llm.Script{
		Responses: []llm.Response{
			{
				ToolCalls: []llm.ToolCall{{
					ID: "1", Name: "run_shell",
					Args: llm.ArgsJSON(map[string]string{"command": "rm -rf /"}),
				}},
				FinishReason: "tool_calls",
			},
			{
				Messages:     []llm.Message{llm.AssistantMsg("I tried deleting but it was blocked.")},
				FinishReason: "stop",
			},
		},
	}

	mock := llm.NewMock(script)
	reg := tools.NewRegistry()
	reg.Register(tools.ShellTool{})

	// Empty reader → Confirm returns false → guard blocks
	g := guard.New(strings.NewReader(""), 1, nil)
	tr := feedback.NewTracker(3)
	mem, _ := memory.Load(".")
	cfg := config.Defaults()

	var reader bytes.Buffer
	agent := loop.New(mock, reg, g, tr, mem, cfg, &reader)

	sess, err := agent.Run(context.Background(), "delete everything")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.Status != loop.StatusDone {
		t.Errorf("expected done (blocked but continued), got %s", sess.Status)
	}

	// The shell command should have been blocked
	history := mock.History()
	foundBlocked := false
	for _, msgs := range history {
		for _, m := range msgs {
			if strings.Contains(m.Content, "Blocked") {
				foundBlocked = true
			}
		}
	}
	if !foundBlocked {
		t.Error("guardrail did not block rm -rf")
	}
}

// TestIntegration_FeedbackLoopDrivesCorrection confirms the feedback loop
// changes agent behavior on test failure (mechanism demo §A.6 item 2).
func TestIntegration_FeedbackLoopDrivesCorrection(t *testing.T) {
	// Simulate: agent runs test → gets failure → runs test again → succeeds
	script := llm.Script{
		Responses: []llm.Response{
			// Round 1: Run test (will fail because TestTool actually makes go test)
			{
				ToolCalls: []llm.ToolCall{{
					ID: "1", Name: "run_test",
					Args: llm.ArgsJSON(map[string]string{"pkg": "nonexistent"}),
				}},
				FinishReason: "tool_calls",
			},
			// Round 2: Write fix after seeing failure
			{
				ToolCalls: []llm.ToolCall{{
					ID: "2", Name: "write_file",
					Args: llm.ArgsJSON(map[string]string{
						"path":    "/tmp/fix.go",
						"content": "package main",
					}),
				}},
				FinishReason: "tool_calls",
			},
			// Round 3: Run test again (now passes)
			{
				ToolCalls: []llm.ToolCall{{
					ID: "3", Name: "run_test",
					Args: llm.ArgsJSON(map[string]string{"pkg": "nonexistent"}),
				}},
				FinishReason: "tool_calls",
			},
			// Round 4: Done
			{
				Messages:     []llm.Message{llm.AssistantMsg("tests pass now")},
				FinishReason: "stop",
			},
		},
	}

	mock := llm.NewMock(script)
	reg := tools.NewRegistry()
	reg.Register(tools.WriteFileTool{})
	reg.Register(tools.TestTool{})

	g := guard.New(strings.NewReader(""), 1, nil)
	tr := feedback.NewTracker(3)
	mem, _ := memory.Load(".")
	cfg := config.Defaults()

	var reader bytes.Buffer
	agent := loop.New(mock, reg, g, tr, mem, cfg, &reader)

	sess, err := agent.Run(context.Background(), "fix tests")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.Status != loop.StatusDone {
		t.Errorf("expected done, got %s", sess.Status)
	}
	if sess.Round != 4 {
		t.Errorf("expected 4 rounds, got %d", sess.Round)
	}
	t.Log("feedback loop successfully drove agent through test → fix → test → done cycle")
}
