package trend

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LawyZheng/nura/internal/model"
	"github.com/LawyZheng/nura/internal/providers"
	"github.com/LawyZheng/nura/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestService_GenerateInsight(t *testing.T) {
	s := newTestStore(t)
	pid, _ := s.CreatePatientProfile(&model.PatientProfile{Name: "Test Patient"})

	// SYNTHETIC DATA - not real patient information
	ps := 6
	s.InsertSymptomLog(&model.SymptomLog{
		PatientID: pid, PainScore: &ps, PainLocation: "upper_abdomen",
		StoolColor: "normal", RecordedAt: time.Now(),
	})
	s.InsertMealLog(&model.MealLog{
		PatientID: pid, MealType: "lunch", Content: "synthetic food",
		HasIrritant: true, IrritantDetail: "spicy",
		RecordedAt: time.Now(),
	})

	llm := &providers.MockProvider{
		CompleteFunc: func(_ context.Context, req providers.CompletionRequest) (*providers.CompletionResponse, error) {
			if !strings.Contains(req.SystemPrompt, "消化性溃疡") {
				t.Error("expected trend system prompt")
			}
			if !strings.Contains(req.UserPrompt, "症状数据") {
				t.Error("expected symptom data in user prompt")
			}
			// SYNTHETIC DATA - not real patient information
			return &providers.CompletionResponse{
				Content: "合成洞察：疼痛评分为 6，建议避免刺激性食物。",
			}, nil
		},
	}

	svc := NewService(llm, s)
	result, err := svc.GenerateInsight(context.Background(), pid, 7)
	if err != nil {
		t.Fatal(err)
	}

	if result.Insight == nil {
		t.Fatal("expected non-nil insight")
	}
	if !strings.Contains(result.Insight.Content, "合成洞察") {
		t.Errorf("content = %q, expected to contain '合成洞察'", result.Insight.Content)
	}
	if result.Disclaimer == "" {
		t.Error("expected disclaimer")
	}
	if result.Insight.InsightType != "trend_7d" {
		t.Errorf("insight_type = %q, want 'trend_7d'", result.Insight.InsightType)
	}
}

func TestService_GenerateInsight_CachedAfterInsert(t *testing.T) {
	s := newTestStore(t)
	pid, _ := s.CreatePatientProfile(&model.PatientProfile{Name: "Test Patient"})

	llm := &providers.MockProvider{
		CompleteFunc: func(_ context.Context, _ providers.CompletionRequest) (*providers.CompletionResponse, error) {
			return &providers.CompletionResponse{Content: "synthetic insight"}, nil
		},
	}

	svc := NewService(llm, s)
	_, err := svc.GenerateInsight(context.Background(), pid, 7)
	if err != nil {
		t.Fatal(err)
	}

	cached, err := s.GetLatestAIInsight(pid, "trend_7d")
	if err != nil {
		t.Fatal("expected cached insight after generation")
	}
	if cached.Content == "" {
		t.Error("expected non-empty cached content")
	}
}
