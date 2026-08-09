package tools

import (
	"os"
	"path/filepath"
	"strings"
)

// ReadFileTool reads a file and returns its contents.
type ReadFileTool struct{}

func (ReadFileTool) Name() string             { return "read_file" }
func (ReadFileTool) Description() string       { return "Read the contents of a file" }
func (ReadFileTool) PermissionLevel() PermissionLevel { return PermSafe }
func (ReadFileTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the file to read",
			},
		},
		"required": []string{"path"},
	}
}

func (ReadFileTool) Execute(args map[string]any) ToolResult {
	path, _ := args["path"].(string)
	if path == "" {
		return ToolResult{Success: false, Error: "path is required"}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()}
	}
	return ToolResult{Success: true, Output: string(data)}
}

// WriteFileTool writes content to a file, creating it if necessary.
type WriteFileTool struct{}

func (WriteFileTool) Name() string             { return "write_file" }
func (WriteFileTool) Description() string       { return "Write content to a file, overwriting if it exists" }
func (WriteFileTool) PermissionLevel() PermissionLevel { return PermSafe }
func (WriteFileTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the file to write",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Content to write to the file",
			},
		},
		"required": []string{"path", "content"},
	}
}

func (WriteFileTool) Execute(args map[string]any) ToolResult {
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)
	if path == "" {
		return ToolResult{Success: false, Error: "path is required"}
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ToolResult{Success: false, Error: err.Error()}
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return ToolResult{Success: false, Error: err.Error()}
	}
	return ToolResult{Success: true, Output: "ok"}
}

// ListDirTool lists the contents of a directory.
type ListDirTool struct{}

func (ListDirTool) Name() string             { return "list_dir" }
func (ListDirTool) Description() string       { return "List files and directories at a given path" }
func (ListDirTool) PermissionLevel() PermissionLevel { return PermSafe }
func (ListDirTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the directory to list",
			},
		},
		"required": []string{"path"},
	}
}

func (ListDirTool) Execute(args map[string]any) ToolResult {
	path, _ := args["path"].(string)
	if path == "" {
		path = "."
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()}
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return ToolResult{Success: true, Output: strings.Join(names, "\n")}
}
