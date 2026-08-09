// Package tools implements the tool system (SPEC §3.4) and the shared action
// data model (SPEC §6.2) used across the harness.
package tools

import (
	"fmt"
	"sync"
)

// ToolCall is a single tool invocation produced by the LLM (SPEC §6.2).
type ToolCall struct {
	ID   string
	Name string
	Args map[string]any // key: "path" for file tools, "command" for run_shell
}

// ToolResult is the outcome of executing a ToolCall (SPEC §6.2).
type ToolResult struct {
	ToolCallID string
	Success    bool
	Output     string
	Error      string
}

// PermissionLevel classifies the risk of a tool.
type PermissionLevel string

const (
	PermSafe     PermissionLevel = "safe"
	PermDangerous PermissionLevel = "dangerous"
)

// Tool defines the interface every tool must implement.
type Tool interface {
	Name() string
	Description() string
	PermissionLevel() PermissionLevel
	Parameters() map[string]any // JSON Schema
	Execute(args map[string]any) ToolResult
}

// ToolRegistry maps tool names to Tool implementations.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry.
func (r *ToolRegistry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
}

// Get looks up a tool by name. Returns nil if not found.
func (r *ToolRegistry) Get(name string) Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// List returns all registered tools.
func (r *ToolRegistry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	return out
}

// Execute runs a ToolCall by looking up the tool and invoking it.
func (r *ToolRegistry) Execute(call ToolCall) ToolResult {
	t := r.Get(call.Name)
	if t == nil {
		return ToolResult{
			ToolCallID: call.ID,
			Success:    false,
			Error:      fmt.Sprintf("unknown tool: %s", call.Name),
		}
	}
	result := t.Execute(call.Args)
	result.ToolCallID = call.ID
	return result
}
