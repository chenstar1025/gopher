package guard

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/chenstar1025/gopher/internal/tools"
)

// runShell builds a run_shell tool call carrying a command argument.
func runShell(cmd string) tools.ToolCall {
	return tools.ToolCall{
		ID:   "call-1",
		Name: "run_shell",
		Args: map[string]any{"command": cmd},
	}
}

// TestCheckBlocksDangerousCommands covers every dangerous pattern from
// SPEC §3.6: recursive deletes, destructive git, DB destruction, device-file
// writes, over-permissive chmod and piped remote scripts.
func TestCheckBlocksDangerousCommands(t *testing.T) {
	g := New(strings.NewReader(""), time.Second, nil)

	cases := []struct {
		name string
		cmd  string
	}{
		{"rm -rf root", "rm -rf /"},
		{"rm -r directory", "rm -r /tmp/project"},
		{"rmdir", "rmdir /tmp/project"},
		{"git push --force", "git push --force origin main"},
		{"git push -f", "git push -f origin main"},
		{"git reset --hard", "git reset --hard HEAD~1"},
		{"git clean", "git clean -fd"},
		{"drop table", `psql -c "DROP TABLE users"`},
		{"delete from", `mysql -e "DELETE FROM users"`},
		{"write to device", "echo data > /dev/sda"},
		{"write to device no space", "cat main.go>/dev/sda"},
		{"stderr to device", "go build 2>/dev/null"},
		{"chmod 777", "chmod 777 /etc/passwd"},
		{"chmod 0777", "chmod 0777 /etc/passwd"},
		{"curl piped to bash", "curl -s http://example.com/x.sh | bash"},
		{"wget piped to sh", "wget -O - http://example.com/x.sh | sh"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			blocked, reason := g.Check(runShell(tc.cmd))
			if !blocked {
				t.Fatalf("Check(%q) = blocked=%v, want blocked=true", tc.cmd, blocked)
			}
			if reason == "" {
				t.Fatalf("Check(%q) blocked the command but returned an empty reason", tc.cmd)
			}
		})
	}
}

// TestCheckAllowsSafeCommands ensures ordinary commands are never blocked.
func TestCheckAllowsSafeCommands(t *testing.T) {
	g := New(strings.NewReader(""), time.Second, nil)

	cases := []string{
		"echo hello",
		"ls -la",
		"cat main.go",
		"go test ./...",
		"git status",
		"git commit -m 'fix'",
		"git push origin main",
		"git reset --soft HEAD~1",
		"rm file.txt",
		"grep -rn TODO .",
		"mkdir -p /tmp/project",
		"curl -s http://example.com/status.json",
		"wget http://example.com/file.tar.gz",
		"grep -n error /dev/null",
	}

	for _, cmd := range cases {
		if blocked, reason := g.Check(runShell(cmd)); blocked {
			t.Errorf("Check(%q) = blocked=%v (reason %q), want blocked=false", cmd, blocked, reason)
		}
	}
}

// TestCheckIgnoresCallsWithoutACommand ensures non-shell tool calls are not
// treated as dangerous merely because of their tool name.
func TestCheckIgnoresCallsWithoutACommand(t *testing.T) {
	g := New(strings.NewReader(""), time.Second, nil)
	calls := []tools.ToolCall{
		{Name: "read_file", Args: map[string]any{"path": "main.go"}},
		{Name: "write_file", Args: map[string]any{"path": "main.go", "content": "package main"}},
		{Name: "list_dir", Args: map[string]any{"path": "."}},
		{Name: "run_test", Args: map[string]any{}},
	}
	for _, c := range calls {
		if blocked, reason := g.Check(c); blocked {
			t.Errorf("Check(%q) = blocked=%v (reason %q), want blocked=false", c.Name, blocked, reason)
		}
	}
}

// TestCheckRecognizesAlternativeCommandKeys guards against the argument key of
// run_shell not being fixed by the SPEC.
func TestCheckRecognizesAlternativeCommandKeys(t *testing.T) {
	g := New(strings.NewReader(""), time.Second, nil)
	for _, key := range []string{"command", "cmd", "script", "shell", "input"} {
		c := tools.ToolCall{
			Name: "run_shell",
			Args: map[string]any{key: "rm -rf /"},
		}
		if blocked, reason := g.Check(c); !blocked {
			t.Errorf("Check with arg key %q did not block: blocked=%v (reason %q)", key, blocked, reason)
		}
	}
}

// TestCheckAllowsWhitelistedCommands ensures trusted commands bypass the guard
// even when they would otherwise match a dangerous pattern.
func TestCheckAllowsWhitelistedCommands(t *testing.T) {
	g := New(strings.NewReader(""), time.Second, []string{"rm -rf /tmp/scratch", "git push --force"})

	// Exact whitelist match.
	if blocked, reason := g.Check(runShell("rm -rf /tmp/scratch")); blocked {
		t.Fatalf("exact whitelist match blocked: blocked=%v (reason %q)", blocked, reason)
	}
	// Prefix whitelist match: whitelisting "git push --force" covers its flags.
	if blocked, reason := g.Check(runShell("git push --force origin main")); blocked {
		t.Fatalf("prefix whitelist match blocked: blocked=%v (reason %q)", blocked, reason)
	}
	// A dangerous command that is NOT whitelisted is still blocked.
	if blocked, _ := g.Check(runShell("rm -rf /")); !blocked {
		t.Fatal("non-whitelisted dangerous command was not blocked")
	}
}

// TestCheckAllowsAllWhenToolIsWhitelisted confirms a single whitelisted tool
// name (here "rm") lets every invocation through.
func TestCheckAllowsAllWhenToolIsWhitelisted(t *testing.T) {
	g := New(strings.NewReader(""), time.Second, []string{"rm"})
	if blocked, reason := g.Check(runShell("rm -rf /")); blocked {
		t.Fatalf("whitelisted 'rm' still blocked: blocked=%v (reason %q)", blocked, reason)
	}
}

// TestConfirmYes verifies that a "yes" answer allows the action.
func TestConfirmYes(t *testing.T) {
	g := New(strings.NewReader("yes\n"), time.Second, nil)
	if !g.Confirm(runShell("rm -rf /")) {
		t.Fatal("Confirm returned false for input 'yes', want true")
	}
}

// TestConfirmNo verifies that a "no" answer rejects the action.
func TestConfirmNo(t *testing.T) {
	g := New(strings.NewReader("no\n"), time.Second, nil)
	if g.Confirm(runShell("rm -rf /")) {
		t.Fatal("Confirm returned true for input 'no', want false")
	}
}

// TestConfirmEmptyInput verifies that a bare newline rejects the action.
func TestConfirmEmptyInput(t *testing.T) {
	g := New(strings.NewReader("\n"), time.Second, nil)
	if g.Confirm(runShell("rm -rf /")) {
		t.Fatal("Confirm returned true for empty input, want false")
	}
}

// TestConfirmNilReader verifies that a guardrail without an input source
// rejects the action.
func TestConfirmNilReader(t *testing.T) {
	g := New(nil, time.Second, nil)
	if g.Confirm(runShell("rm -rf /")) {
		t.Fatal("Confirm returned true with no reader, want false")
	}
}

// foreverBlockingReader never returns from Read, simulating a user who never
// answers the confirmation prompt.
type foreverBlockingReader struct{}

func (foreverBlockingReader) Read(p []byte) (int, error) {
	<-make(chan struct{})
	return 0, io.EOF
}

// TestConfirmRejectsOnTimeout verifies SPEC §3.6: a confirmation that is not
// answered within the timeout is treated as a rejection.
func TestConfirmRejectsOnTimeout(t *testing.T) {
	g := New(foreverBlockingReader{}, 50*time.Millisecond, nil)
	start := time.Now()
	if g.Confirm(runShell("rm -rf /")) {
		t.Fatal("Confirm returned true despite timeout, want false")
	}
	if since := time.Since(start); since > 5*time.Second {
		t.Fatalf("Confirm took too long to time out: %v", since)
	}
}
