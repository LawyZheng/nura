package model

import "time"

// TraceRun records a complete agent execution.
type TraceRun struct {
	ID          string      `json:"id"` // UUID
	StartedAt   time.Time   `json:"started_at"`
	FinishedAt  *time.Time  `json:"finished_at,omitempty"`
	Input       string      `json:"input"`
	Output      string      `json:"output,omitempty"`
	Steps       []TraceStep `json:"steps"`
	Error       string      `json:"error,omitempty"`
	DurationMs  int64       `json:"duration_ms,omitempty"`
}

// TraceStep records a single step inside an agent run.
type TraceStep struct {
	Index      int       `json:"index"`
	StepType   string    `json:"step_type"` // plan / tool_call / llm_call / policy_check / guard
	Name       string    `json:"name"`
	Input      string    `json:"input,omitempty"`
	Output     string    `json:"output,omitempty"`
	Error      string    `json:"error,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	DurationMs int64     `json:"duration_ms"`
}
