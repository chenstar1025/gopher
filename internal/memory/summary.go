package memory

import (
	"os"
	"path/filepath"
	"strings"
)

// LoadRules reads the project rules file and returns it as a system prompt.
func LoadRules(root string) string {
	path := filepath.Join(root, ".gopher", "rules.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// SummarizeMessages creates a compressed summary from old messages.
// This is a deterministic, non-LLM implementation: it extracts the first
// line of each message, truncating to maxTokens bytes. At least one
// message is always included.
func SummarizeMessages(messages []string, maxTokens int) string {
	if len(messages) == 0 {
		return ""
	}
	var lines []string
	used := 0
	for i, msg := range messages {
		firstLine := strings.SplitN(msg, "\n", 2)[0]
		// Always include the first message; truncate subsequent ones
		if i > 0 && used+len(firstLine) > maxTokens {
			lines = append(lines, "(truncated)")
			break
		}
		lines = append(lines, firstLine)
		used += len(firstLine)
	}
	return strings.Join(lines, "\n")
}
