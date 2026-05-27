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

func newTestPipeline(t *testing.T) (*Pipeline, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	// Mock LLM that returns structured responses based on report content.
	llm := &providers.MockProvider{
		CompleteFunc: func(ctx context.Context, req providers.CompletionRequest) (*providers.CompletionResponse, error) {
			// Classification request: system prompt mentions "classifier".
			if strings.Contains(req.SystemPrompt, "classifier") {
				return &providers.CompletionResponse{
					Content: "gastroscopy",
					Usage:   providers.Usage{InputTokens: 10, OutputTokens: 5},
				}, nil
			}

			// Extraction request: system prompt mentions "extraction".
			if strings.Contains(req.SystemPrompt, "extraction") {
				facts := map[string]any{
					"indicators": []map[string]any{
						{
							"name":           "ulcer_stage",
							"name_cn":        "溃疡分期",
							"value":          "A2",
							"is_abnormal":    true,
						},
						{
							"name":           "ulcer_size",
							"name_cn":        "溃疡大小",
							"value":          "0.8x0.6",
							"unit":           "cm",
							"is_abnormal":    false,
						},
						{
							"name":           "hp_status",
							"name_cn":        "幽门螺杆菌",
							"value":          "阳性",
							"is_abnormal":    true,
						},
					},
				}
				data, _ := json.Marshal(facts)
				return &providers.CompletionResponse{
					Content: string(data),
					Usage:   providers.Usage{InputTokens: 100, OutputTokens: 50},
				}, nil
			}

			// Explanation request.
			return &providers.CompletionResponse{
				Content: "您的胃镜报告显示十二指肠球部有一处 A2 期溃疡（活动期），大小约 0.8×0.6cm。HP 检测阳性，建议进行根治治疗。\n\n以上内容仅供健康参考，不构成医疗建议。如有疑问请咨询专业医生。",
				Usage:   providers.Usage{InputTokens: 200, OutputTokens: 100},
			}, nil
		},
	}

	return NewPipeline(llm, s), s
}

func createTestPatient(t *testing.T, s *store.Store) int {
	t.Helper()
	id, err := s.CreatePatientProfile(&model.PatientProfile{Name: "Test Patient"})
	if err != nil {
		t.Fatalf("create patient: %v", err)
	}
	return id
}

func TestPipeline_GastroscopyReport(t *testing.T) {
	p, s := newTestPipeline(t)
	pid := createTestPatient(t, s)

	rawText := `胃镜检查报告
十二指肠球部：前壁可见一处溃疡，大小约 0.8×0.6cm，底覆白苔。
分期：A2 期（活动期）。
快速尿素酶试验：阳性（+）`

	result, err := p.Run(context.Background(), pid, rawText, "2024-03-01")
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

	if len(result.Stages) != 5 {
		t.Errorf("expected 5 stages, got %d", len(result.Stages))
	}

	// Check stored report is scoped to patient.
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

	result, err := p.Run(context.Background(), pid, "test report", "2024-01-01")
	if err != nil {
		t.Fatal(err)
	}

	expectedStages := []string{
		"classify_report",
		"extract_facts",
		"normalize_indicators",
		"merge_state",
		"generate_explanation",
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

func TestPipeline_WithSyntheticSample(t *testing.T) {
	// Find the sample data directory relative to the test.
	samplePath := filepath.Join("..", "..", "data", "sample_gastroscopy_report.txt")
	data, err := os.ReadFile(samplePath)
	if err != nil {
		t.Skipf("sample data not found at %s: %v", samplePath, err)
	}

	p, s := newTestPipeline(t)
	pid := createTestPatient(t, s)
	result, err := p.Run(context.Background(), pid, string(data), "2024-03-01")
	if err != nil {
		t.Fatal(err)
	}

	if result.ReportType == "" {
		t.Error("expected non-empty report type")
	}

	if len(result.Stages) != 5 {
		t.Errorf("expected 5 pipeline stages, got %d", len(result.Stages))
	}
}
