package feedback

import "strings"

// Classify examines test events and returns the FeedbackType.
// Priority: compile_error (highest) > panic > timeout > test_failure > success.
func Classify(events []TestEvent) FeedbackType {
	// First pass: check for compile errors (absolute priority — nothing else matters)
	for _, ev := range events {
		if ev.Action == "output" && (strings.Contains(ev.Output, "build failed") || strings.Contains(ev.Output, "setup failed")) {
			return CompileError
		}
	}

	// Second pass: check for panic, timeout, and test failures
	hasPanic := false
	hasTimeout := false
	hasFailed := false
	for _, ev := range events {
		if ev.Action == "output" && strings.Contains(ev.Output, "panic:") {
			hasPanic = true
		}
		if ev.Action == "fail" && strings.Contains(ev.Output, "test timed out") {
			hasTimeout = true
		}
		if ev.Action == "output" && strings.Contains(ev.Output, "*** Test killed") {
			hasTimeout = true
		}
		if ev.Action == "fail" {
			hasFailed = true
		}
	}

	if hasPanic {
		return Panic
	}
	if hasTimeout {
		return Timeout
	}
	if hasFailed {
		return TestFailure
	}
	return Success
}

// ExtractFailures pulls individual test failure details from events.
func ExtractFailures(events []TestEvent) []Failure {
	var failures []Failure
	outputByTest := map[string][]string{}

	for _, ev := range events {
		if ev.Test == "" {
			continue
		}
		key := ev.Package + "." + ev.Test
		if ev.Action == "output" {
			outputByTest[key] = append(outputByTest[key], ev.Output)
		}
		if ev.Action == "fail" {
			failures = append(failures, Failure{
				TestName: ev.Test,
				Package:  ev.Package,
				Message:  strings.Join(outputByTest[key], ""),
			})
		}
	}
	return failures
}

// BuildSummary creates a human-readable summary of the feedback for LLM consumption.
func BuildSummary(fb Feedback) string {
	switch fb.Type {
	case Success:
		return "All tests passed."
	case CompileError:
		return "Build failed. The code does not compile. Fix compilation errors before running tests."
	case Panic:
		return "A test panic occurred. Check for nil pointer dereferences or out-of-range access."
	case Timeout:
		return "A test timed out. The test may be stuck in an infinite loop or waiting on I/O."
	case TestFailure:
		var b strings.Builder
		b.WriteString("Test failures detected:\n")
		for _, f := range fb.Failures {
			b.WriteString("- " + f.TestName)
			if f.Message != "" {
				b.WriteString(": " + f.Message)
			}
			b.WriteString("\n")
		}
		return strings.TrimSpace(b.String())
	default:
		return "Unknown test result."
	}
}
