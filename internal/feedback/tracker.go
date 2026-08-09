package feedback

import "sync"

// Tracker tracks per-task retry rounds to prevent infinite repair loops.
type Tracker struct {
	mu        sync.Mutex
	statuses  map[string]*Status
	maxRounds int
}

// NewTracker creates a Tracker with the given max retry limit.
func NewTracker(maxRounds int) *Tracker {
	return &Tracker{
		statuses:  make(map[string]*Status),
		maxRounds: maxRounds,
	}
}

// Record increments the round counter for a task. Returns true if the
// agent should stop (max rounds exceeded).
func (t *Tracker) Record(taskID string) (shouldStop bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	s, ok := t.statuses[taskID]
	if !ok {
		s = &Status{TaskID: taskID, MaxRounds: t.maxRounds}
		t.statuses[taskID] = s
	}
	s.Round++
	return s.Round > s.MaxRounds
}

// Reset clears the round counter for a task (e.g. after a successful fix).
func (t *Tracker) Reset(taskID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.statuses, taskID)
}

// Status returns the current Status for a task, or nil if not tracked.
func (t *Tracker) Status(taskID string) *Status {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.statuses[taskID]
}
