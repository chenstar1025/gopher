// Package tools implements the tool system (SPEC §3.4) and the shared action
// data model (SPEC §6.2) used across the harness.
package tools

// ToolCall is a single tool invocation produced by the LLM (SPEC §6.2).
// Name identifies the tool (e.g. "run_shell") and Args holds the JSON
// parameters passed to it.
type ToolCall struct {
	ID   string
	Name string
	Args map[string]any
}

// ToolResult is the outcome of executing a ToolCall (SPEC §6.2).
type ToolResult struct {
	ToolCallID string
	Success    bool
	Output     string
	Error      string
}
