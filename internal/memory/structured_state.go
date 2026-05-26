package memory

import (
	"fmt"
	"strings"

	"github.com/LawyZheng/nura/internal/model"
	"github.com/LawyZheng/nura/internal/store"
)

// PatientState is the derived current state of the patient, built from raw events.
type PatientState struct {
	Profile          *model.PatientProfile   `json:"profile,omitempty"`
	Diagnoses        []*model.Diagnosis      `json:"diagnoses,omitempty"`
	ActiveMedications []*model.Medication    `json:"active_medications,omitempty"`
	RecentSymptoms   []*model.SymptomLog     `json:"recent_symptoms,omitempty"`
	AbnormalIndicators []*model.MedicalIndicator `json:"abnormal_indicators,omitempty"`
	MemorySummaries  []*model.MemorySummary  `json:"memory_summaries,omitempty"`
}

// StructuredState derives and queries the current patient state.
type StructuredState struct {
	store *store.Store
}

func NewStructuredState(s *store.Store) *StructuredState {
	return &StructuredState{store: s}
}

// BuildState assembles the current patient state from all raw data.
func (ss *StructuredState) BuildState() (*PatientState, error) {
	profile, err := ss.store.GetPatientProfile()
	if err != nil {
		profile = nil // no profile yet is fine
	}

	diagnoses, err := ss.store.ListDiagnoses()
	if err != nil {
		return nil, fmt.Errorf("list diagnoses: %w", err)
	}

	meds, err := ss.store.ListActiveMedications()
	if err != nil {
		return nil, fmt.Errorf("list medications: %w", err)
	}

	symptoms, err := ss.store.ListSymptomLogs(10)
	if err != nil {
		return nil, fmt.Errorf("list symptoms: %w", err)
	}

	summaries, err := ss.store.GetActiveMemorySummaries()
	if err != nil {
		return nil, fmt.Errorf("list summaries: %w", err)
	}

	return &PatientState{
		Profile:           profile,
		Diagnoses:         diagnoses,
		ActiveMedications: meds,
		RecentSymptoms:    symptoms,
		MemorySummaries:   summaries,
	}, nil
}

// Summarize returns a human-readable summary of the current patient state
// suitable for injecting into an LLM prompt.
func (ss *StructuredState) Summarize() (string, error) {
	state, err := ss.BuildState()
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("## 患者状态摘要\n\n")

	if state.Profile != nil {
		sb.WriteString("### 基本信息\n")
		if state.Profile.Name != "" {
			sb.WriteString(fmt.Sprintf("- 姓名：%s\n", state.Profile.Name))
		}
		if state.Profile.Gender != "" {
			sb.WriteString(fmt.Sprintf("- 性别：%s\n", state.Profile.Gender))
		}
		if state.Profile.BirthDate != "" {
			sb.WriteString(fmt.Sprintf("- 出生日期：%s\n", state.Profile.BirthDate))
		}
		if len(state.Profile.Allergies) > 0 {
			sb.WriteString(fmt.Sprintf("- 过敏史：%s\n", strings.Join(state.Profile.Allergies, "、")))
		}
		sb.WriteString("\n")
	}

	if len(state.Diagnoses) > 0 {
		sb.WriteString("### 诊断历史\n")
		for _, d := range state.Diagnoses {
			sb.WriteString(fmt.Sprintf("- %s：%s", d.DiagnosisDate, d.Condition))
			if d.Note != "" {
				sb.WriteString(fmt.Sprintf("（%s）", d.Note))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if len(state.ActiveMedications) > 0 {
		sb.WriteString("### 当前用药\n")
		for _, m := range state.ActiveMedications {
			sb.WriteString(fmt.Sprintf("- %s %s %s", m.Name, m.Dosage, m.Frequency))
			if m.CourseEnd != "" {
				sb.WriteString(fmt.Sprintf("（至 %s）", m.CourseEnd))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if len(state.MemorySummaries) > 0 {
		sb.WriteString("### AI 记忆摘要\n")
		for _, ms := range state.MemorySummaries {
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", ms.Category, ms.Content))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}
