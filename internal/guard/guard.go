// Package guard implements the governance guardrail (SPEC §3.6): it matches
// dangerous shell commands against a deterministic pattern list and pauses for
// human confirmation before they are allowed to run.
package guard

import (
	"bufio"
	"io"
	"os"
	"strings"
	"time"

	"github.com/chenstar1025/gopher/internal/tools"
)

// DefaultConfirmTimeout bounds how long Confirm waits for a human answer
// before rejecting the action (SPEC §3.6: 60 seconds).
const DefaultConfirmTimeout = 60 * time.Second

// Guardrail intercepts dangerous tool calls and asks a human to confirm them.
type Guardrail struct {
	// reader supplies the confirmation input (normally os.Stdin).
	reader io.Reader
	// timeout bounds how long Confirm waits for an answer.
	timeout time.Duration
	// whitelist holds trusted commands that bypass the guard entirely.
	whitelist []string
}

// New builds a Guardrail that reads confirmation input from r, waits at most
// timeout for an answer (a zero timeout means DefaultConfirmTimeout) and
// trusts the whitelisted commands.
func New(r io.Reader, timeout time.Duration, whitelist []string) *Guardrail {
	return &Guardrail{reader: r, timeout: timeout, whitelist: whitelist}
}

// NewDefault returns a Guardrail wired to os.Stdin with the default 60-second
// confirmation timeout and no whitelist.
func NewDefault() *Guardrail {
	return New(os.Stdin, DefaultConfirmTimeout, nil)
}

// SetWhitelist replaces the set of trusted commands.
func (g *Guardrail) SetWhitelist(cmds []string) {
	g.whitelist = cmds
}

// Check reports whether action is dangerous. It returns blocked=true together
// with a human-readable reason when the underlying shell command matches one
// of the dangerous patterns in SPEC §3.6. Commands on the whitelist are always
// allowed. Safe commands and non-shell tool calls return blocked=false.
func (g *Guardrail) Check(action tools.ToolCall) (blocked bool, reason string) {
	cmd := commandFromCall(action)
	if cmd == "" {
		return false, ""
	}
	if g.isWhitelisted(cmd) {
		return false, ""
	}
	for _, p := range dangerousPatterns {
		if p.re.MatchString(cmd) {
			return true, p.reason
		}
	}
	return false, ""
}

// Confirm pauses execution and waits for a yes/no answer on the guardrail's
// input. It returns true only when the human answers "yes" (or "y") before the
// timeout. A "no" answer, an empty input or a timeout returns false, in which
// case the action must be rejected (SPEC §3.6).
func (g *Guardrail) Confirm(_ tools.ToolCall) bool {
	if g.reader == nil {
		return false
	}

	result := make(chan string, 1)
	go func() {
		line, err := readLine(g.reader)
		if err != nil && line == "" {
			result <- ""
			return
		}
		result <- line
	}()

	timeout := g.timeout
	if timeout <= 0 {
		timeout = DefaultConfirmTimeout
	}

	select {
	case line := <-result:
		return isYes(line)
	case <-time.After(timeout):
		return false
	}
}

// commandFromCall extracts the shell command from a tool call. It prefers the
// conventional argument keys; for the run_shell tool it falls back to
// concatenating all string arguments so no shell payload is missed.
func commandFromCall(action tools.ToolCall) string {
	for _, key := range []string{"command", "cmd", "script", "shell", "input"} {
		if v, ok := action.Args[key]; ok {
			if s, ok := v.(string); ok {
				if strings.TrimSpace(s) != "" {
					return s
				}
			}
		}
	}
	if action.Name == "run_shell" {
		var b strings.Builder
		for k, v := range action.Args {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				if b.Len() > 0 {
					b.WriteByte(' ')
				}
				b.WriteString(k)
				b.WriteByte('=')
				b.WriteString(s)
			}
		}
		return b.String()
	}
	return ""
}

// isWhitelisted reports whether cmd is covered by the whitelist. An entry
// matches the exact command or any command that starts with it, so whitelisting
// "rm" allows every rm invocation while "rm -rf /tmp/x" allows only that exact
// command.
func (g *Guardrail) isWhitelisted(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return false
	}
	for _, wl := range g.whitelist {
		wl = strings.TrimSpace(wl)
		if wl == "" {
			continue
		}
		if cmd == wl || strings.HasPrefix(cmd, wl+" ") || strings.HasPrefix(cmd, wl+"\t") {
			return true
		}
	}
	return false
}

// readLine reads a single line from r.
func readLine(r io.Reader) (string, error) {
	return bufio.NewReader(r).ReadString('\n')
}

// isYes reports whether the trimmed, lower-cased answer is an affirmative.
func isYes(answer string) bool {
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "yes", "y":
		return true
	default:
		return false
	}
}
