package feedback

import "github.com/chenstar1025/gopher/internal/llm"

// Inject converts a Feedback into a tool-result message suitable for
// injection into the LLM conversation.
func Inject(fb Feedback) llm.Message {
	return llm.Message{
		Role:    "tool",
		Content: BuildSummary(fb),
	}
}

// MakeFeedback runs the full pipeline: parse → classify → extract → summarize.
// Returns a Feedback ready for Inject.
func MakeFeedback(jsonl string) Feedback {
	events, err := ParseTestOutput(jsonl)
	if err != nil || len(events) == 0 {
		return Feedback{Type: CompileError, Summary: "No test output produced. The code may not compile."}
	}
	fb := Feedback{Type: Classify(events)}
	if fb.Type == TestFailure {
		fb.Failures = ExtractFailures(events)
	}
	fb.Summary = BuildSummary(fb)
	return fb
}
