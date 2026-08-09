package tools

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
)

// ShellTool executes arbitrary shell commands. Marked dangerous because it
// can perform arbitrary system modifications.
type ShellTool struct{}

func (ShellTool) Name() string                       { return "run_shell" }
func (ShellTool) Description() string                 { return "Execute a shell command. Marked dangerous — requires guardrail approval." }
func (ShellTool) PermissionLevel() PermissionLevel    { return PermDangerous }
func (ShellTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The shell command to execute",
			},
		},
		"required": []string{"command"},
	}
}

func (ShellTool) Execute(args map[string]any) ToolResult {
	command, _ := args["command"].(string)
	if command == "" {
		return ToolResult{Success: false, Error: "command is required"}
	}
	cmd := exec.Command("sh", "-c", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	output := strings.TrimSpace(stdout.String())
	if errStr := strings.TrimSpace(stderr.String()); errStr != "" {
		if output != "" {
			output += "\n"
		}
		output += errStr
	}
	if err != nil {
		return ToolResult{Success: false, Output: output, Error: err.Error()}
	}
	return ToolResult{Success: true, Output: output}
}

// TestTool runs go test and returns structured JSON output for the feedback loop.
type TestTool struct{}

func (TestTool) Name() string                    { return "run_test" }
func (TestTool) Description() string              { return "Run go test with -json output for the feedback loop to parse" }
func (TestTool) PermissionLevel() PermissionLevel { return PermSafe }
func (TestTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pkg": map[string]any{
				"type":        "string",
				"description": "Package pattern to test (default ./...)",
			},
		},
	}
}

func (TestTool) Execute(args map[string]any) ToolResult {
	pkg, _ := args["pkg"].(string)
	if pkg == "" {
		pkg = "./..."
	}
	cmd := exec.Command("go", "test", "-json", pkg)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	output := strings.TrimSpace(stdout.String())
	if errStr := strings.TrimSpace(stderr.String()); errStr != "" {
		if output != "" {
			output += "\n"
		}
		output += errStr
	}
	// run_test reports success even if tests fail — the feedback loop
	// interprets the JSON output, not the exit code. Only report failure
	// if the test command itself couldn't run (e.g. no go binary).
	if err != nil {
		// Check if it was just test failures (exit code 1) vs command error
		if _, ok := err.(*exec.ExitError); ok {
			return ToolResult{Success: true, Output: output}
		}
		return ToolResult{Success: false, Output: output, Error: err.Error()}
	}
	return ToolResult{Success: true, Output: output}
}

// Ensure json is used (for future use by feedback parser)
var _ = json.Valid
