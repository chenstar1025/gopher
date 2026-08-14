// Package config handles declarative configuration (SPEC §3.5, §6.5).
package config

import (
	"encoding/json"
	"os"
)

// Config holds all user-facing configuration for a Gopher session.
type Config struct {
	LLMEndpoint   string   `json:"llm_endpoint"`
	LLMModel      string   `json:"llm_model"`
	MaxRounds     int      `json:"max_rounds"`
	MaxRetries    int      `json:"max_retries"`
	WorkDir       string   `json:"work_dir"`
	WhitelistCmds []string `json:"whitelist_cmds"`
}

// Defaults returns a Config populated with safe defaults.
func Defaults() Config {
	return Config{
		LLMEndpoint:   "https://api.openai.com",
		LLMModel:      "gpt-4o",
		MaxRounds:     50,
		MaxRetries:    3,
		WhitelistCmds: nil,
	}
}

// Load reads a JSON config file and merges it over defaults.
func Load(path string) (Config, error) {
	cfg := Defaults()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// MergeEnv overrides config fields from environment variables.
// Supported: GOPHER_LLM_ENDPOINT, GOPHER_LLM_MODEL.
func MergeEnv(cfg Config) Config {
	if v := os.Getenv("GOPHER_LLM_ENDPOINT"); v != "" {
		cfg.LLMEndpoint = v
	}
	if v := os.Getenv("GOPHER_LLM_MODEL"); v != "" {
		cfg.LLMModel = v
	}
	return cfg
}
