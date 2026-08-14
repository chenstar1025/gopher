// Gopher — a coding agent harness built in Go.
// CLI entry point (SPEC §3.8).
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/chenstar1025/gopher/internal/config"
	"github.com/chenstar1025/gopher/internal/credential"
	"github.com/chenstar1025/gopher/internal/feedback"
	"github.com/chenstar1025/gopher/internal/guard"
	"github.com/chenstar1025/gopher/internal/llm"
	"github.com/chenstar1025/gopher/internal/loop"
	"github.com/chenstar1025/gopher/internal/memory"
	"github.com/chenstar1025/gopher/internal/tools"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: gopher <command> [args]")
		fmt.Fprintln(os.Stderr, "Commands: run, status, config")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		runCmd()
	case "status":
		statusCmd()
	case "config":
		configCmd()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runCmd() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: gopher run <task>")
		os.Exit(1)
	}
	task := strings.Join(os.Args[2:], " ")

	// Load config
	cfg, _ := config.Load(".gopher/config.json")
	cfg = config.MergeEnv(cfg)

	// Get API key
	credMgr := credential.NewManager()
	apiKey, err := credMgr.Retrieve()
	if err != nil {
		fmt.Fprintln(os.Stderr, "No API key found.")
		fmt.Fprint(os.Stderr, "Enter your LLM API key: ")
		fmt.Scanln(&apiKey)
		if apiKey == "" {
			fmt.Fprintln(os.Stderr, "API key is required.")
			os.Exit(1)
		}
		if err := credMgr.Store(apiKey); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not save key: %v\n", err)
		} else {
			fmt.Fprintln(os.Stderr, "Key saved.")
		}
	}

	// Wire components
	l := llm.NewOpenAI(cfg.LLMEndpoint, apiKey, cfg.LLMModel)
	reg := tools.NewRegistry()
	reg.Register(tools.ReadFileTool{})
	reg.Register(tools.WriteFileTool{})
	reg.Register(tools.ListDirTool{})
	reg.Register(tools.ShellTool{})
	reg.Register(tools.TestTool{})

	g := guard.NewDefault()
	tr := feedback.NewTracker(cfg.MaxRetries)
	mem, _ := memory.Load(".")

	agent := loop.New(l, reg, g, tr, mem, cfg, os.Stdin)

	fmt.Printf("Gopher starting: %s\n", task)
	sess, err := agent.Run(context.Background(), task)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
	if sess != nil {
		fmt.Printf("Session %s finished: %s (%d rounds)\n", sess.ID, sess.Status, sess.Round)
		// Print the last assistant response
		for i := len(sess.Messages) - 1; i >= 0; i-- {
			if sess.Messages[i].Role == "assistant" && sess.Messages[i].Content != "" {
				fmt.Println()
				fmt.Println(sess.Messages[i].Content)
				break
			}
		}
	}
}

func statusCmd() {
	credMgr := credential.NewManager()
	fmt.Println(credMgr.Status())
}

func configCmd() {
	credMgr := credential.NewManager()

	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: gopher config <set-key|clear-key>")
		os.Exit(1)
	}

	switch os.Args[2] {
	case "set-key":
		fmt.Fprint(os.Stderr, "Enter your LLM API key: ")
		var key string
		fmt.Scanln(&key)
		if key == "" {
			fmt.Fprintln(os.Stderr, "Key cannot be empty.")
			os.Exit(1)
		}
		if err := credMgr.Store(key); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save key: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Key saved.")

	case "clear-key":
		if err := credMgr.Delete(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to clear key: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Key cleared.")

	default:
		fmt.Fprintf(os.Stderr, "Unknown config command: %s\n", os.Args[2])
		os.Exit(1)
	}
}
