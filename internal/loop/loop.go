// Package loop implements the agent main loop (SPEC §3.2) — the core of
// the harness. It assembles context, calls the LLM, dispatches tool calls
// through the guardrail, feeds back test results, and decides when to stop.
package loop

import (
	"context"
	"fmt"
	"io"

	"github.com/chenstar1025/gopher/internal/config"
	"github.com/chenstar1025/gopher/internal/feedback"
	"github.com/chenstar1025/gopher/internal/guard"
	"github.com/chenstar1025/gopher/internal/llm"
	"github.com/chenstar1025/gopher/internal/memory"
	"github.com/chenstar1025/gopher/internal/tools"
)

// Session holds the live state of a single agent run.
type Session struct {
	ID       string
	Messages []llm.Message
	Round    int
	Status   SessionStatus
}

// SessionStatus is the current state of a session.
type SessionStatus string

const (
	StatusRunning        SessionStatus = "running"
	StatusWaitingApproval SessionStatus = "waiting_approval"
	StatusDone           SessionStatus = "done"
	StatusFailed         SessionStatus = "failed"
)

// Agent orchestrates a single coding session.
type Agent struct {
	llm      llm.LLM
	tools    *tools.ToolRegistry
	guard    *guard.Guardrail
	tracker  *feedback.Tracker
	mem      *memory.Store
	cfg      config.Config
	confirm  io.Reader // stdin for guardrail confirmations
}

// New creates an Agent with the given components.
func New(l llm.LLM, reg *tools.ToolRegistry, g *guard.Guardrail, tr *feedback.Tracker, mem *memory.Store, cfg config.Config, confirm io.Reader) *Agent {
	return &Agent{
		llm:     l,
		tools:   reg,
		guard:   g,
		tracker: tr,
		mem:     mem,
		cfg:     cfg,
		confirm: confirm,
	}
}

// Run executes the main loop for a given user task and returns the session.
func (a *Agent) Run(ctx context.Context, task string) (*Session, error) {
	sess := &Session{ID: newSessionID(), Status: StatusRunning}
	sess.Messages = []llm.Message{
		{Role: "system", Content: a.buildSystemPrompt()},
		{Role: "user", Content: task},
	}

	for sess.Round = 1; sess.Round <= a.cfg.MaxRounds; sess.Round++ {
		select {
		case <-ctx.Done():
			sess.Status = StatusFailed
			return sess, ctx.Err()
		default:
		}

		// Step 1: Call LLM
		resp, err := a.llm.Chat(ctx, sess.Messages, a.toolDefs())
		if err != nil {
			sess.Status = StatusFailed
			return sess, fmt.Errorf("round %d: LLM error: %w", sess.Round, err)
		}

		sess.Messages = append(sess.Messages, resp.Messages...)

		// Step 2: If no tool calls, agent is done
		if len(resp.ToolCalls) == 0 {
			sess.Status = StatusDone
			return sess, nil
		}

		// Step 3: Execute each tool call
		var allResults []llm.Message
		for _, tc := range resp.ToolCalls {
			tcTools := tools.ToolCall{ID: tc.ID, Name: tc.Name, Args: tc.Args}

			// Guard check (only for shell commands)
			if tcTools.Name == "run_shell" {
				blocked, reason := a.guard.Check(tcTools)
				if blocked {
					if !a.guard.Confirm(tcTools) {
						allResults = append(allResults, llm.ToolResultMsg(tc.ID,
							fmt.Sprintf("Blocked: %s", reason)))
						continue
					}
				}
			}

			// Execute
			result := a.tools.Execute(tcTools)
			msg := llm.ToolResultMsg(tc.ID, result.Output)
			if !result.Success && result.Error != "" {
				msg.Content = fmt.Sprintf("Error: %s\n%s", result.Error, result.Output)
			}
			allResults = append(allResults, msg)

			// Feedback loop: if this was run_test, parse and inject feedback
			if tc.Name == "run_test" {
				fb := feedback.MakeFeedback(result.Output)
				allResults = append(allResults, feedback.Inject(fb))

				if fb.Type != feedback.Success {
					shouldStop := a.tracker.Record(sess.ID)
					if shouldStop {
						sess.Messages = append(sess.Messages, allResults...)
						st := a.tracker.Status(sess.ID)
						maxRounds := 0
						if st != nil {
							maxRounds = st.MaxRounds
						}
						sess.Messages = append(sess.Messages,
							llm.AssistantMsg(fmt.Sprintf("Max retry rounds (%d) exceeded. Stopping.", maxRounds)))
						sess.Status = StatusFailed
						return sess, nil
					}
				} else {
					a.tracker.Reset(sess.ID)
				}
			}
		}

		sess.Messages = append(sess.Messages, allResults...)

		// Step 4: Dead-loop detection — same tool call repeated
		if a.isDeadLoop(sess) {
			sess.Status = StatusFailed
			return sess, fmt.Errorf("dead loop detected at round %d", sess.Round)
		}
	}

	sess.Status = StatusFailed
	return sess, fmt.Errorf("max rounds (%d) exceeded", a.cfg.MaxRounds)
}

func (a *Agent) buildSystemPrompt() string {
	prompt := `You are Gopher, a coding agent. You help users with software development tasks.
You can read files, write files, list directories, run shell commands, and run tests.

When you make code changes, ALWAYS run tests afterward to verify your changes work.
If tests fail, analyze the failure and fix the problem. Do not guess — read error output carefully.
Do not perform dangerous operations — they will be blocked and require confirmation.

Work step by step. Read first, then think, then act.`
	rules := memory.LoadRules(".")
	if rules != "" {
		prompt += "\n\n## Project Rules\n" + rules
	}
	return prompt
}

func (a *Agent) toolDefs() []llm.ToolDef {
	var defs []llm.ToolDef
	for _, t := range a.tools.List() {
		defs = append(defs, llm.ToolDef{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
	}
	return defs
}

// isDeadLoop checks if the agent session has exceeded retry limits.
// The primary dead-loop guard is the feedback tracker; this is a secondary check.
func (a *Agent) isDeadLoop(sess *Session) bool {
	// If no tool calls from LLM in the last round, we're not in a loop.
	// Dead loop detection is primarily handled by the feedback tracker's
	// Record/ShouldStop mechanism.
	return false
}

var idCounter int

func newSessionID() string {
	idCounter++
	return fmt.Sprintf("session-%d", idCounter)
}
