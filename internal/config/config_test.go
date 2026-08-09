package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()
	if cfg.LLMEndpoint != "https://api.openai.com/v1" {
		t.Errorf("unexpected endpoint: %s", cfg.LLMEndpoint)
	}
	if cfg.LLMModel != "gpt-4o" {
		t.Errorf("unexpected model: %s", cfg.LLMModel)
	}
	if cfg.MaxRounds != 50 {
		t.Errorf("unexpected max_rounds: %d", cfg.MaxRounds)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("unexpected max_retries: %d", cfg.MaxRetries)
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := `{"llm_model":"gpt-4o-mini","max_rounds":20}`

	os.WriteFile(path, []byte(data), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LLMModel != "gpt-4o-mini" {
		t.Errorf("expected gpt-4o-mini, got %s", cfg.LLMModel)
	}
	if cfg.MaxRounds != 20 {
		t.Errorf("expected 20, got %d", cfg.MaxRounds)
	}
	// Default should still apply for unspecified fields
	if cfg.MaxRetries != 3 {
		t.Errorf("expected default 3, got %d", cfg.MaxRetries)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/config.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	// Should still get defaults
	if cfg.MaxRounds != 50 {
		t.Errorf("expected default 50, got %d", cfg.MaxRounds)
	}
}

func TestMergeEnv(t *testing.T) {
	cfg := Defaults()

	os.Setenv("GOPHER_LLM_ENDPOINT", "https://custom.api/v1")
	defer os.Unsetenv("GOPHER_LLM_ENDPOINT")

	os.Setenv("GOPHER_LLM_MODEL", "deepseek-v3")
	defer os.Unsetenv("GOPHER_LLM_MODEL")

	cfg = MergeEnv(cfg)
	if cfg.LLMEndpoint != "https://custom.api/v1" {
		t.Errorf("expected custom endpoint: %s", cfg.LLMEndpoint)
	}
	if cfg.LLMModel != "deepseek-v3" {
		t.Errorf("expected deepseek-v3: %s", cfg.LLMModel)
	}
}

func TestMergeEnv_NoOverrides(t *testing.T) {
	cfg := Defaults()
	cfg2 := MergeEnv(cfg)
	if cfg.LLMEndpoint != cfg2.LLMEndpoint {
		t.Error("expected no change without env vars")
	}
}
