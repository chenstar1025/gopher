// Package feedback implements the feedback loop — Gopher's primary deep dimension
// (SPEC §3.5, §11.2). It parses go test -json output into structured feedback,
// classifies failures, injects feedback into the LLM context, and tracks
// retry rounds to prevent infinite loops. All mechanisms are deterministic
// code, not prompts — verifiable with mock LLM tests.
package feedback

// FeedbackType classifies the outcome of a test run.
type FeedbackType string

const (
	Success       FeedbackType = "success"
	CompileError  FeedbackType = "compile_error"
	TestFailure   FeedbackType = "test_failure"
	Timeout       FeedbackType = "timeout"
	Panic         FeedbackType = "panic"
)

// TestEvent is a single line from go test -json output.
type TestEvent struct {
	Time    string `json:"Time"`
	Action  string `json:"Action"`  // run | pause | cont | pass | fail | output | skip
	Package string `json:"Package"`
	Test    string `json:"Test"`
	Output  string `json:"Output"`
	Elapsed float64 `json:"Elapsed"`
}

// Failure captures a single failing test with location info.
type Failure struct {
	TestName string
	Package  string
	Message  string
}

// Feedback is the structured result of a test run, ready for injection
// into the LLM conversation.
type Feedback struct {
	Type     FeedbackType
	Failures []Failure
	Summary  string
}

// Status tracks retry state for a single task.
type Status struct {
	TaskID    string
	Round     int
	MaxRounds int
}
