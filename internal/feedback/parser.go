package feedback

import (
	"encoding/json"
	"strings"
)

// ParseTestOutput parses go test -json (newline-delimited JSON) into TestEvents.
func ParseTestOutput(jsonl string) ([]TestEvent, error) {
	lines := strings.Split(strings.TrimSpace(jsonl), "\n")
	var events []TestEvent
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var ev TestEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			// Skip non-JSON lines (e.g. "FAIL", "ok", compiler output)
			continue
		}
		events = append(events, ev)
	}
	return events, nil
}
