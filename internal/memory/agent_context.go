package memory

import (
	"fmt"
	"strings"

	"github.com/LawyZheng/nura/internal/store"
)

// AgentContext is the context snapshot assembled for the current agent task.
type AgentContext struct {
	PatientSummary string   `json:"patient_summary"`
	RelevantFacts  []string `json:"relevant_facts,omitempty"`
	Uncertainties  []string `json:"uncertainties,omitempty"`
}

// ContextBuilder assembles the relevant context for a given task.
type ContextBuilder struct {
	state *StructuredState
	raw   *RawEvents
}

func NewContextBuilder(s *store.Store) *ContextBuilder {
	return &ContextBuilder{
		state: NewStructuredState(s),
		raw:   NewRawEvents(s),
	}
}

// Build creates an AgentContext for the current task, incorporating patient state
// and optionally relevant facts based on the task hint.
func (cb *ContextBuilder) Build(patientID int, taskHint string) (*AgentContext, error) {
	summary, err := cb.state.Summarize(patientID)
	if err != nil {
		return nil, fmt.Errorf("build patient summary: %w", err)
	}

	ctx := &AgentContext{
		PatientSummary: summary,
	}

	switch {
	case strings.Contains(taskHint, "report"):
		reports, err := cb.raw.ListReports(patientID)
		if err == nil {
			for _, r := range reports {
				ctx.RelevantFacts = append(ctx.RelevantFacts,
					fmt.Sprintf("[%s] %s 报告 (%s)", r.ReportDate, string(r.ReportType), r.Institution))
			}
		}

	case strings.Contains(taskHint, "symptom"):
		symptoms, err := cb.raw.ListSymptoms(patientID, 20)
		if err == nil {
			for _, s := range symptoms {
				fact := fmt.Sprintf("[%s] 疼痛评分:%v 位置:%s",
					s.RecordedAt.Format("2006-01-02"), s.PainScore, s.PainLocation)
				ctx.RelevantFacts = append(ctx.RelevantFacts, fact)
			}
		}

	case strings.Contains(taskHint, "medication"):
		meds, err := cb.raw.ListActiveMedications(patientID)
		if err == nil {
			for _, m := range meds {
				ctx.RelevantFacts = append(ctx.RelevantFacts,
					fmt.Sprintf("%s %s %s (active)", m.Name, m.Dosage, m.Frequency))
			}
		}
	}

	return ctx, nil
}
