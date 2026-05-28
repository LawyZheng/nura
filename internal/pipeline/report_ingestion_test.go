package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LawyZheng/nura/internal/model"
	"github.com/LawyZheng/nura/internal/providers"
	"github.com/LawyZheng/nura/internal/store"
)

type mockConfig struct {
	extractionJSON string // JSON string returned for extraction stage
	explanation    string // text returned for explanation stage
}

func newMockPipeline(t *testing.T, cfg mockConfig) (*Pipeline, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	archiveDir := filepath.Join(dir, "evidence")

	llm := &providers.MockProvider{
		CompleteFunc: func(ctx context.Context, req providers.CompletionRequest) (*providers.CompletionResponse, error) {
			if strings.Contains(req.SystemPrompt, "classifier") {
				return &providers.CompletionResponse{
					Content: "gastroscopy",
					Usage:   providers.Usage{InputTokens: 10, OutputTokens: 5},
				}, nil
			}
			if strings.Contains(req.SystemPrompt, "extraction") {
				return &providers.CompletionResponse{
					Content: cfg.extractionJSON,
					Usage:   providers.Usage{InputTokens: 100, OutputTokens: 50},
				}, nil
			}
			return &providers.CompletionResponse{
				Content: cfg.explanation,
				Usage:   providers.Usage{InputTokens: 200, OutputTokens: 100},
			}, nil
		},
	}
	return NewPipeline(llm, s, archiveDir), s
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(data)
}

func createTestPatient(t *testing.T, s *store.Store) int {
	t.Helper()
	id, err := s.CreatePatientProfile(&model.PatientProfile{Name: "Test Patient"})
	if err != nil {
		t.Fatalf("create patient: %v", err)
	}
	return id
}

// SYNTHETIC DATA - not real patient information.
var defaultExtraction = map[string]any{
	"indicators": []map[string]any{
		{"name": "ulcer_stage", "name_cn": "溃疡分期", "value": "A2", "is_abnormal": true},
		{"name": "ulcer_size", "name_cn": "溃疡大小", "value": "0.8x0.6", "unit": "cm", "is_abnormal": false},
		{"name": "hp_status", "name_cn": "幽门螺杆菌", "value": "阳性", "is_abnormal": true},
	},
}

const defaultExplanation = "您的胃镜报告显示十二指肠球部有一处 A2 期溃疡（活动期），大小约 0.8×0.6cm。HP 检测阳性，建议咨询医生进一步评估。"

func newTestPipeline(t *testing.T) (*Pipeline, *store.Store) {
	t.Helper()
	return newMockPipeline(t, mockConfig{
		extractionJSON: mustJSON(t, defaultExtraction),
		explanation:    defaultExplanation,
	})
}

func TestPipeline_GastroscopyReport(t *testing.T) {
	p, s := newTestPipeline(t)
	pid := createTestPatient(t, s)

	// SYNTHETIC DATA - not real patient information.
	rawText := `胃镜检查报告
十二指肠球部：前壁可见一处溃疡，大小约 0.8×0.6cm，底覆白苔。
分期：A2 期（活动期）。
快速尿素酶试验：阳性（+）`

	result, err := p.Run(context.Background(), pid, rawText, "2024-03-01", "")
	if err != nil {
		t.Fatal(err)
	}

	if result.ReportType != model.ReportGastroscopy {
		t.Errorf("report type = %q, want gastroscopy", result.ReportType)
	}
	if len(result.NormalizedIndicators) == 0 {
		t.Error("expected normalized indicators")
	}
	if result.Explanation == "" {
		t.Error("expected non-empty explanation")
	}
	if len(result.Stages) != 6 {
		t.Errorf("expected 6 stages, got %d", len(result.Stages))
	}

	reports, err := s.ListHealthReports(pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 {
		t.Errorf("expected 1 stored report, got %d", len(reports))
	}
}

func TestPipeline_StageNames(t *testing.T) {
	p, s := newTestPipeline(t)
	pid := createTestPatient(t, s)

	result, err := p.Run(context.Background(), pid, "test report", "2024-01-01", "")
	if err != nil {
		t.Fatal(err)
	}

	expectedStages := []string{
		"classify_report",
		"extract_facts",
		"normalize_indicators",
		"merge_state",
		"generate_explanation",
		"update_memory",
	}
	for i, expected := range expectedStages {
		if i >= len(result.Stages) {
			t.Errorf("missing stage %d: %s", i, expected)
			continue
		}
		if result.Stages[i].StageName != expected {
			t.Errorf("stage %d name = %q, want %q", i, result.Stages[i].StageName, expected)
		}
	}
}

func TestPipeline_ReportExplanationUsesOutputGuard(t *testing.T) {
	// SYNTHETIC DATA — explanation deliberately omits disclaimer.
	p, s := newMockPipeline(t, mockConfig{
		extractionJSON: mustJSON(t, map[string]any{
			"indicators": []map[string]any{
				{"name": "ulcer_stage", "name_cn": "溃疡分期", "value": "A2", "is_abnormal": true},
				{"name": "hp_status", "name_cn": "幽门螺杆菌", "value": "阳性", "is_abnormal": true},
			},
		}),
		explanation: "您的胃镜报告显示十二指肠球部有一处 A2 期溃疡（活动期）。HP 检测阳性，建议咨询医生进一步评估。",
	})
	pid := createTestPatient(t, s)

	rawText := `胃镜检查报告
十二指肠球部：前壁可见一处溃疡，大小约 0.8×0.6cm。
分期：A2 期（活动期）。
快速尿素酶试验：阳性（+）`

	result, err := p.Run(context.Background(), pid, rawText, "2024-03-01", "")
	if err != nil {
		t.Fatal(err)
	}

	if result.ExplanationDisclaimer == "" {
		t.Error("expected non-empty ExplanationDisclaimer")
	}
	if !strings.Contains(result.Explanation, "不构成医疗建议") {
		t.Error("expected Explanation to contain standard disclaimer text")
	}
	if !result.ExplanationSafetyLabels.HasDisclaimer {
		t.Error("expected ExplanationSafetyLabels.HasDisclaimer to be true")
	}
	if result.ExplanationConfidence != "medium" {
		t.Errorf("expected ExplanationConfidence = 'medium', got %q", result.ExplanationConfidence)
	}
	if !strings.Contains(strings.Join(result.ExplanationSources, " "), "gastroscopy") {
		t.Error("expected sources to reference report type")
	}
	for _, st := range result.Stages {
		if st.StageName == "generate_explanation" {
			if st.Data["sources_count"] == nil || st.Data["has_disclaimer"] == nil || st.Data["confidence"] == nil {
				t.Error("expected generate_explanation stage data to include safety metadata")
			}
			return
		}
	}
	t.Fatal("missing generate_explanation stage")
}

func TestPipeline_ReportExplanationSanitizesUnsafeContent(t *testing.T) {
	// SYNTHETIC DATA — explanation deliberately includes unsafe prescription language.
	p, s := newMockPipeline(t, mockConfig{
		extractionJSON: mustJSON(t, map[string]any{
			"indicators": []map[string]any{
				{"name": "ulcer_stage", "name_cn": "溃疡分期", "value": "A2", "is_abnormal": true},
			},
		}),
		explanation: "您的溃疡处于活动期，建议服用奥美拉唑进行治疗。HP 阳性，建议进行根治治疗。好转后可以减量。",
	})
	pid := createTestPatient(t, s)

	rawText := `胃镜检查报告
十二指肠球部：前壁可见一处溃疡。`

	result, err := p.Run(context.Background(), pid, rawText, "2024-01-01", "")
	if err != nil {
		t.Fatal(err)
	}

	forbidden := []string{"根治治疗", "建议服用", "减量"}
	for _, word := range forbidden {
		if strings.Contains(result.Explanation, word) {
			t.Errorf("explanation still contains unsafe phrase %q after guard", word)
		}
	}
	if !strings.Contains(result.Explanation, "请咨询医生") {
		t.Error("expected safe replacement wording containing '请咨询医生'")
	}
	if !result.ExplanationSafetyLabels.HasSanitizedContent {
		t.Error("expected HasSanitizedContent label to be true")
	}
}

func TestPipeline_ReportExplanationLowConfidenceOnIndicatorGaps(t *testing.T) {
	cases := []struct {
		name       string
		extraction any
	}{
		{"missing indicators field", map[string]any{"summary": "报告文本模糊，无法提取结构化指标"}},
		{"empty indicators array", map[string]any{"indicators": []map[string]any{}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, s := newMockPipeline(t, mockConfig{
				extractionJSON: mustJSON(t, tc.extraction),
				explanation:    "报告未发现可提取的结构化指标，建议咨询医生进一步评估。",
			})
			pid := createTestPatient(t, s)
			result, err := p.Run(context.Background(), pid, "胃镜检查报告（内容不清）", "2024-01-01", "")
			if err != nil {
				t.Fatal(err)
			}
			if len(result.MissingFields) == 0 {
				t.Fatal("expected non-empty MissingFields")
			}
			if result.ExplanationConfidence != "low" {
				t.Errorf("expected confidence 'low', got %q", result.ExplanationConfidence)
			}
		})
	}
}

func TestPipeline_PartialIndicatorGapDetected(t *testing.T) {
	// SYNTHETIC DATA — one valid, one incomplete (missing value).
	p, s := newMockPipeline(t, mockConfig{
		extractionJSON: mustJSON(t, map[string]any{
			"indicators": []map[string]any{
				{"name": "ulcer_stage", "name_cn": "溃疡分期", "value": "A2", "is_abnormal": true},
				{"name": "hp_status", "name_cn": "幽门螺杆菌", "is_abnormal": true},
			},
		}),
		explanation: "您的胃镜报告显示溃疡活动期。部分指标信息不完整，建议咨询医生进一步评估。",
	})
	pid := createTestPatient(t, s)

	rawText := `胃镜检查报告
十二指肠球部：前壁可见一处溃疡。
快速尿素酶试验：阳性（+）`

	result, err := p.Run(context.Background(), pid, rawText, "2024-03-01", "")
	if err != nil {
		t.Fatal(err)
	}

	if len(result.NormalizedIndicators) != 1 {
		t.Fatalf("expected 1 normalized indicator, got %d", len(result.NormalizedIndicators))
	}
	if len(result.MissingFields) == 0 {
		t.Fatal("expected non-empty MissingFields when indicator was skipped during normalization")
	}
	joined := strings.Join(result.MissingFields, " ")
	if !strings.Contains(joined, "skipped") && !strings.Contains(joined, "incomplete") {
		t.Errorf("expected MissingFields to mention skipped/incomplete indicator, got %v", result.MissingFields)
	}
	if result.ExplanationConfidence != "low" {
		t.Errorf("expected confidence 'low' for partial extraction gap, got %q", result.ExplanationConfidence)
	}

	for _, st := range result.Stages {
		if st.StageName == "generate_explanation" {
			if conf, ok := st.Data["confidence"]; !ok || conf != "low" {
				t.Errorf("expected stage data confidence 'low', got %v", conf)
			}
			break
		}
	}
}

func TestPipeline_WithSyntheticSample(t *testing.T) {
	samplePath := filepath.Join("..", "..", "data", "sample_gastroscopy_report.txt")
	data, err := os.ReadFile(samplePath)
	if err != nil {
		t.Skipf("sample data not found at %s: %v", samplePath, err)
	}

	p, s := newTestPipeline(t)
	pid := createTestPatient(t, s)
	result, err := p.Run(context.Background(), pid, string(data), "2024-03-01", "")
	if err != nil {
		t.Fatal(err)
	}

	if result.ReportType == "" {
		t.Error("expected non-empty report type")
	}
	if len(result.Stages) != 6 {
		t.Errorf("expected 6 pipeline stages, got %d", len(result.Stages))
	}
}

// --- Source evidence retention tests ---

func TestPipeline_SourceEvidenceRetention(t *testing.T) {
	p, s := newTestPipeline(t)
	pid := createTestPatient(t, s)

	// SYNTHETIC DATA - not real patient information
	rawText := "胃镜检查报告\n十二指肠球部：前壁可见一处溃疡，大小约 0.8×0.6cm。"
	result, err := p.Run(context.Background(), pid, rawText, "2024-03-01", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.SourcePath == "" || result.SourceHash == "" {
		t.Fatal("expected non-empty SourcePath and SourceHash")
	}
	if result.SourceType != "text" {
		t.Errorf("SourceType = %q, want 'text' (default)", result.SourceType)
	}
	data, err := os.ReadFile(result.SourcePath)
	if err != nil {
		t.Fatalf("read archived evidence: %v", err)
	}
	if string(data) != rawText {
		t.Error("archived content does not match original raw_text")
	}
	reports, _ := s.ListHealthReports(pid)
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}
	r := reports[0]
	if r.SourcePath != result.SourcePath || r.SourceHash != result.SourceHash || r.SourceType != "text" {
		t.Errorf("stored report evidence mismatch: path=%q hash=%q type=%q", r.SourcePath, r.SourceHash, r.SourceType)
	}
}

func TestPipeline_SourceEvidenceRetention_ArchiveFailureBlocksAll(t *testing.T) {
	dir := t.TempDir()
	s, err := store.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	badPath := filepath.Join(dir, "not-a-dir")
	os.WriteFile(badPath, []byte("block"), 0o644)

	llmCalls := 0
	llm := &providers.MockProvider{
		CompleteFunc: func(_ context.Context, req providers.CompletionRequest) (*providers.CompletionResponse, error) {
			llmCalls++
			return &providers.CompletionResponse{Content: "gastroscopy"}, nil
		},
	}
	p := NewPipeline(llm, s, badPath)
	pid, _ := s.CreatePatientProfile(&model.PatientProfile{Name: "Test"})

	_, err = p.Run(context.Background(), pid, "SYNTHETIC DATA: test", "2024-01-01", "")
	if err == nil {
		t.Fatal("expected error when evidence archive fails")
	}
	if llmCalls != 0 {
		t.Fatalf("expected archive failure before any LLM calls, got %d calls", llmCalls)
	}
	reports, _ := s.ListHealthReports(pid)
	if len(reports) != 0 {
		t.Error("expected no reports stored")
	}
	indicators, _ := s.GetAbnormalIndicators(pid)
	if len(indicators) != 0 {
		t.Error("expected no indicators stored")
	}
}

func TestPipeline_SourceType_RejectsNonText(t *testing.T) {
	p, s := newTestPipeline(t)
	pid := createTestPatient(t, s)

	for _, bad := range []string{"photo", "pdf", "image"} {
		if _, err := p.Run(context.Background(), pid, "SYNTHETIC: test", "2024-06-01", bad); err == nil {
			t.Errorf("source_type=%q: expected error", bad)
		}
	}
	reports, _ := s.ListHealthReports(pid)
	if len(reports) != 0 {
		t.Error("expected no reports for rejected source_type")
	}
}
