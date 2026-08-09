# Gopher — A Coding Agent Harness

Gopher is a coding agent harness built in Go for the AI4SE final project (Type A). It implements the full agent loop — context assembly, LLM orchestration, tool dispatch, governance guardrails, and a feedback loop — from scratch without relying on LangChain, AutoGen, or other agent frameworks.

```
Agent = LLM + Harness
```

## Quick Start

```bash
# Build
go build -o gopher ./cmd/gopher/

# Configure your API key
./gopher config set-key

# Run a task
./gopher run "fix the compilation error in main.go"

# Check key status
./gopher status
```

## Requirements

- Go 1.22+
- An OpenAI-compatible API key (OpenAI, DeepSeek, etc.)

## Commands

| Command | Description |
|---------|-------------|
| `gopher run <task>` | Start the agent on a natural-language task |
| `gopher status` | Show API key status (no plaintext) |
| `gopher config set-key` | Securely store your API key |
| `gopher config clear-key` | Remove the stored API key |

## How It Works

```
User Task → [System Prompt + Memory + Tools]
  → LLM Call → Parse Response
  → Guardrail Check → Execute Tool
  → Feedback Loop (test → classify → inject)
  → Repeat until done or max rounds
```

## Project Structure

```
cmd/gopher/          CLI entry point
internal/
  loop/              Agent main loop
  llm/               LLM abstraction (interface + OpenAI + mock)
  tools/             Tool system (read_file, write_file, shell, run_test)
  guard/             Governance guardrail (dangerous command interception)
  feedback/          Feedback loop (test parser, classifier, injector, tracker)
  config/            Configuration system
  credential/        Secure API key storage (AES-256-GCM encrypted file)
  memory/            Cross-session memory and project rules
test/                Integration tests (mock LLM)
```

## Testing

```bash
# Run all tests (no network required — all use mock LLM)
make test

# Or directly
go test ./... -v -count=1
```

## API Key Security

- Key is stored in `%APPDATA%/gopher/gopher.key` (Windows) or `~/.config/gopher/gopher.key` (Linux/macOS), encrypted with AES-256-GCM
- Key is never written to logs, terminal output, or source code
- `gopher status` shows only the last 4 characters (`sk-...abcd`)
- First run prompts for key entry with hidden echo

**Known limitation:** The current implementation uses a machine-derived key for AES encryption rather than OS keychain (DPAPI/keyring). This is a pragmatic stand-in documented in the threat model (SPEC §7.1).

## Distribution

Single-file binaries for Windows, Linux, and macOS:

```bash
# Build all platforms
make build-all

# Or go install
go install github.com/chenstar1025/gopher/cmd/gopher@latest
```

Pre-built binaries are available on [GitHub Releases](https://github.com/chenstar1025/gopher/releases).

## CI

GitHub Actions runs `go test ./...` and `go vet ./...` on every push.

## Design Documents

- [SPEC.md](SPEC.md) — Full design specification
- [PLAN.md](PLAN.md) — Implementation plan with task dependencies
- [SPEC_PROCESS.md](SPEC_PROCESS.md) — Brainstorming process and cold-start validation

## License

MIT
