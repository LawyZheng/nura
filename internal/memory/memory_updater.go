package memory

import (
	"context"
	"fmt"

	"github.com/LawyZheng/nura/internal/model"
	"github.com/LawyZheng/nura/internal/providers"
	"github.com/LawyZheng/nura/internal/store"
)

// Updater regenerates MemorySummary records after data changes (e.g., report ingestion).
type Updater struct {
	llm   providers.LLMProvider
	store *store.Store
	state *StructuredState
}

func NewUpdater(llm providers.LLMProvider, s *store.Store) *Updater {
	return &Updater{
		llm:   llm,
		store: s,
		state: NewStructuredState(s),
	}
}

// AfterIngestion regenerates the report_summary MemorySummary for the given patient.
// It supersedes any previous active summary of the same category.
func (u *Updater) AfterIngestion(patientID int) error {
	summary, err := u.state.Summarize(patientID)
	if err != nil {
		return fmt.Errorf("build state summary: %w", err)
	}

	resp, err := u.llm.Complete(context.Background(), providers.CompletionRequest{
		SystemPrompt: `你是健康数据管理助手。根据患者当前状态，生成一段简洁的中文摘要（200字以内）。
摘要应包含：当前诊断、用药情况、最近检查结果要点、异常指标。
不做诊断建议，只客观归纳已有数据。`,
		UserPrompt:  summary,
		Temperature: 0.1,
	})
	if err != nil {
		return fmt.Errorf("generate summary: %w", err)
	}

	active, err := u.store.GetActiveMemorySummaries(patientID)
	if err != nil {
		return fmt.Errorf("get active summaries: %w", err)
	}

	newID, err := u.store.InsertMemorySummary(&model.MemorySummary{
		PatientID: patientID,
		Category:  "report_summary",
		Content:   resp.Content,
	})
	if err != nil {
		return fmt.Errorf("insert new summary: %w", err)
	}

	for _, ms := range active {
		if ms.Category == "report_summary" {
			_ = u.store.SupersedeMemorySummary(ms.ID, newID)
		}
	}

	return nil
}
