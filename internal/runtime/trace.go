package runtime

import (
	"sync"
	"time"

	"github.com/LawyZheng/nura/internal/model"
	"github.com/google/uuid"
)

// Tracer records steps during an agent run.
type Tracer struct {
	mu    sync.Mutex
	run   model.TraceRun
	index int
}

func NewTracer(input string) *Tracer {
	return &Tracer{
		run: model.TraceRun{
			ID:        uuid.New().String(),
			StartedAt: time.Now(),
			Input:     input,
		},
	}
}

// ID returns the trace run ID.
func (t *Tracer) ID() string {
	return t.run.ID
}

// StartStep begins a new trace step and returns its index.
func (t *Tracer) StartStep(stepType, name, input string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	idx := t.index
	t.run.Steps = append(t.run.Steps, model.TraceStep{
		Index:     idx,
		StepType:  stepType,
		Name:      name,
		Input:     input,
		StartedAt: time.Now(),
	})
	t.index++
	return idx
}

// EndStep finalizes a trace step with its output.
func (t *Tracer) EndStep(idx int, output, errMsg string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if idx < len(t.run.Steps) {
		t.run.Steps[idx].Output = output
		t.run.Steps[idx].Error = errMsg
		t.run.Steps[idx].DurationMs = time.Since(t.run.Steps[idx].StartedAt).Milliseconds()
	}
}

// Finish marks the trace run as complete.
func (t *Tracer) Finish(output, errMsg string) model.TraceRun {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	t.run.FinishedAt = &now
	t.run.Output = output
	t.run.Error = errMsg
	t.run.DurationMs = now.Sub(t.run.StartedAt).Milliseconds()
	return t.run
}

// Run returns a copy of the current trace run.
func (t *Tracer) Run() model.TraceRun {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.run
}

// TraceStore keeps completed traces in memory for debugging.
type TraceStore struct {
	mu     sync.RWMutex
	traces map[string]model.TraceRun
}

func NewTraceStore() *TraceStore {
	return &TraceStore{traces: make(map[string]model.TraceRun)}
}

func (ts *TraceStore) Save(run model.TraceRun) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.traces[run.ID] = run
}

func (ts *TraceStore) Get(id string) (model.TraceRun, bool) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	run, ok := ts.traces[id]
	return run, ok
}
