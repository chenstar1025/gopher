package feedback

import (
	"testing"
)

// ---- Parser tests ----

func TestParseTestOutput_Success(t *testing.T) {
	jsonl := `{"Time":"2026-01-01T00:00:00Z","Action":"run","Package":"example","Test":"TestFoo"}
{"Time":"2026-01-01T00:00:01Z","Action":"pass","Package":"example","Test":"TestFoo","Elapsed":0.1}
{"Time":"2026-01-01T00:00:01Z","Action":"pass","Package":"example","Elapsed":0.1}`

	events, err := ParseTestOutput(jsonl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	if events[0].Test != "TestFoo" {
		t.Errorf("expected TestFoo, got %s", events[0].Test)
	}
}

func TestParseTestOutput_HandlesNonJSONLines(t *testing.T) {
	jsonl := `{"Time":"2026-01-01T00:00:00Z","Action":"fail","Package":"example","Test":"TestFoo"}
FAIL
some garbled output`

	events, _ := ParseTestOutput(jsonl)
	if len(events) != 1 {
		t.Fatalf("expected 1 valid event, got %d", len(events))
	}
	if events[0].Action != "fail" {
		t.Errorf("expected fail, got %s", events[0].Action)
	}
}

func TestParseTestOutput_Empty(t *testing.T) {
	events, err := ParseTestOutput("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

// ---- Classifier tests ----

func TestClassify_Success(t *testing.T) {
	events := []TestEvent{
		{Action: "run", Test: "TestA"},
		{Action: "pass", Test: "TestA"},
		{Action: "pass"},
	}
	if got := Classify(events); got != Success {
		t.Errorf("expected success, got %s", got)
	}
}

func TestClassify_CompileError(t *testing.T) {
	events := []TestEvent{
		{Action: "output", Output: "build failed: missing import\n"},
	}
	if got := Classify(events); got != CompileError {
		t.Errorf("expected compile_error, got %s", got)
	}
}

func TestClassify_SetupFailed(t *testing.T) {
	events := []TestEvent{
		{Action: "output", Output: "setup failed: cannot create temp dir\n"},
	}
	if got := Classify(events); got != CompileError {
		t.Errorf("expected compile_error for setup failed, got %s", got)
	}
}

func TestClassify_Panic(t *testing.T) {
	events := []TestEvent{
		{Action: "output", Output: "panic: runtime error: nil pointer dereference\n"},
	}
	if got := Classify(events); got != Panic {
		t.Errorf("expected panic, got %s", got)
	}
}

func TestClassify_Timeout(t *testing.T) {
	events := []TestEvent{
		{Action: "fail", Output: "test timed out after 30s"},
	}
	if got := Classify(events); got != Timeout {
		t.Errorf("expected timeout, got %s", got)
	}
}

func TestClassify_TestFailure(t *testing.T) {
	events := []TestEvent{
		{Action: "run", Test: "TestFoo"},
		{Action: "fail", Test: "TestFoo"},
		{Action: "pass"},
	}
	if got := Classify(events); got != TestFailure {
		t.Errorf("expected test_failure, got %s", got)
	}
}

func TestClassify_PanicOverridesFailure(t *testing.T) {
	events := []TestEvent{
		{Action: "fail", Test: "TestFoo"},
		{Action: "output", Output: "panic: unexpected error"},
	}
	if got := Classify(events); got != Panic {
		t.Errorf("panic should override failure, got %s", got)
	}
}

func TestClassify_CompileOverridesEverything(t *testing.T) {
	events := []TestEvent{
		{Action: "fail", Test: "TestFoo"},
		{Action: "output", Output: "panic: some error"},
		{Action: "output", Output: "build failed: syntax error"},
	}
	if got := Classify(events); got != CompileError {
		t.Errorf("compile_error should override everything, got %s", got)
	}
}

// ---- ExtractFailures tests ----

func TestExtractFailures(t *testing.T) {
	events := []TestEvent{
		{Action: "output", Package: "pkg", Test: "TestA", Output: "expected 1, got 2\n"},
		{Action: "fail", Package: "pkg", Test: "TestA"},
		{Action: "output", Package: "pkg", Test: "TestB", Output: "unexpected nil\n"},
		{Action: "fail", Package: "pkg", Test: "TestB"},
		{Action: "pass", Test: "TestC"}, // passed, not a failure
	}

	failures := ExtractFailures(events)
	if len(failures) != 2 {
		t.Fatalf("expected 2 failures, got %d", len(failures))
	}
	if failures[0].TestName != "TestA" {
		t.Errorf("expected TestA, got %s", failures[0].TestName)
	}
	if failures[1].TestName != "TestB" {
		t.Errorf("expected TestB, got %s", failures[1].TestName)
	}
}

// ---- BuildSummary tests ----

func TestBuildSummary_Success(t *testing.T) {
	fb := Feedback{Type: Success}
	got := BuildSummary(fb)
	if got != "All tests passed." {
		t.Errorf("unexpected: %s", got)
	}
}

func TestBuildSummary_TestFailure(t *testing.T) {
	fb := Feedback{
		Type: TestFailure,
		Failures: []Failure{
			{TestName: "TestFoo", Message: "expected 1, got 2"},
			{TestName: "TestBar", Message: "nil pointer"},
		},
	}
	got := BuildSummary(fb)
	if !contains(got, "TestFoo") || !contains(got, "TestBar") {
		t.Errorf("missing test names: %s", got)
	}
}

// ---- Inject tests ----

func TestInject(t *testing.T) {
	fb := Feedback{Type: Success, Summary: "All tests passed."}
	msg := Inject(fb)
	if msg.Role != "tool" {
		t.Errorf("expected role tool, got %s", msg.Role)
	}
	if msg.Content != "All tests passed." {
		t.Errorf("unexpected content: %s", msg.Content)
	}
}

// ---- MakeFeedback pipeline tests ----

func TestMakeFeedback_Success(t *testing.T) {
	jsonl := `{"Action":"run","Test":"TestA"}
{"Action":"pass","Test":"TestA"}
{"Action":"pass"}`
	fb := MakeFeedback(jsonl)
	if fb.Type != Success {
		t.Errorf("expected success, got %s", fb.Type)
	}
}

func TestMakeFeedback_Failure(t *testing.T) {
	jsonl := `{"Action":"run","Package":"pkg","Test":"TestFoo"}
{"Action":"output","Package":"pkg","Test":"TestFoo","Output":"FAIL: TestFoo"}
{"Action":"fail","Package":"pkg","Test":"TestFoo"}
{"Action":"pass","Package":"pkg"}`
	fb := MakeFeedback(jsonl)
	if fb.Type != TestFailure {
		t.Errorf("expected test_failure, got %s", fb.Type)
	}
	if len(fb.Failures) != 1 {
		t.Errorf("expected 1 failure, got %d", len(fb.Failures))
	}
}

func TestMakeFeedback_CompileError(t *testing.T) {
	jsonl := `{"Action":"output","Output":"build failed: syntax error at main.go:10\n"}`
	fb := MakeFeedback(jsonl)
	if fb.Type != CompileError {
		t.Errorf("expected compile_error, got %s", fb.Type)
	}
}

func TestMakeFeedback_Empty(t *testing.T) {
	fb := MakeFeedback("")
	if fb.Type != CompileError {
		t.Errorf("expected compile_error for empty input, got %s", fb.Type)
	}
}

// ---- Tracker tests ----

func TestTracker_UnderLimit(t *testing.T) {
	tr := NewTracker(3)
	if tr.Record("task1") {
		t.Error("should not stop after 1st retry")
	}
	if tr.Record("task1") {
		t.Error("should not stop after 2nd retry")
	}
	if tr.Record("task1") {
		t.Error("should not stop after 3rd retry (max)")
	}
	if !tr.Record("task1") {
		t.Error("should stop after 4th retry (exceeds max)")
	}
}

func TestTracker_SeparateTasks(t *testing.T) {
	tr := NewTracker(3)
	tr.Record("task1")
	tr.Record("task1")
	// task2 should start fresh
	if tr.Record("task2") {
		t.Error("task2 should not stop on 1st retry")
	}
}

func TestTracker_Reset(t *testing.T) {
	tr := NewTracker(2)
	tr.Record("task1")
	tr.Record("task1")
	tr.Reset("task1")
	if tr.Record("task1") {
		t.Error("should be back at round 1 after reset")
	}
}

func TestTracker_Status(t *testing.T) {
	tr := NewTracker(3)
	tr.Record("taskX")
	tr.Record("taskX")

	s := tr.Status("taskX")
	if s == nil {
		t.Fatal("expected status")
	}
	if s.Round != 2 {
		t.Errorf("expected round 2, got %d", s.Round)
	}
}

// ---- helpers ----

func contains(s, sub string) bool {
	return len(s) >= len(sub) && containsBytes(s, sub)
}

func containsBytes(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
