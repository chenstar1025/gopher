package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	reg.Register(ReadFileTool{})

	tool := reg.Get("read_file")
	if tool == nil {
		t.Fatal("expected to find read_file")
	}
	if tool.Name() != "read_file" {
		t.Errorf("expected read_file, got %s", tool.Name())
	}
}

func TestRegistry_UnknownTool(t *testing.T) {
	reg := NewRegistry()
	result := reg.Execute(ToolCall{ID: "1", Name: "nonexistent"})
	if result.Success {
		t.Error("expected failure for unknown tool")
	}
	if !strings.Contains(result.Error, "unknown") {
		t.Errorf("expected 'unknown' in error, got %s", result.Error)
	}
}

func TestRegistry_Execute(t *testing.T) {
	reg := NewRegistry()
	reg.Register(ReadFileTool{})

	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	os.WriteFile(path, []byte("hello"), 0644)

	result := reg.Execute(ToolCall{ID: "call_1", Name: "read_file", Args: map[string]any{"path": path}})
	if !result.Success {
		t.Fatalf("expected success: %s", result.Error)
	}
	if result.Output != "hello" {
		t.Errorf("expected 'hello', got %s", result.Output)
	}
	if result.ToolCallID != "call_1" {
		t.Errorf("expected call_1, got %s", result.ToolCallID)
	}
}

func TestRegistry_List(t *testing.T) {
	reg := NewRegistry()
	reg.Register(ReadFileTool{})
	reg.Register(WriteFileTool{})

	list := reg.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(list))
	}
}

func TestReadFile_Success(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "data.txt")
	os.WriteFile(path, []byte("content"), 0644)

	tool := ReadFileTool{}
	result := tool.Execute(map[string]any{"path": path})
	if !result.Success {
		t.Fatalf("expected success: %s", result.Error)
	}
	if result.Output != "content" {
		t.Errorf("expected 'content', got '%s'", result.Output)
	}
}

func TestReadFile_Missing(t *testing.T) {
	tool := ReadFileTool{}
	result := tool.Execute(map[string]any{"path": "/nonexistent/file.xyz"})
	if result.Success {
		t.Fatal("expected failure for missing file")
	}
}

func TestReadFile_NoPath(t *testing.T) {
	tool := ReadFileTool{}
	result := tool.Execute(map[string]any{})
	if result.Success {
		t.Fatal("expected failure for missing path")
	}
}

func TestWriteFile_Success(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "out.txt")

	tool := WriteFileTool{}
	result := tool.Execute(map[string]any{"path": path, "content": "written"})
	if !result.Success {
		t.Fatalf("expected success: %s", result.Error)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "written" {
		t.Errorf("expected 'written', got '%s'", string(data))
	}
}

func TestWriteFile_CreatesParentDir(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "sub", "nested", "file.txt")

	tool := WriteFileTool{}
	result := tool.Execute(map[string]any{"path": path, "content": "deep"})
	if !result.Success {
		t.Fatalf("expected success: %s", result.Error)
	}
}

func TestListDir_Success(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(tmp, "b.txt"), []byte("b"), 0644)

	tool := ListDirTool{}
	result := tool.Execute(map[string]any{"path": tmp})
	if !result.Success {
		t.Fatalf("expected success: %s", result.Error)
	}
	if !strings.Contains(result.Output, "a.txt") || !strings.Contains(result.Output, "b.txt") {
		t.Errorf("expected a.txt and b.txt in output: %s", result.Output)
	}
}

func TestShellTool_Success(t *testing.T) {
	tool := ShellTool{}
	result := tool.Execute(map[string]any{"command": "echo hello"})
	if !result.Success {
		t.Fatalf("expected success: %s", result.Error)
	}
	if !strings.Contains(result.Output, "hello") {
		t.Errorf("expected 'hello' in output: %s", result.Output)
	}
}

func TestShellTool_NoCommand(t *testing.T) {
	tool := ShellTool{}
	result := tool.Execute(map[string]any{})
	if result.Success {
		t.Fatal("expected failure for missing command")
	}
}

func TestShellTool_PermissionLevel(t *testing.T) {
	tool := ShellTool{}
	if tool.PermissionLevel() != PermDangerous {
		t.Errorf("shell should be dangerous, got %s", tool.PermissionLevel())
	}
}

func TestTestTool_PermissionLevel(t *testing.T) {
	tool := TestTool{}
	if tool.PermissionLevel() != PermSafe {
		t.Errorf("test tool should be safe, got %s", tool.PermissionLevel())
	}
}

func TestToolName_AllUnique(t *testing.T) {
	reg := NewRegistry()
	reg.Register(ReadFileTool{})
	reg.Register(WriteFileTool{})
	reg.Register(ListDirTool{})
	reg.Register(ShellTool{})
	reg.Register(TestTool{})

	seen := map[string]bool{}
	for _, tool := range reg.List() {
		if seen[tool.Name()] {
			t.Errorf("duplicate tool name: %s", tool.Name())
		}
		seen[tool.Name()] = true
	}
	if len(reg.List()) != 5 {
		t.Errorf("expected 5 tools, got %d", len(reg.List()))
	}
}
