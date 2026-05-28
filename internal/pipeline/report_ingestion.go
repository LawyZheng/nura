package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/LawyZheng/nura/internal/memory"
	"github.com/LawyZheng/nura/internal/model"
	"github.com/LawyZheng/nura/internal/policy"
	"github.com/LawyZheng/nura/internal/providers"
	"github.com/LawyZheng/nura/internal/store"
)

// StageResult holds the output of a pipeline stage.
type StageResult struct {
	StageName string         `json:"stage_name"`
	Data      map[string]any `json:"data,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// IngestionResult is the final output of the report ingestion pipeline.
type IngestionResult struct {
	ReportType              model.ReportType          `json:"report_type"`
	ExtractedFacts          map[string]any            `json:"extracted_facts"`
	NormalizedIndicators    []*model.MedicalIndicator `json:"normalized_indicators"`
	MergeActions            []string                  `json:"merge_actions,omitempty"`
	PatientStateUpdates     []string                  `json:"patient_state_updates,omitempty"`
	MissingFields           []string                  `json:"missing_or_uncertain_fields,omitempty"`
	Explanation             string                    `json:"user_facing_explanation"`
	ExplanationDisclaimer   string                    `json:"explanation_disclaimer,omitempty"`
	ExplanationConfidence   string                    `json:"explanation_confidence,omitempty"`
	ExplanationSources      []string                  `json:"explanation_sources,omitempty"`
	ExplanationSafetyLabels policy.SafetyLabels       `json:"explanation_safety_labels"`
	Stages                  []StageResult             `json:"stages"`
}

// Pipeline processes raw report text through classification, extraction,
// normalization, merge, and explanation stages.
type Pipeline struct {
	llm     providers.LLMProvider
	store   *store.Store
	updater *memory.Updater
}

func NewPipeline(llm providers.LLMProvider, s *store.Store) *Pipeline {
	return &Pipeline{llm: llm, store: s, updater: memory.NewUpdater(llm, s)}
}

// Run executes the full ingestion pipeline on a raw report for the given patient.
func (p *Pipeline) Run(ctx context.Context, patientID int, rawText, reportDate string) (*IngestionResult, error) {
	result := &IngestionResult{}

	// Stage 1: Classify report type.
	reportType, classifyResult, err := p.classifyReport(ctx, rawText)
	if err != nil {
		return nil, fmt.Errorf("classify: %w", err)
	}
	result.ReportType = reportType
	result.Stages = append(result.Stages, classifyResult)

	// Stage 2: Extract facts.
	facts, extractResult, err := p.extractFacts(ctx, rawText, reportType)
	if err != nil {
		return nil, fmt.Errorf("extract: %w", err)
	}
	result.ExtractedFacts = facts
	result.Stages = append(result.Stages, extractResult)

	// Stage 3: Normalize indicators.
	indicators, normalizeResult, err := p.normalizeIndicators(facts, reportType, reportDate)
	if err != nil {
		return nil, fmt.Errorf("normalize: %w", err)
	}
	result.NormalizedIndicators = indicators
	result.Stages = append(result.Stages, normalizeResult)

	// Stage 4: Merge with existing state.
	mergeActions, mergeResult, err := p.mergeState(ctx, patientID, indicators, reportType, rawText, reportDate)
	if err != nil {
		return nil, fmt.Errorf("merge: %w", err)
	}
	result.MergeActions = mergeActions
	result.Stages = append(result.Stages, mergeResult)

	// Stage 5: Generate user-facing explanation (guarded through OutputGuard).
	explainOut, missingFields, explainResult, err := p.generateExplanation(ctx, rawText, reportType, reportDate, facts, indicators)
	if err != nil {
		return nil, fmt.Errorf("explain: %w", err)
	}
	result.Explanation = explainOut.Content
	result.ExplanationDisclaimer = explainOut.Disclaimer
	result.ExplanationConfidence = explainOut.Confidence
	result.ExplanationSources = explainOut.Sources
	result.ExplanationSafetyLabels = explainOut.Labels
	result.MissingFields = missingFields
	result.Stages = append(result.Stages, explainResult)

	// Stage 6: Update memory.
	memoryResult := StageResult{StageName: "update_memory"}
	if err := p.updater.AfterIngestion(patientID); err != nil {
		memoryResult.Error = err.Error()
	} else {
		memoryResult.Data = map[string]any{"status": "updated"}
	}
	result.Stages = append(result.Stages, memoryResult)

	return result, nil
}

func (p *Pipeline) classifyReport(ctx context.Context, rawText string) (model.ReportType, StageResult, error) {
	resp, err := p.llm.Complete(ctx, providers.CompletionRequest{
		SystemPrompt: `You are a medical report classifier. Given the raw text of a medical report, classify it into one of these types: gastroscopy, hp_breath, hp_antibody, blood_routine, liver_function, kidney_function, stool_routine, stool_occult_blood, general_checkup, unknown. Respond with ONLY the type string.`,
		UserPrompt:   rawText,
		Temperature:  0.0,
	})

	stage := StageResult{StageName: "classify_report"}

	if err != nil {
		stage.Error = err.Error()
		return model.ReportUnknown, stage, err
	}

	typeStr := strings.TrimSpace(strings.ToLower(resp.Content))
	rt := model.ReportType(typeStr)
	stage.Data = map[string]any{"classified_type": string(rt)}
	return rt, stage, nil
}

func (p *Pipeline) extractFacts(ctx context.Context, rawText string, reportType model.ReportType) (map[string]any, StageResult, error) {
	prompt := fmt.Sprintf(`Extract structured medical facts from this %s report. Return a JSON object with all relevant indicators, values, units, and reference ranges found in the text. Include an "indicators" array with objects having fields: name, name_cn, value, unit, reference_low, reference_high, is_abnormal.

Report text:
%s`, string(reportType), rawText)

	resp, err := p.llm.Complete(ctx, providers.CompletionRequest{
		SystemPrompt: "You are a medical data extraction assistant. Extract structured data from medical reports. Always respond with valid JSON.",
		UserPrompt:   prompt,
		Temperature:  0.0,
	})

	stage := StageResult{StageName: "extract_facts"}

	if err != nil {
		stage.Error = err.Error()
		return nil, stage, err
	}

	var facts map[string]any
	content := resp.Content
	// Try to parse the LLM response as JSON; if it fails, wrap in a simple structure.
	if err := json.Unmarshal([]byte(content), &facts); err != nil {
		facts = map[string]any{"raw_extraction": content}
	}

	stage.Data = facts
	return facts, stage, nil
}

func (p *Pipeline) normalizeIndicators(facts map[string]any, reportType model.ReportType, reportDate string) ([]*model.MedicalIndicator, StageResult, error) {
	stage := StageResult{StageName: "normalize_indicators"}
	var indicators []*model.MedicalIndicator

	indicatorsRaw, ok := facts["indicators"]
	if !ok {
		stage.Data = map[string]any{"count": 0, "note": "no indicators array in extracted facts"}
		return indicators, stage, nil
	}

	// The indicators field may be a []any from JSON unmarshalling.
	indicatorsList, ok := indicatorsRaw.([]any)
	if !ok {
		stage.Data = map[string]any{"count": 0, "note": "indicators field is not an array"}
		return indicators, stage, nil
	}

	category := reportTypeToCategory(reportType)

	for _, item := range indicatorsList {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}

		ind := &model.MedicalIndicator{
			Category:        category,
			IndicatorName:   getString(m, "name"),
			IndicatorNameCN: getString(m, "name_cn"),
			Value:           getString(m, "value"),
			Unit:            getString(m, "unit"),
			IsAbnormal:      getBool(m, "is_abnormal"),
			MeasuredAt:      reportDate,
		}

		if v, ok := getFloat(m, "reference_low"); ok {
			ind.ReferenceLow = &v
		}
		if v, ok := getFloat(m, "reference_high"); ok {
			ind.ReferenceHigh = &v
		}

		if ind.IsAbnormal {
			ind.AbnormalDirection = inferAbnormalDirection(ind)
		}

		if ind.IndicatorName != "" && ind.Value != "" {
			indicators = append(indicators, ind)
		}
	}

	stage.Data = map[string]any{"count": len(indicators)}
	return indicators, stage, nil
}

func (p *Pipeline) mergeState(ctx context.Context, patientID int, indicators []*model.MedicalIndicator, reportType model.ReportType, rawText, reportDate string) ([]string, StageResult, error) {
	stage := StageResult{StageName: "merge_state"}
	var actions []string

	// Store the report.
	reportID, err := p.store.InsertHealthReport(&model.HealthReport{
		PatientID:  patientID,
		ReportType: reportType,
		ReportDate: reportDate,
		RawText:    rawText,
	})
	if err != nil {
		stage.Error = err.Error()
		return nil, stage, fmt.Errorf("insert report: %w", err)
	}
	actions = append(actions, fmt.Sprintf("created health_report id=%d", reportID))

	// Store normalized indicators.
	for _, ind := range indicators {
		ind.PatientID = patientID
		ind.ReportID = reportID
		id, err := p.store.InsertIndicator(ind)
		if err != nil {
			// Dedup conflict is expected for same indicator+date.
			actions = append(actions, fmt.Sprintf("indicator %s: dedup or error: %v", ind.IndicatorName, err))
			continue
		}
		actions = append(actions, fmt.Sprintf("created indicator id=%d name=%s", id, ind.IndicatorName))
	}

	// Mark report as processed.
	_ = p.store.UpdateHealthReportProcessed(reportID, string(reportType), "")
	_ = ctx // reserved for future async operations

	stage.Data = map[string]any{"report_id": reportID, "actions": actions}
	return actions, stage, nil
}

func (p *Pipeline) generateExplanation(ctx context.Context, rawText string, reportType model.ReportType, reportDate string, facts map[string]any, indicators []*model.MedicalIndicator) (policy.GuardedOutput, []string, StageResult, error) {
	stage := StageResult{StageName: "generate_explanation"}
	var missing []string

	var abnormal []string
	for _, ind := range indicators {
		if ind.IsAbnormal {
			abnormal = append(abnormal, fmt.Sprintf("%s(%s): %s %s", ind.IndicatorNameCN, ind.IndicatorName, ind.Value, ind.Unit))
		}
	}

	prompt := fmt.Sprintf(`Based on this %s report, provide a clear, patient-friendly Chinese explanation of the findings. Focus on abnormal indicators and what they mean in plain language.

Abnormal indicators found: %s

Rules:
- Write in Chinese
- Use plain language a patient can understand
- Do NOT diagnose or prescribe
- For abnormal values, explain what they mean and suggest consulting a doctor if needed
- End with a reminder that this is for reference only

Report type: %s`, string(reportType), strings.Join(abnormal, "; "), string(reportType))

	resp, err := p.llm.Complete(ctx, providers.CompletionRequest{
		SystemPrompt: "你是一位专业的消化科报告解读助手。用通俗易懂的中文帮助普通人理解检查报告。不做诊断，不建议具体药物。",
		UserPrompt:   prompt,
		Temperature:  0.1,
	})

	if err != nil {
		stage.Error = err.Error()
		return policy.GuardedOutput{}, missing, stage, err
	}

	missing = append(missing, detectIndicatorGaps(facts, indicators)...)

	sources := buildExplanationSources(reportType, reportDate, abnormal)

	confidence := "medium"
	if len(missing) > 0 {
		confidence = "low"
	}

	guard := policy.NewOutputGuard()
	guarded, err := guard.GuardOutput(ctx, policy.AgentOutput{
		Content:    resp.Content,
		Sources:    sources,
		Confidence: confidence,
		RiskLevel:  policy.RiskMedium,
	})
	if err != nil {
		stage.Error = err.Error()
		return policy.GuardedOutput{}, missing, stage, fmt.Errorf("guard explanation: %w", err)
	}

	_ = rawText
	stage.Data = map[string]any{
		"explanation_length": len(guarded.Content),
		"missing_fields":     missing,
		"confidence":         guarded.Confidence,
		"sources_count":      len(sources),
		"sources":            sources,
		"has_disclaimer":     guarded.Labels.HasDisclaimer,
	}
	return guarded, missing, stage, nil
}

func buildExplanationSources(reportType model.ReportType, reportDate string, abnormal []string) []string {
	var sources []string
	sources = append(sources, fmt.Sprintf("报告类型: %s", string(reportType)))
	if len(abnormal) > 0 {
		sources = append(sources, fmt.Sprintf("异常指标: %s", strings.Join(abnormal, ", ")))
	}
	if reportDate != "" {
		sources = append(sources, fmt.Sprintf("报告日期: %s", reportDate))
	}
	return sources
}

func detectIndicatorGaps(facts map[string]any, normalized []*model.MedicalIndicator) []string {
	var gaps []string

	raw, ok := facts["indicators"]
	if !ok {
		gaps = append(gaps, "structured indicators not extracted")
		return gaps
	}

	list, ok := raw.([]any)
	if !ok {
		gaps = append(gaps, "indicators field is not an array")
		return gaps
	}

	if len(normalized) == 0 {
		gaps = append(gaps, "no normalized indicators produced")
		return gaps
	}

	skipped := len(list) - len(normalized)
	if skipped > 0 {
		gaps = append(gaps, fmt.Sprintf("%d incomplete indicator(s) skipped during normalization", skipped))
	}
	return gaps
}

// --- helpers ---

func reportTypeToCategory(rt model.ReportType) string {
	switch rt {
	case model.ReportGastroscopy:
		return "gastroscopy"
	case model.ReportHPBreath, model.ReportHPAntibody:
		return "hp"
	case model.ReportBloodRoutine:
		return "blood"
	case model.ReportLiverFunction:
		return "liver"
	case model.ReportKidneyFunction:
		return "kidney"
	case model.ReportStoolRoutine, model.ReportStoolOccultBlood:
		return "stool"
	default:
		return "general"
	}
}

func getString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func getBool(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func getFloat(m map[string]any, key string) (float64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch f := v.(type) {
	case float64:
		return f, true
	case int:
		return float64(f), true
	default:
		return 0, false
	}
}

func inferAbnormalDirection(ind *model.MedicalIndicator) string {
	// If we have reference ranges, compare numerically.
	if ind.ReferenceLow != nil || ind.ReferenceHigh != nil {
		// Simple heuristic: try parsing value as float.
		var val float64
		if _, err := fmt.Sscanf(ind.Value, "%f", &val); err == nil {
			if ind.ReferenceHigh != nil && val > *ind.ReferenceHigh {
				return "high"
			}
			if ind.ReferenceLow != nil && val < *ind.ReferenceLow {
				return "low"
			}
		}
	}

	// For non-numeric values like HP status.
	lower := strings.ToLower(ind.Value)
	if strings.Contains(lower, "阳性") || strings.Contains(lower, "positive") {
		return "positive"
	}

	return "high" // default assumption for abnormal
}
