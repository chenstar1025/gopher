package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStore_AddAndRead(t *testing.T) {
	dir := t.TempDir()
	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	s.Add(Entry{Role: "user", Content: "hello"})
	s.Add(Entry{Role: "assistant", Content: "hi there"})

	// Reload from disk
	s2, err := Load(dir)
	if err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if len(s2.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(s2.Entries))
	}
	if s2.Entries[0].Content != "hello" {
		t.Errorf("unexpected content: %s", s2.Entries[0].Content)
	}
}

func TestStore_Clear(t *testing.T) {
	dir := t.TempDir()
	s, _ := Load(dir)
	s.Add(Entry{Role: "user", Content: "data"})
	s.Clear()

	if len(s.Entries) != 0 {
		t.Error("expected empty after clear")
	}

	s2, _ := Load(dir)
	if len(s2.Entries) != 0 {
		t.Error("expected empty on disk after clear")
	}
}

func TestLoadRules_Exists(t *testing.T) {
	dir := t.TempDir()
	gopherDir := filepath.Join(dir, ".gopher")
	os.MkdirAll(gopherDir, 0755)
	os.WriteFile(filepath.Join(gopherDir, "rules.md"), []byte("# Project Rules\nUse TDD."), 0644)

	rules := LoadRules(dir)
	if rules != "# Project Rules\nUse TDD." {
		t.Errorf("unexpected rules: %s", rules)
	}
}

func TestLoadRules_Missing(t *testing.T) {
	dir := t.TempDir()
	rules := LoadRules(dir)
	if rules != "" {
		t.Errorf("expected empty, got %s", rules)
	}
}

func TestSummarizeMessages(t *testing.T) {
	msgs := []string{
		"line one\nmore detail here",
		"line two\nmore detail here",
		"line three\nmore detail here",
	}
	result := SummarizeMessages(msgs, 50)
	if result != "line one\nline two\nline three" {
		t.Errorf("unexpected summary: %s", result)
	}
}

func TestSummarizeMessages_Truncation(t *testing.T) {
	msgs := []string{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	result := SummarizeMessages(msgs, 20)
	// First message always included; second is truncated
	if result != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n(truncated)" {
		t.Errorf("expected truncation after first msg: %s", result)
	}
}
